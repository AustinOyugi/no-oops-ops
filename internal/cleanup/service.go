// Package cleanup removes registry images and deployment state that are no
// longer needed for a deploy or rollback.
package cleanup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"github.com/AustinOyugi/no-oops-ops/internal/state"
)

const DefaultKeep = 3

type Options struct {
	Apply            bool
	Keep             int
	Orphaned         bool
	App              string
	Environment      string
	ReleaseRetention bool
}
type Plan struct {
	ReleasePaths, DeploymentPaths, StackPaths, Images, LocalImages, ProtectedImages []string
	Protected                                                                       int
}
type Service struct {
	logger *slog.Logger
	cfg    config.Config
	runner commandRunner
}

type commandRunner interface {
	Run(context.Context, string, []string, command.RunOptions) (command.Result, error)
}

func NewService(logger *slog.Logger, cfg config.Config) *Service {
	return &Service{logger: logger, cfg: cfg, runner: command.NewRunner(logger)}
}

func (s *Service) Run(ctx context.Context, options Options) (Plan, error) {
	if (options.App == "") != (options.Environment == "") {
		return Plan{}, fmt.Errorf("cleanup app and environment must be supplied together")
	}
	if options.ReleaseRetention && options.App == "" {
		return Plan{}, fmt.Errorf("automatic retention requires an app and environment")
	}
	if options.Keep < 0 {
		return Plan{}, fmt.Errorf("keep must be zero or greater")
	}
	if options.Apply {
		unlock, err := state.AcquireLock(ctx, filepath.Join(s.cfg.StateDir, "registry.lock"))
		if err != nil {
			return Plan{}, err
		}
		defer unlock()
	}
	plan, err := s.buildPlan(ctx, options)
	if err != nil || !options.Apply {
		return plan, err
	}
	plan, err = s.buildPlan(ctx, options) // Recheck immediately before deletion.
	if err != nil {
		return plan, err
	}
	client := s.registryClient()
	protected, err := client.protectedDigests(ctx, plan.ProtectedImages)
	if err != nil {
		return plan, err
	}
	deletedAny := false
	for _, image := range plan.Images {
		deleted, err := client.deleteImage(ctx, image, protected)
		if err != nil {
			return plan, err
		}
		deletedAny = deletedAny || deleted
	}
	for _, image := range plan.LocalImages {
		if err := s.removeLocalImage(ctx, image); err != nil {
			return plan, err
		}
	}
	for _, path := range append(append(plan.ReleasePaths, plan.DeploymentPaths...), plan.StackPaths...) {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return plan, fmt.Errorf("remove cleanup artifact %q: %w", path, err)
		}
	}
	// A previous run can delete registry manifests and then be interrupted
	// before GC (for example while pruning host image tags). A retry no longer
	// sees those deleted tags as candidates, so history pruning also requests
	// the offline GC pass and lets cleanup converge.
	if deletedAny || len(plan.ReleasePaths) > 0 || len(plan.DeploymentPaths) > 0 {
		if err := s.garbageCollect(ctx); err != nil {
			return plan, err
		}
	}
	return plan, nil
}

func (s *Service) buildPlan(ctx context.Context, options Options) (Plan, error) {
	live, err := s.liveServices(ctx)
	if err != nil {
		return Plan{}, err
	}
	plan, err := s.planOptions(live, options)
	if err != nil {
		return plan, err
	}
	// Scoped retention deletes only recorded builds for this app/environment;
	// global registry inventory includes unrelated and unfinished releases.
	if options.App != "" {
		return plan, nil
	}
	client := s.registryClient()
	if err := client.addRegistryCandidates(ctx, &plan); err != nil {
		return plan, err
	}
	return plan, nil
}
func (s *Service) registryClient() registryClient {
	return registryClient{runner: s.runner, service: s.cfg.RegistryName + "_registry", port: s.cfg.RegistryPort}
}
func (s *Service) removeLocalImage(ctx context.Context, image string) error {
	result, err := s.runner.Run(ctx, "docker", []string{"image", "rm", image}, command.RunOptions{})
	if err == nil || strings.Contains(string(result.Output), "No such image") || localImageInUse(string(result.Output)) {
		return nil
	}
	return fmt.Errorf("remove local image %q: %w: %s", image, err, strings.TrimSpace(string(result.Output)))
}

// A stopped container can retain an image reference even though Swarm no
// longer runs that version. Never force-remove it: registry cleanup is safe,
// while host cache eviction is best-effort and must not abort the run.
func localImageInUse(output string) bool {
	return strings.Contains(output, "conflict: unable to delete") && strings.Contains(output, "is using its referenced image")
}
func (s *Service) garbageCollect(ctx context.Context) (operationErr error) {
	service := s.cfg.RegistryName + "_registry"
	defer func() {
		restoreCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 45*time.Second)
		defer cancel()
		result, err := s.runner.Run(restoreCtx, "docker", []string{"service", "scale", service + "=1"}, command.RunOptions{LogCommand: true})
		if err != nil {
			operationErr = errors.Join(operationErr, fmt.Errorf("restart registry after cleanup: %w: %s", err, result.Output))
			return
		}
		if err := s.waitForRegistry(restoreCtx); err != nil {
			operationErr = errors.Join(operationErr, err)
		}
	}()
	if _, err := s.runner.Run(ctx, "docker", []string{"service", "scale", service + "=0"}, command.RunOptions{LogCommand: true}); err != nil {
		return err
	}
	for i := 0; i < 60; i++ {
		out, err := s.runner.Run(ctx, "docker", []string{"ps", "-q", "--filter", "name=" + service}, command.RunOptions{})
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(out.Output)) == "" {
			_, err = s.runner.Run(ctx, "docker", []string{"run", "--rm", "-v", filepath.Join(s.cfg.DataDir, "data") + ":/var/lib/registry", "-v", filepath.Join(s.cfg.StateDir, "registry", "config.yml") + ":/etc/docker/registry/config.yml:ro", "registry:2", "registry", "garbage-collect", "/etc/docker/registry/config.yml", "--delete-untagged"}, command.RunOptions{LogCommand: true})
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("timed out waiting for registry service to stop")
}

// Wait for the registry API before a release --deploy or subsequent release
// continues; scaling the service up alone does not mean it is ready to serve.
func (s *Service) waitForRegistry(ctx context.Context) error {
	for {
		client := s.registryClient()
		response, err := client.request(ctx, "GET", "/v2/", "application/json")
		if err == nil && strings.Contains(response, " 200 ") {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("registry did not become ready after cleanup: %w", ctx.Err())
		case <-time.After(time.Second):
		}
	}
}
