package deploy

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"encoding/json"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"github.com/AustinOyugi/no-oops-ops/internal/state"
	"gopkg.in/yaml.v3"
	"path/filepath"
)

type RecoveryProgress struct {
	App         string    `json:"app"`
	Environment string    `json:"environment"`
	Stage       string    `json:"stage"`
	Dependency  string    `json:"dependency,omitempty"`
	Error       string    `json:"error,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Recover replays the last successfully deployed stack, rather than creating a
// release or selecting a newer, undeployed image. The saved stack retains
// secrets, volume names and the exact deployed image (including wrappers).
func (s *Service) Recover(ctx context.Context, environment, path string) (operationErr error) {
	m, err := manifest.Load(path)
	if err != nil {
		return err
	}
	unlock, err := s.operationLock(ctx, m.Name, environment)
	if err != nil {
		return err
	}
	defer unlock()
	progress := RecoveryProgress{App: m.Name, Environment: environment}
	report := func(stage, dependency string) error {
		progress.Stage = stage
		progress.Dependency = dependency
		progress.UpdatedAt = time.Now().UTC()
		data, err := json.MarshalIndent(progress, "", "  ")
		if err != nil {
			return err
		}
		s.logger.InfoContext(ctx, "recovery progress", "app", m.Name, "stage", stage, "dependency", dependency, "error", progress.Error)
		return state.WriteFile(filepath.Join(appDir(s.config, m.Name, environment), "recovery.json"), append(data, '\n'), 0o600)
	}
	defer func() {
		if operationErr != nil {
			progress.Error = operationErr.Error()
			if err := report("failed", progress.Dependency); err != nil {
				operationErr = fmt.Errorf("%w; save recovery failure: %v", operationErr, err)
			}
		}
	}()
	if err := report("checking_dependencies", ""); err != nil {
		return err
	}
	for _, dependency := range m.DependsOn {
		if err := report("waiting_dependency", dependency); err != nil {
			return err
		}
		dm, err := manifest.LoadService(m.Path, dependency)
		if err != nil {
			return fmt.Errorf("dependency %s: %w", dependency, err)
		}
		d, err := s.deployments.Latest(s.config, dependency, environment)
		if err != nil {
			return err
		}
		if d.ServiceName == "" {
			return fmt.Errorf("dependency %s has no successful deployment; deploy or recover it before %s", dependency, m.Name)
		}
		if err := s.waitForRecoveredService(ctx, dm, d); err != nil {
			return fmt.Errorf("dependency %s failed readiness; %s remains blocked: %w", dependency, m.Name, err)
		}
	}
	if err := report("loading_deployment", ""); err != nil {
		return err
	}
	d, err := s.deployments.Latest(s.config, m.Name, environment)
	if err != nil {
		return err
	}
	if d.StackName == "" || d.ServiceName == "" {
		return fmt.Errorf("no successful deployment for %s in %s; deploy a release first", m.Name, environment)
	}
	saved := stackPath(s.config, m.Name, environment)
	if d.StackName != stackName(environment, m.Name) {
		saved = releaseStackPath(s.config, m.Name, environment, d.StackName)
	}
	if d.RecoveryStack != "" {
		saved, err = recoverySnapshotPath(appDir(s.config, m.Name, environment), d.RecoveryStack)
		if err != nil {
			return err
		}
	}
	image, replicas, err := recoveryStack(saved, d.ServiceName)
	if err != nil {
		return err
	}
	expected := d.DeployedImage
	if expected == "" {
		expected = d.ReleaseImage
	}
	if image != expected && image != wrappedImageRef(s.config, d.ReleaseImage, m.Name) {
		return fmt.Errorf("saved stack image %s differs from last successful deployment %s; restore the matching stack from backup", image, d.ReleaseImage)
	}
	if _, exists, err := loadJournal(s.config, m.Name, environment); err != nil {
		return err
	} else if exists {
		return fmt.Errorf("unfinished deployment journal for %s; reconcile it before recovery to avoid replaying partially updated state", m.Name)
	}
	if replicas == 0 {
		return fmt.Errorf("saved deployment %s is scaled to zero; recovery will not start it", m.Name)
	}
	// Pull the saved reference before altering the service; failures identify the
	// exact missing image and leave application storage untouched.
	if err := report("checking_image", ""); err != nil {
		return err
	}
	if err := pullImage(ctx, s.runner, image); err != nil {
		return fmt.Errorf("recovery image %s is unavailable: restore the registry/image backup and retry; %w", image, err)
	}
	if err := s.ensureStackNetworks(ctx, saved); err != nil {
		return err
	}
	if err := report("starting", ""); err != nil {
		return err
	}
	if err := s.deployStack(ctx, saved, d.StackName); err != nil {
		return err
	}
	if err := report("waiting_readiness", ""); err != nil {
		return err
	}
	timeout, monitor, err := convergenceConfig(m)
	if err != nil {
		return err
	}
	if _, _, err := s.waitForSwarmConvergence(ctx, d.ServiceName, image, replicas, timeout, monitor); err != nil {
		return fmt.Errorf("recover %s: inspect noops logs and docker service ps --no-trunc %s: %w", m.Name, d.ServiceName, err)
	}
	return report("ready", "")
}
func recoveryStack(path, service string) (string, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, fmt.Errorf("read saved recovery stack %s: restore deployment state from backup: %w", path, err)
	}
	var doc struct {
		Services map[string]struct {
			Image  string `yaml:"image"`
			Deploy struct {
				Replicas *int `yaml:"replicas"`
			} `yaml:"deploy"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return "", 0, err
	}
	_, key, _ := strings.Cut(service, "_")
	selected, ok := doc.Services[key]
	if !ok || selected.Image == "" || len(doc.Services) != 1 {
		return "", 0, fmt.Errorf("saved stack %s does not identify recovery service %s", path, service)
	}
	replicas := 1
	if selected.Deploy.Replicas != nil {
		replicas = *selected.Deploy.Replicas
	}
	return selected.Image, replicas, nil
}
func (s *Service) waitForRecoveredService(ctx context.Context, m manifest.Manifest, d Deployment) error {
	out, err := s.runner.Run(ctx, "docker", []string{"service", "inspect", "--format", "{{.Spec.TaskTemplate.ContainerSpec.Image}}|{{if .Spec.Mode.Replicated}}{{.Spec.Mode.Replicated.Replicas}}{{end}}", d.ServiceName}, command.RunOptions{})
	if err != nil {
		return fmt.Errorf("missing deployed dependency %s; recover it first: %w", m.Name, err)
	}
	image, count, _ := strings.Cut(strings.TrimSpace(string(out.Output)), "|")
	replicas, parseErr := strconv.Atoi(count)
	if parseErr != nil || replicas < 1 {
		return fmt.Errorf("dependency %s is scaled to zero or uses an unsupported service mode", m.Name)
	}
	if image == "" {
		return fmt.Errorf("dependency %s has no image", m.Name)
	}
	timeout, monitor, err := convergenceConfig(m)
	if err != nil {
		return err
	}
	_, _, err = s.waitForSwarmConvergence(ctx, d.ServiceName, image, replicas, timeout, monitor)
	return err
}
