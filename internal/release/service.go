package release

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

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

	image := fmt.Sprintf("%s:%s", m.Image.Repository, m.Image.Tag)

	baseDir := filepath.Dir(absPath)
	contextDir := resolveSourcePath(baseDir, m.Source.Context)
	dockerfile := resolveSourcePath(baseDir, m.Source.Dockerfile)

	if err := s.buildImage(ctx, image, dockerfile, contextDir); err != nil {
		return Result{}, err
	}

	return Result{
		Environment:  environment,
		ManifestPath: absPath,
		Image:        image,
		Built:        true,
		Manifest:     m,
	}, nil
}

func (s *Service) buildImage(ctx context.Context, image string, dockerfile string, contextDir string) error {
	result, err := s.runner.Run(
		ctx,
		"docker",
		[]string{
			"build",
			"-t",
			image,
			"-f",
			dockerfile,
			contextDir,
		},
		command.RunOptions{
			LogCommand: true,
		},
	)
	if err != nil {
		return fmt.Errorf("build image %q: %w: %s", image, err, strings.TrimSpace(string(result.Output)))
	}

	return nil
}

func resolveSourcePath(baseDir string, value string) string {
	if filepath.IsAbs(value) {
		return value
	}

	return filepath.Join(baseDir, value)
}
