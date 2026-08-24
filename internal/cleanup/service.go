// Package cleanup removes registry images and deployment state that are no
// longer needed for a deploy or rollback.
package cleanup

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/deploy"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"github.com/AustinOyugi/no-oops-ops/internal/release"
)

type Options struct {
	Apply    bool
	Keep     int
	Orphaned bool
}
type Plan struct {
	ReleasePaths, DeploymentPaths, Images []string
	Protected                             int
}
type Service struct {
	logger *slog.Logger
	cfg    config.Config
	runner *command.Runner
}

func NewService(logger *slog.Logger, cfg config.Config) *Service {
	return &Service{logger: logger, cfg: cfg, runner: command.NewRunner(logger)}
}

func (s *Service) Run(ctx context.Context, options Options) (Plan, error) {
	if options.Keep < 0 {
		return Plan{}, fmt.Errorf("keep must be zero or greater")
	}
	liveServices, err := s.liveServices(ctx)
	if err != nil {
		return Plan{}, err
	}
	plan, err := s.plan(liveServices, options.Keep, options.Orphaned)
	if err != nil || !options.Apply {
		return plan, err
	}
	client := registryClient{runner: s.runner, service: s.cfg.RegistryName + "_registry"}
	deletedAny := false
	for _, image := range plan.Images {
		deleted, err := client.deleteImage(ctx, image)
		if err != nil {
			return plan, err
		}
		deletedAny = deletedAny || deleted
	}
	for _, path := range append(plan.ReleasePaths, plan.DeploymentPaths...) {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return plan, fmt.Errorf("remove cleanup metadata %q: %w", path, err)
		}
	}
	if deletedAny {
		if err := s.garbageCollect(ctx); err != nil {
			return plan, err
		}
	}
	return plan, nil
}

func (s *Service) plan(liveServices map[string]string, keep int, orphanedOnly bool) (Plan, error) {
	protected := make(map[string]struct{}, len(liveServices))
	for _, image := range liveServices {
		protected[image] = struct{}{}
	}
	var plan Plan
	root := filepath.Join(s.cfg.StateDir, "apps")
	apps, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return plan, nil
	}
	if err != nil {
		return plan, err
	}
	for _, app := range apps {
		if !app.IsDir() {
			continue
		}
		envs, err := os.ReadDir(filepath.Join(root, app.Name()))
		if err != nil {
			return plan, err
		}
		for _, env := range envs {
			if !env.IsDir() {
				continue
			}
			dir := filepath.Join(root, app.Name(), env.Name())
			releases, err := release.ListHistory(s.cfg, app.Name(), env.Name())
			if err != nil {
				return plan, err
			}
			sort.Slice(releases, func(i, j int) bool { return releases[i].CreateAt.After(releases[j].CreateAt) })
			deployments, paths, err := readDeployments(dir)
			if err != nil {
				return plan, err
			}
			orphaned := orphanedOnly && !hasLiveDeployment(deployments, liveServices)
			if !orphaned {
				for i, item := range releases {
					if i < keep {
						protected[item.RegistryImage] = struct{}{}
					}
				}
			}
			success := make([]deploy.Deployment, 0, len(deployments))
			for _, d := range deployments {
				if d.Outcome == "" || d.Outcome == deploy.SwarmOutcomeCompleted {
					success = append(success, d)
				}
			}
			sort.Slice(success, func(i, j int) bool { return success[i].CreatedAt.After(success[j].CreatedAt) })
			for i, d := range success {
				if !orphaned && i < 2 {
					protected[d.ReleaseImage] = struct{}{}
				}
			}
			for _, item := range releases {
				_, protectedImage := protected[item.RegistryImage]
				if orphaned || !protectedImage {
					plan.ReleasePaths = append(plan.ReleasePaths, filepath.Join(dir, "releases", item.Tag+".json"))
					if !protectedImage {
						plan.Images = append(plan.Images, item.RegistryImage)
					}
				}
			}
			for i, d := range deployments {
				if orphaned {
					plan.DeploymentPaths = append(plan.DeploymentPaths, paths[i])
					continue
				}
				if _, ok := protected[d.ReleaseImage]; !ok {
					plan.DeploymentPaths = append(plan.DeploymentPaths, paths[i])
				}
			}
		}
	}
	plan.Protected = len(protected)
	return plan, nil
}

func readDeployments(dir string) ([]deploy.Deployment, []string, error) {
	entries, err := os.ReadDir(filepath.Join(dir, "deployments"))
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var items []deploy.Deployment
	var paths []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, "deployments", e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}
		var d deploy.Deployment
		if err := json.Unmarshal(data, &d); err != nil {
			return nil, nil, err
		}
		items = append(items, d)
		paths = append(paths, path)
	}
	return items, paths, nil
}

func hasLiveDeployment(deployments []deploy.Deployment, liveServices map[string]string) bool {
	for _, deployment := range deployments {
		if _, ok := liveServices[deployment.ServiceName]; ok {
			return true
		}
	}
	return false
}

func (s *Service) liveServices(ctx context.Context) (map[string]string, error) {
	out, err := s.runner.Run(ctx, "docker", []string{"service", "ls", "-q"}, command.RunOptions{})
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	for _, id := range strings.Fields(string(out.Output)) {
		tasks, err := s.runner.Run(ctx, "docker", []string{"service", "ps", "--filter", "desired-state=running", "--format", "{{.CurrentState}}", id}, command.RunOptions{})
		if err != nil {
			return nil, fmt.Errorf("inspect running tasks for service %q: %w", id, err)
		}
		if !hasRunningTask(string(tasks.Output)) {
			continue
		}
		img, err := s.runner.Run(ctx, "docker", []string{"service", "inspect", "--format", "{{.Spec.TaskTemplate.ContainerSpec.Image}}", id}, command.RunOptions{})
		if err == nil {
			result[id] = strings.TrimSpace(string(img.Output))
		}
	}
	return result, nil
}

func hasRunningTask(output string) bool {
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Running") {
			return true
		}
	}
	return false
}

func (s *Service) garbageCollect(ctx context.Context) error {
	service := s.cfg.RegistryName + "_registry"
	if _, err := s.runner.Run(ctx, "docker", []string{"service", "scale", service + "=0"}, command.RunOptions{LogCommand: true}); err != nil {
		return err
	}
	defer s.runner.Run(context.Background(), "docker", []string{"service", "scale", service + "=1"}, command.RunOptions{LogCommand: true})
	stopped := false
	for i := 0; i < 60; i++ {
		out, err := s.runner.Run(ctx, "docker", []string{"ps", "-q", "--filter", "name=" + service}, command.RunOptions{})
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(out.Output)) == "" {
			stopped = true
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	if !stopped {
		return fmt.Errorf("timed out waiting for registry service to stop")
	}
	_, err := s.runner.Run(ctx, "docker", []string{"run", "--rm", "-v", filepath.Join(s.cfg.DataDir, "data") + ":/var/lib/registry", "-v", filepath.Join(s.cfg.StateDir, "registry", "config.yml") + ":/etc/docker/registry/config.yml:ro", "registry:2", "registry", "garbage-collect", "/etc/docker/registry/config.yml", "--delete-untagged"}, command.RunOptions{LogCommand: true})
	return err
}

type registryClient struct {
	runner             *command.Runner
	service, container string
}

func (c *registryClient) deleteImage(ctx context.Context, image string) (bool, error) {
	prefix := "127.0.0.1:"
	if !strings.HasPrefix(image, prefix) {
		return false, nil
	}
	name := strings.TrimPrefix(image, prefix)
	slash := strings.Index(name, "/")
	colon := strings.LastIndex(name, ":")
	if slash < 0 || colon < slash {
		return false, fmt.Errorf("invalid registry image %q", image)
	}
	repo, tag := name[slash+1:colon], name[colon+1:]
	head, err := c.request(ctx, "HEAD", "/v2/"+repo+"/manifests/"+tag, "application/vnd.docker.distribution.manifest.v2+json")
	if err != nil {
		return false, err
	}
	digest := registryDigest(head)
	if digest == "" {
		// The manifest may already have been removed by a previous cleanup or
		// an external registry operation. It is safe to continue so the stale
		// No Oops Ops metadata can be cleared as well.
		return false, nil
	}
	response, err := c.request(ctx, "DELETE", "/v2/"+repo+"/manifests/"+digest, "")
	if err != nil {
		return false, err
	}
	return registryManifestDeleted(response), nil
}

func registryDigest(response string) string {
	for _, line := range strings.Split(response, "\n") {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "docker-content-digest:") {
			return strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		}
	}
	return ""
}

func registryManifestDeleted(response string) bool {
	return strings.Contains(response, " 202 ")
}
func (c *registryClient) request(ctx context.Context, method, path, accept string) (string, error) {
	if c.container == "" {
		out, err := c.runner.Run(ctx, "docker", []string{"ps", "--filter", "name=" + c.service, "--format", "{{.ID}}"}, command.RunOptions{})
		if err != nil {
			return "", err
		}
		c.container = strings.TrimSpace(string(out.Output))
		if c.container == "" {
			return "", fmt.Errorf("registry service is not running")
		}
	}
	script := `printf '%s %s HTTP/1.1\r\nHost: 127.0.0.1\r\nAccept: %s\r\nConnection: close\r\n\r\n' "$1" "$2" "$3" | nc -w 10 127.0.0.1 5000`
	out, err := c.runner.Run(ctx, "docker", []string{"exec", c.container, "/bin/sh", "-c", script, "noops-cleanup", method, path, accept}, command.RunOptions{})
	if err != nil {
		return "", err
	}
	if !strings.Contains(string(out.Output), " 200 ") && !strings.Contains(string(out.Output), " 202 ") && !strings.Contains(string(out.Output), " 404 ") {
		return "", fmt.Errorf("registry request failed: %s", strings.TrimSpace(string(out.Output)))
	}
	return string(out.Output), nil
}
