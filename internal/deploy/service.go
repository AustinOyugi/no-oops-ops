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
	"github.com/AustinOyugi/no-oops-ops/internal/secret"
)

type Service struct {
	logger      *slog.Logger
	config      config.Config
	runner      *command.Runner
	releases    release.Store
	deployments deploymentStore
	secrets     *secret.Service
}

func NewService(logger *slog.Logger, cfg config.Config) *Service {
	return &Service{
		logger:      logger,
		config:      cfg,
		runner:      command.NewRunner(logger),
		releases:    release.NewFilesystemStore(),
		deployments: newFilesystemDeploymentStore(),
		secrets:     secret.NewService(logger, cfg),
	}
}

func (s *Service) Run(ctx context.Context, environment string, path string, optionalReleaseVersion string) (Result, error) {
	return s.run(ctx, environment, path, optionalReleaseVersion, nil)
}

func (s *Service) run(ctx context.Context, environment string, path string, optionalReleaseVersion string, pinnedSecrets []SecretBinding) (Result, error) {
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

	var resolvable []string
	if m.Env.Secrets != nil {
		resolvable = m.Env.Secrets.Resolvable
	}
	resolvedEnv := ResolveEnvFile(envFile, environment, resolvable)

	if err := ValidateResolvableKeys(m, envFile); err != nil {
		return Result{}, err
	}

	secretBindings, err := s.resolveSecretBindings(ctx, environment, resolvedEnv.SecretRefs, pinnedSecrets)
	if err != nil {
		return Result{}, err
	}

	resolutionMode := ""
	if m.Env.Secrets != nil {
		resolutionMode = m.Env.Secrets.Resolution
	}

	envPath, err := writeEnvMap(s.config, m.Name, environment, resolvedEnv.Values)
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

	var wrapperCfg WrapperConfig

	if resolutionMode == "env" && len(secretBindings) > 0 {
		if err := pullImage(ctx, s.runner, releaseMetadata.RegistryImage); err != nil {
			return Result{}, fmt.Errorf("pull application image: %w", err)
		}
		imgMeta, err := inspectImage(ctx, s.runner, releaseMetadata.RegistryImage)
		if err != nil {
			return Result{}, fmt.Errorf("inspect application image: %w", err)
		}
		wrapperCfg = BuildWrapperConfig(resolutionMode, releaseMetadata.RegistryImage, imgMeta, m.Service.Command, secretBindings)
		if !wrapperCfg.UseWrapper {
			return Result{}, fmt.Errorf("application image %q has neither an entrypoint nor a command", releaseMetadata.RegistryImage)
		}
		wrappedImage, err := s.buildWrappedImage(ctx, releaseMetadata.RegistryImage, m.Name)
		if err != nil {
			return Result{}, fmt.Errorf("build wrapped application image: %w", err)
		}
		wrapperCfg.WrapperImage = wrappedImage
	}

	stackPath, err := writeStack(s.config, environment, m, releaseMetadata.RegistryImage, secretBindings, wrapperCfg)
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
		App:            m.Name,
		CreatedAt:      time.Now().UTC(),
		Environment:    environment,
		ReleaseImage:   releaseMetadata.RegistryImage,
		ReleaseTag:     releaseMetadata.Tag,
		SecretBindings: secretBindings,
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
	return s.run(ctx, environment, absPath, previous.ReleaseTag, previous.SecretBindings)
}

func (s *Service) resolveSecretBindings(ctx context.Context, environment string, refs []EnvSecretRef, pinned []SecretBinding) ([]SecretBinding, error) {
	pinnedByKey := make(map[string]SecretBinding, len(pinned))
	for _, binding := range pinned {
		pinnedByKey[binding.EnvKey] = binding
	}

	bindings := make([]SecretBinding, 0, len(refs))
	for _, ref := range refs {
		if binding, ok := pinnedByKey[ref.Key]; ok {
			bindings = append(bindings, binding)
			continue
		}

		metadata, err := s.secrets.Latest(ctx, environment, ref.SecretName)
		if err != nil {
			return nil, fmt.Errorf("resolve secret for environment key %q: %w", ref.Key, err)
		}
		bindings = append(bindings, SecretBinding{
			EnvKey:     ref.Key,
			SecretName: metadata.Key,
			SwarmName:  metadata.SwarmName,
			Version:    metadata.Version,
		})
	}

	return bindings, nil
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
