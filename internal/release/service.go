package release

import (
	"context"
	"encoding/json"
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

	baseDir := filepath.Dir(absPath)
	contextDir := resolveSourcePath(baseDir, m.Source.Context)
	dockerfile := resolveSourcePath(baseDir, m.Source.Dockerfile)

	if err := s.runBuildCommand(ctx, contextDir, m.Source.Build.Command); err != nil {
		return Result{}, err
	}

	if err := s.buildImage(ctx, image, dockerfile, contextDir); err != nil {
		return Result{}, err
	}

	registryImage := registryImage(s.config, image)

	if err := s.tagImage(ctx, image, registryImage); err != nil {
		return Result{}, err
	}

	if err := s.pushImage(ctx, registryImage); err != nil {
		return Result{}, err
	}

	metadataPath, err := writeMetadata(s.config, m.Name, Metadata{
		Environment:   environment,
		Image:         image,
		RegistryImage: registryImage,
		Tag:           tag,
	})
	if err != nil {
		return Result{}, err
	}

	return Result{
		Environment:   environment,
		MetadataPath:  metadataPath,
		ManifestPath:  absPath,
		Image:         image,
		RegistryImage: registryImage,
		Built:         true,
		Tag:           tag,
		Pushed:        true,
		Manifest:      m,
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
			LogCommand:   true,
			Workdir:      contextDir,
			StreamOutput: true,
			Stdout:       os.Stdout,
			Stderr:       os.Stderr,
		},
	)
	if err != nil {
		return fmt.Errorf("build image %q: %w: %s", image, err, strings.TrimSpace(string(result.Output)))
	}

	return nil
}

func (s *Service) runBuildCommand(ctx context.Context, contextDir string, commandArgs []string) error {
	if len(commandArgs) == 0 {
		return nil
	}

	name := commandArgs[0]
	args := commandArgs[1:]

	result, err := s.runner.Run(
		ctx,
		name,
		args,
		command.RunOptions{
			LogCommand:   true,
			Workdir:      contextDir,
			StreamOutput: true,
			Stdout:       os.Stdout,
			Stderr:       os.Stderr,
		},
	)

	if err != nil {
		return fmt.Errorf(
			"run build command %q: %w: %s",
			strings.Join(commandArgs, " "),
			err,
			strings.TrimSpace(string(result.Output)),
		)
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

func (s *Service) tagImage(ctx context.Context, sourceImage string, targetImage string) error {
	result, err := s.runner.Run(
		ctx,
		"docker",
		[]string{
			"tag",
			sourceImage,
			targetImage,
		},
		command.RunOptions{
			LogCommand: true,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"tag image %q as %q: %w: %s",
			sourceImage,
			targetImage,
			err,
			strings.TrimSpace(string(result.Output)),
		)
	}

	return nil
}

func releaseTag() string {
	return time.Now().UTC().Format("20060102-150405")
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

func releaseMetadataPath(cfg config.Config, appName string, environment string) string {
	return filepath.Join(appDir(cfg, appName, environment), "release.json")
}

func writeMetadata(cfg config.Config, appName string, metadata Metadata) (string, error) {
	dir := appDir(cfg, appName, metadata.Environment)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create app dir %q: %w", dir, err)
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal release metadata: %w", err)
	}

	data = append(data, '\n')

	path := releaseMetadataPath(cfg, appName, metadata.Environment)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write release metadata %q: %w", path, err)
	}

	return path, nil
}
