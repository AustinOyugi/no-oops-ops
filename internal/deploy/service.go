package deploy

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"github.com/AustinOyugi/no-oops-ops/internal/release"
)

type Service struct {
	logger      *slog.Logger
	config      config.Config
	runner      *command.Runner
	releases    release.Store
	deployments deploymentStore
}

func NewService(logger *slog.Logger, cfg config.Config) *Service {
	return &Service{
		logger:      logger,
		config:      cfg,
		runner:      command.NewRunner(logger),
		releases:    release.NewFilesystemStore(),
		deployments: newFilesystemDeploymentStore(),
	}
}

func (s *Service) Run(ctx context.Context, environment string, path string, optionalReleaseVersion string) (Result, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return Result{}, fmt.Errorf("resolve manifest path %q: %w", path, err)
	}

	s.logger.InfoContext(ctx, "starting deploy", "environment", environment, "version", optionalReleaseVersion)

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

	var releaseTag string

	if optionalReleaseVersion != "" {
		releaseTag = optionalReleaseVersion
	} else {
		currentReleaseMetadata, err := s.releases.Latest(s.config, m.Name, environment)
		if err != nil {
			return Result{}, err
		}

		if currentReleaseMetadata.IsAvailable {
			releaseTag = currentReleaseMetadata.Tag
		} else {
			return Result{
				Environment:  environment,
				ServiceName:  serviceName(environment, m.Name),
				Executed:     false,
				Verified:     false,
				ManifestPath: absPath,
				EnvFilePath:  envFilePath,
				StackName:    stackName(environment, m.Name),
				EnvPath:      envPath,
				Manifest:     m,
			}, nil
		}
	}

	releaseMetadata, err := s.releases.Find(s.config, m.Name, environment, releaseTag)

	if err != nil {
		return Result{}, err
	}

	stackPath, err := writeStack(s.config, environment, m, releaseMetadata.RegistryImage)
	if err != nil {
		return Result{}, err
	}

	if err := s.deployStack(ctx, stackPath, stackName(environment, m.Name)); err != nil {
		return Result{}, err
	}

	if err := s.verifyService(ctx, swarmServiceName(environment, m.Name)); err != nil {
		return Result{}, err
	}

	timeout, interval, err := readinessConfig(m)
	if err != nil {
		return Result{}, err
	}

	runningTasks, err := s.waitForRunningTasks(
		ctx,
		swarmServiceName(environment, m.Name),
		m.Service.Replicas,
		timeout,
		interval,
	)
	if err != nil {
		return Result{}, err
	}

	deploymentPath, err := s.deployments.Save(s.config, Deployment{
		App:          m.Name,
		CreatedAt:    time.Now().UTC(),
		Environment:  environment,
		ReleaseImage: releaseMetadata.RegistryImage,
		ReleaseTag:   releaseMetadata.Tag,
	})
	if err != nil {
		return Result{}, err
	}

	err = s.releases.SetLatest(s.config, m.Name, release.ActiveRelease{Tag: releaseTag, IsAvailable: true}, environment)
	if err != nil {
		return Result{}, err
	}

	return Result{
		DeploymentPath: deploymentPath,
		Environment:    environment,
		ServiceName:    serviceName(environment, m.Name),
		Executed:       true,
		Verified:       true,
		RunningTasks:   runningTasks,
		ReleaseImage:   releaseMetadata.RegistryImage,
		ReleaseTag:     releaseMetadata.Tag,
		ManifestPath:   absPath,
		StackPath:      stackPath,
		EnvFilePath:    envFilePath,
		StackName:      stackName(environment, m.Name),
		EnvPath:        envPath,
		Manifest:       m,
	}, nil
}

func (s *Service) Rollback(ctx context.Context, environment string, path string) (Result, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return Result{}, fmt.Errorf("resolve manifest path %q: %w", path, err)
	}

	m, err := manifest.Load(absPath)
	if err != nil {
		return Result{}, err
	}

	previous, err := s.deployments.Previous(s.config, m.Name, environment)
	if err != nil {
		return Result{}, err
	}

	s.logger.InfoContext(ctx, "starting rollback", "manifest", absPath, "environment", environment, "release_tag", previous.ReleaseTag)
	return s.Run(ctx, environment, absPath, previous.ReleaseTag)
}

func resolveEnvFilePath(manifestPath string, envFile string) string {
	return filepath.Join(filepath.Dir(manifestPath), envFile)
}

func readinessConfig(m manifest.Manifest) (time.Duration, time.Duration, error) {
	timeout, err := time.ParseDuration(m.Rollout.ReadinessTimeout)
	if err != nil {
		return 0, 0, fmt.Errorf("parse rollout.readiness_timeout %q: %w", m.Rollout.ReadinessTimeout, err)
	}

	interval, err := time.ParseDuration(m.Rollout.ReadinessInterval)
	if err != nil {
		return 0, 0, fmt.Errorf("parse rollout.readiness_interval %q: %w", m.Rollout.ReadinessInterval, err)
	}

	return timeout, interval, nil
}
