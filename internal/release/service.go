package release

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/environment"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"github.com/AustinOyugi/no-oops-ops/internal/secret"
	"github.com/AustinOyugi/no-oops-ops/internal/templateutil"
)

type Service struct {
	logger       *slog.Logger
	config       config.Config
	runner       *command.Runner
	secrets      *secret.Service
	afterRelease func(context.Context, Result) error
}

func NewService(logger *slog.Logger, cfg config.Config) *Service {
	return &Service{
		logger:  logger,
		config:  cfg,
		runner:  command.NewRunner(logger),
		secrets: secret.NewService(logger, cfg),
	}
}

// SetAfterRelease installs a synchronous retention job. It runs after release
// locks are released and only once the image and metadata were saved.
func (s *Service) SetAfterRelease(job func(context.Context, Result) error) { s.afterRelease = job }

// Options controls explicit release overrides.
type Options struct {
	Force bool
}

func (s *Service) Run(ctx context.Context, environment, path string) (Result, error) {
	return s.RunWithOptions(ctx, environment, path, Options{})
}

// RunWithOptions creates a release, optionally replacing an existing tag.
func (s *Service) RunWithOptions(ctx context.Context, environment, path string, options Options) (Result, error) {
	result, err := s.runLocked(ctx, environment, path, options)
	if err != nil {
		return result, err
	}
	s.completeRelease(ctx, result)
	return result, nil
}

func (s *Service) completeRelease(ctx context.Context, result Result) {
	if s.afterRelease == nil || !result.Pushed {
		return
	}
	if err := s.afterRelease(ctx, result); err != nil {
		s.logger.WarnContext(ctx, "release completed but automatic cleanup failed", "app", result.Manifest.Name, "environment", result.Environment, "tag", result.Tag, "error", err)
	}
}

func (s *Service) runLocked(ctx context.Context, environment string, path string, options Options) (Result, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return Result{}, fmt.Errorf("resolve manifest path %q: %w", path, err)
	}

	s.logger.InfoContext(ctx, "starting release", "manifest", absPath, "environment", environment)

	m, err := manifest.Load(absPath)
	if err != nil {
		return Result{}, err
	}

	unlock, err := s.acquireReleaseLocks(ctx, m.Name, environment)
	if err != nil {
		return Result{}, err
	}
	defer unlock()

	if m.Image.ShouldBuild() {
		return s.releaseBuild(ctx, environment, absPath, m, options)
	}

	return s.releaseExternalImage(ctx, environment, absPath, m, options)
}

func (s *Service) buildPulledImage(ctx context.Context, targetImage, sourceImage string) error {
	contextDir, err := os.MkdirTemp("", "noops-release-*")
	if err != nil {
		return fmt.Errorf("create temporary Docker build context: %w", err)
	}

	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {

		}
	}(contextDir)

	dockerfile := filepath.Join(contextDir, "Dockerfile")
	content, err := templateutil.Dockerfile(sourceImage, false)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dockerfile, content, 0o600); err != nil {
		return fmt.Errorf("write temporary Dockerfile: %w", err)
	}

	if err := s.buildImage(ctx, targetImage, dockerfile, contextDir, manifest.BuildResources{}, false, nil); err != nil {
		return fmt.Errorf("build release image from %q: %w", sourceImage, err)
	}

	return nil
}

// buildEnvironmentValues deliberately resolves only ordinary values. Managed
// secrets are injected by Swarm when the service starts and must never become
// Docker build arguments or image layers.
func buildEnvironmentValues(manifestPath string, m manifest.Manifest, target string) (map[string]string, error) {
	path := ""
	if m.Env.File != "" {
		path = filepath.Join(filepath.Dir(manifestPath), m.Env.File)
	}
	file, err := environment.LoadOptional(path)
	if err != nil {
		return nil, err
	}
	return environment.Resolve(file, target, nil).Values, nil
}

func resolveSourcePath(baseDir string, value string) string {
	if filepath.IsAbs(value) {
		return value
	}

	return filepath.Join(baseDir, value)
}

func resolveGitSourcePath(baseDir, value string) (string, error) {
	if filepath.IsAbs(value) {
		return "", fmt.Errorf("git build contexts require relative build.context and build.dockerfile paths")
	}

	path := filepath.Clean(filepath.Join(baseDir, value))
	rel, err := filepath.Rel(baseDir, path)

	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("git build path %q escapes the checked-out repository", value)
	}

	return path, nil
}

func registryImage(cfg config.Config, image string) string {
	return fmt.Sprintf("127.0.0.1:%s/%s", cfg.RegistryPort, image)
}

func (s *Service) resolvePushedDigest(ctx context.Context, image string) (string, error) {
	result, err := s.runner.Run(ctx, "docker", []string{"image", "inspect", "--format", `{{join .RepoDigests "\n"}}`, image}, command.RunOptions{})
	if err != nil {
		return "", fmt.Errorf("inspect pushed image %q: %w: %s", image, err, strings.TrimSpace(string(result.Output)))
	}

	registry := strings.Split(image, "/")[0]
	for _, candidate := range strings.Fields(string(result.Output)) {
		if strings.HasPrefix(candidate, registry+"/") && strings.Contains(candidate, "@sha256:") {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("inspect pushed image %q: registry digest is unavailable", image)
}

func (s *Service) ensureTagUnused(ctx context.Context, app, environment, tag string, options Options) error {
	_, err := NewFilesystemStore().Find(s.config, app, environment, tag)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect release tag %q: %w", tag, err)
	}
	if options.Force {
		s.logger.WarnContext(ctx, "overwriting existing release tag", "app", app, "environment", environment, "tag", tag)
		return nil
	}
	return fmt.Errorf("deterministic release tag %q already exists; refusing to overwrite it (use --force or -f to overwrite)", tag)
}

// sourceTag records the mutable upstream tag that was snapshotted when an
// external image release is requested. Source builds have no upstream image
// tag to retain.
func sourceTag(m manifest.Manifest) string {
	if m.Image.ShouldBuild() {
		return ""
	}
	if m.Image.SourceReference != "" {
		return m.Image.SourceReference
	}

	return m.Image.Tag
}

func (s *Service) pushImage(ctx context.Context, image string) error {
	result, err := s.runner.Run(
		ctx,
		"docker",
		[]string{
			"push",
			image,
		},
		command.RunOptions{
			LogCommand:   true,
			StreamOutput: true,
			Stdout:       os.Stdout,
			Stderr:       os.Stderr,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"push image %q: %w: %s",
			image,
			err,
			strings.TrimSpace(string(result.Output)),
		)
	}

	return nil
}

func appDir(cfg config.Config, appName string, environment string) string {
	return filepath.Join(cfg.StateDir, "apps", appName, environment)
}

func releaseHistoryMetadataDir(cfg config.Config, appName string, environment string) string {
	return filepath.Join(appDir(cfg, appName, environment), "releases")
}

func releaseHistoryMetadataPath(path string, tag string) string {
	return filepath.Join(path, tag+".json")
}
