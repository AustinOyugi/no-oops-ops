package deploy

import (
	"context"
	"fmt"
	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"log/slog"
	"path/filepath"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

type Service struct {
	logger *slog.Logger
	config config.Config
}

func NewService(logger *slog.Logger, cfg config.Config) *Service {
	return &Service{
		logger: logger,
		config: cfg,
	}
}

func (s *Service) Run(ctx context.Context, environment string, path string) (Result, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return Result{}, fmt.Errorf("resolve manifest path %q: %w", path, err)
	}

	s.logger.InfoContext(ctx, "starting deploy", "manifest", absPath, "environment", environment)

	m, err := manifest.Load(absPath)
	if err != nil {
		return Result{}, err
	}

	envFilePath := resolveEnvFilePath(absPath, m.Env.File)

	envFile, err := LoadEnvFile(envFilePath)
	if err != nil {
		return Result{}, err
	}

	resolvedEnv := ResolveEnvFile(envFile, environment)

	envPath, err := writeEnvMap(s.config, environment, m.Name, resolvedEnv)
	if err != nil {
		return Result{}, err
	}

	stackPath, err := writeStack(s.config, environment, m)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Environment:  environment,
		ServiceName:  serviceName(environment, m.Name),
		ManifestPath: absPath,
		StackPath:    stackPath,
		EnvPath:      envPath,
		Manifest:     m,
	}, nil
}

func resolveEnvFilePath(manifestPath string, envFile string) string {
	return filepath.Join(filepath.Dir(manifestPath), envFile)
}
