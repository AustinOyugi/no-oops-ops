package deploy

import (
	"context"
	"fmt"
	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"log/slog"
	"path/filepath"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
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

	envPath, err := writeEnvMap(s.config, m.Name, environment, resolvedEnv)
	if err != nil {
		return Result{}, err
	}

	stackPath, err := writeStack(s.config, environment, m)
	if err != nil {
		return Result{}, err
	}

	if err := s.deployStack(ctx, stackPath, stackName(environment, m.Name)); err != nil {
		return Result{}, err
	}

	if err := s.verifyService(ctx, swarmServiceName(environment, m.Name)); err != nil {
		return Result{}, err
	}

	return Result{
		Environment:  environment,
		ServiceName:  serviceName(environment, m.Name),
		Executed:     true,
		Verified:     true,
		ManifestPath: absPath,
		StackPath:    stackPath,
		EnvFilePath:  envFilePath,
		StackName:    stackName(environment, m.Name),
		EnvPath:      envPath,
		Manifest:     m,
	}, nil
}

func resolveEnvFilePath(manifestPath string, envFile string) string {
	return filepath.Join(filepath.Dir(manifestPath), envFile)
}

func (s *Service) deployStack(ctx context.Context, stackPath string, stackName string) error {
	_, err := s.runner.Run(
		ctx,
		"docker",
		[]string{
			"stack",
			"deploy",
			"--compose-file",
			stackPath,
			stackName,
		},
		command.RunOptions{
			LogCommand: true,
		},
	)
	if err != nil {
		return fmt.Errorf("deploy stack %q: %w", stackName, err)
	}

	return nil
}

func (s *Service) verifyService(ctx context.Context, serviceName string) error {
	_, err := s.runner.Run(
		ctx,
		"docker",
		[]string{
			"service",
			"inspect",
			serviceName,
		},
		command.RunOptions{},
	)
	if err != nil {
		return fmt.Errorf("verify service %q: %w", serviceName, err)
	}

	return nil
}
