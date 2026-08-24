package release

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

type Service struct {
	logger *slog.Logger
	config config.Config
	runner *command.Runner
}

func NewService(logger *slog.Logger, cfg config.Config) *Service {
	return &Service{
		logger: logger,
		config: cfg,
		runner: command.NewRunner(logger),
	}
}

func (s *Service) Run(ctx context.Context, environment string, path string) (Result, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return Result{}, fmt.Errorf("resolve manifest path %q: %w", path, err)
	}

	s.logger.InfoContext(ctx, "starting release", "manifest", absPath, "environment", environment)

	m, err := manifest.Load(absPath)
	if err != nil {
		return Result{}, err
	}

	tag := releaseTag()
	image := fmt.Sprintf("%s:%s", m.Image.Repository, tag)
	registryImage := registryImage(s.config, image)

	if m.Image.ShouldBuild() {
		baseDir := filepath.Dir(absPath)
		contextDir := resolveSourcePath(baseDir, m.Source.Context)
		dockerfile := resolveSourcePath(baseDir, m.Source.Dockerfile)

		if err := s.runBuildCommand(ctx, contextDir, m.Source.Build.Command); err != nil {
			return Result{}, err
		}

		if err := s.buildImage(ctx, registryImage, dockerfile, contextDir); err != nil {
			return Result{}, err
		}
	} else {
		sourceReference := m.Image.SourceReference
		if sourceReference == "" {
			sourceReference = fmt.Sprintf("%s:%s", m.Image.Repository, m.Image.Tag)
		}
		if err := s.buildPulledImage(ctx, registryImage, sourceReference); err != nil {
			return Result{}, err
		}
	}

	if err := s.pushImage(ctx, registryImage); err != nil {
		return Result{}, err
	}

	metadataHistoryPath, err := saveMetadataHistory(s.config, m.Name, Metadata{
		App:           m.Name,
		Build:         m.Image.ShouldBuild(),
		CreateAt:      time.Now().UTC(),
		Environment:   environment,
		Image:         image,
		RegistryImage: registryImage,
		SourceTag:     sourceTag(m),
		Tag:           tag,
	})

	if err != nil {
		return Result{}, err
	}

	return Result{
		Environment:   environment,
		MetadataPath:  metadataHistoryPath,
		ManifestPath:  absPath,
		Image:         image,
		RegistryImage: registryImage,
		Built:         true,
		Tag:           tag,
		Pushed:        true,
		Manifest:      m,
	}, nil
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
	if err := os.WriteFile(dockerfile, []byte(fmt.Sprintf("FROM %s\n", sourceImage)), 0o600); err != nil {
		return fmt.Errorf("write temporary Dockerfile: %w", err)
	}

	if err := s.buildImage(ctx, targetImage, dockerfile, contextDir); err != nil {
		return fmt.Errorf("build release image from %q: %w", sourceImage, err)
	}

	return nil
}

func resolveSourcePath(baseDir string, value string) string {
	if filepath.IsAbs(value) {
		return value
	}

	return filepath.Join(baseDir, value)
}

func registryImage(cfg config.Config, image string) string {
	return fmt.Sprintf("127.0.0.1:%s/%s", cfg.RegistryPort, image)
}

func releaseTag() string {
	return time.Now().UTC().Format("20060102-150405")
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
