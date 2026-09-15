// Package cleanup removes registry images and deployment state that are no
// longer needed for a deploy or rollback.
package cleanup

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

type Options struct {
	Apply    bool
	Keep     int
	Orphaned bool
}
type Plan struct {
	ReleasePaths, DeploymentPaths, Images, LocalImages, ProtectedImages []string
	Protected                                                           int
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

func (s *Service) buildPlan(ctx context.Context, options Options) (Plan, error) {
	live, err := s.liveServices(ctx)
	if err != nil {
		return Plan{}, err
	}
	plan, err := s.plan(live, options.Keep, options.Orphaned)
	if err != nil {
		return plan, err
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
	result, err := s.runner.Run(ctx, "docker", []string{"image", "rm", image}, command.RunOptions{LogCommand: true})
	if err == nil || strings.Contains(string(result.Output), "No such image") {
		return nil
	}
	return fmt.Errorf("remove local image %q: %w: %s", image, err, strings.TrimSpace(string(result.Output)))
}
func (s *Service) garbageCollect(ctx context.Context) error {
	service := s.cfg.RegistryName + "_registry"
	if _, err := s.runner.Run(ctx, "docker", []string{"service", "scale", service + "=0"}, command.RunOptions{LogCommand: true}); err != nil {
		return err
	}
	defer s.runner.Run(context.Background(), "docker", []string{"service", "scale", service + "=1"}, command.RunOptions{LogCommand: true})
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
