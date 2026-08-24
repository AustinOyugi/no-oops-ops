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
	Apply bool
	Keep  int
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
	live, err := s.liveImages(ctx)
	if err != nil {
		return Plan{}, err
	}
	plan, err := s.plan(live, options.Keep)
	if err != nil || !options.Apply {
		return plan, err
	}
	client := registryClient{runner: s.runner, service: s.cfg.RegistryName + "_registry"}
	for _, image := range plan.Images {
		if err := client.deleteImage(ctx, image); err != nil {
			return plan, err
		}
	}
	for _, path := range append(plan.ReleasePaths, plan.DeploymentPaths...) {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return plan, fmt.Errorf("remove cleanup metadata %q: %w", path, err)
		}
	}
	if len(plan.Images) > 0 {
		if err := s.garbageCollect(ctx); err != nil {
			return plan, err
		}
	}
	return plan, nil
}

func (s *Service) plan(live map[string]struct{}, keep int) (Plan, error) {
	protected := live
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
			for i, item := range releases {
				if i < keep {
					protected[item.RegistryImage] = struct{}{}
				}
			}
			deployments, paths, err := readDeployments(dir)
			if err != nil {
				return plan, err
			}
			success := make([]deploy.Deployment, 0, len(deployments))
			for _, d := range deployments {
				if d.Outcome == "" || d.Outcome == deploy.SwarmOutcomeCompleted {
					success = append(success, d)
				}
			}
			sort.Slice(success, func(i, j int) bool { return success[i].CreatedAt.After(success[j].CreatedAt) })
			for i, d := range success {
				if i < 2 {
					protected[d.ReleaseImage] = struct{}{}
				}
			}
			for _, item := range releases {
				if _, ok := protected[item.RegistryImage]; !ok {
					plan.ReleasePaths = append(plan.ReleasePaths, filepath.Join(dir, "releases", item.Tag+".json"))
					plan.Images = append(plan.Images, item.RegistryImage)
				}
			}
			for i, d := range deployments {
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

func (s *Service) liveImages(ctx context.Context) (map[string]struct{}, error) {
	out, err := s.runner.Run(ctx, "docker", []string{"service", "ls", "-q"}, command.RunOptions{})
	if err != nil {
		return nil, err
	}
	result := map[string]struct{}{}
	for _, id := range strings.Fields(string(out.Output)) {
		img, err := s.runner.Run(ctx, "docker", []string{"service", "inspect", "--format", "{{.Spec.TaskTemplate.ContainerSpec.Image}}", id}, command.RunOptions{})
		if err == nil {
			result[strings.TrimSpace(string(img.Output))] = struct{}{}
		}
	}
	return result, nil
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

func (c *registryClient) deleteImage(ctx context.Context, image string) error {
	prefix := "127.0.0.1:"
	if !strings.HasPrefix(image, prefix) {
		return nil
	}
	name := strings.TrimPrefix(image, prefix)
	slash := strings.Index(name, "/")
	colon := strings.LastIndex(name, ":")
	if slash < 0 || colon < slash {
		return fmt.Errorf("invalid registry image %q", image)
	}
	repo, tag := name[slash+1:colon], name[colon+1:]
	head, err := c.request(ctx, "HEAD", "/v2/"+repo+"/manifests/"+tag, "application/vnd.docker.distribution.manifest.v2+json")
	if err != nil {
		return err
	}
	digest := ""
	for _, line := range strings.Split(head, "\n") {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "docker-content-digest:") {
			digest = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		}
	}
	if digest == "" {
		return fmt.Errorf("registry digest missing for %s", image)
	}
	_, err = c.request(ctx, "DELETE", "/v2/"+repo+"/manifests/"+digest, "")
	return err
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
