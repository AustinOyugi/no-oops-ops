package deploy

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/ingress"
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
	ingress     *ingress.Service
}

// RunOptions controls behavior for a single deploy without changing the
// manifest's durable rollout policy.
type RunOptions struct {
	// Quick uses the health-check start period as the Swarm monitor window. It is
	// useful for development feedback loops; normal deploys retain the
	// manifest's full monitor period.
	Quick bool
}

func NewService(logger *slog.Logger, cfg config.Config) *Service {
	return &Service{
		logger:      logger,
		config:      cfg,
		runner:      command.NewRunner(logger),
		releases:    release.NewFilesystemStore(),
		deployments: newFilesystemDeploymentStore(),
		secrets:     secret.NewService(logger, cfg),
		ingress:     ingress.NewService(logger, cfg),
	}
}

// SetACMEEmail propagates an interactively configured ACME email to ingress.
func (s *Service) SetACMEEmail(email string) {
	s.ingress.SetACMEEmail(email)
}

func (s *Service) Run(ctx context.Context, environment string, path string, optionalReleaseVersion string) (Result, error) {
	return s.RunWithOptions(ctx, environment, path, optionalReleaseVersion, RunOptions{})
}

func (s *Service) RunWithOptions(ctx context.Context, environment string, path string, optionalReleaseVersion string, options RunOptions) (Result, error) {
	return s.run(ctx, environment, path, optionalReleaseVersion, nil, options)
}

func (s *Service) run(ctx context.Context, environment string, path string, optionalReleaseVersion string, pinnedSecrets []SecretBinding, options RunOptions) (Result, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return Result{}, fmt.Errorf("resolve manifest path %q: %w", path, err)
	}

	s.logger.InfoContext(ctx, "starting deploy", "environment", environment, "version", optionalReleaseVersion)

	m, err := manifest.Load(absPath)
	if err != nil {
		return Result{}, err
	}
	if options.Quick {
		monitor, err := quickRolloutMonitor(m)
		if err != nil {
			return Result{}, err
		}
		m.Rollout.Monitor = monitor
		m.Rollout.Rollback.Monitor = monitor
		s.logger.InfoContext(ctx, "using quick rollout monitor", "monitor", monitor, "convergence_timeout", m.Rollout.ConvergenceTimeout)
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

	activeDeployment, err := s.deployments.Latest(s.config, m.Name, environment)
	if err != nil {
		return Result{}, err
	}
	deploymentStack := stackName(environment, m.Name)
	deploymentService := serviceName(environment, m.Name)
	deploymentSwarmService := swarmServiceName(environment, m.Name)
	deploymentStackPath := stackPath(s.config, m.Name, environment)
	// A blue/green deployment always receives a fresh candidate stack, even
	// when redeploying the same immutable release. This lets Swarm validate the
	// new task before ingress moves away from the currently active stack.
	blueGreen := m.Expose.Enabled && m.Expose.BlueGreenEnabled() && activeDeployment.StackName != ""
	if blueGreen {
		if !m.Expose.Enabled {
			return Result{}, fmt.Errorf("blue-green deployment requires expose.enabled so nginx can promote the candidate service")
		}
		if len(namedVolumes(m.Volumes)) > 0 {
			return Result{}, fmt.Errorf("blue-green deployment does not support named volumes; use a shared external service or deploy this app in place")
		}
		deploymentStack = candidateStackName(environment, m.Name, releaseTag, time.Now().UTC())
		deploymentService = "app"
		deploymentSwarmService = deploymentStack + "_" + deploymentService
		deploymentStackPath = releaseStackPath(s.config, m.Name, environment, deploymentStack)
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
		if len(m.Service.Entrypoint) > 0 {
			imgMeta.Entrypoint = m.Service.Entrypoint
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

	deployedImage := releaseMetadata.RegistryImage
	if wrapperCfg.UseWrapper {
		deployedImage = wrapperCfg.WrapperImage
	}

	stackPath, err := writeStackForService(s.config, environment, m, releaseMetadata.RegistryImage, secretBindings, wrapperCfg, deploymentService, deploymentStackPath)
	if err != nil {
		return Result{}, err
	}

	if err := s.deployStack(ctx, stackPath, deploymentStack); err != nil {
		return Result{}, err
	}

	if err := s.verifyService(ctx, deploymentSwarmService); err != nil {
		if blueGreen {
			_ = s.removeStack(ctx, deploymentStack)
		}
		return Result{}, err
	}

	timeout, monitor, err := convergenceConfig(m)
	if err != nil {
		if blueGreen {
			_ = s.removeStack(ctx, deploymentStack)
		}
		return Result{}, err
	}

	outcome, runningTasks, err := s.waitForSwarmConvergence(
		ctx,
		deploymentSwarmService,
		deployedImage,
		m.Service.Replicas,
		timeout,
		monitor,
	)
	if err != nil {
		if blueGreen {
			_ = s.removeStack(ctx, deploymentStack)
		}
		if outcome == "" {
			outcome = SwarmOutcomeFailed
		}
		failure := Deployment{
			App:            m.Name,
			CreatedAt:      time.Now().UTC(),
			Environment:    environment,
			Outcome:        outcome,
			Reason:         err.Error(),
			ReleaseImage:   releaseMetadata.RegistryImage,
			ReleaseTag:     releaseMetadata.Tag,
			StackName:      deploymentStack,
			ServiceName:    deploymentSwarmService,
			SecretBindings: secretBindings,
		}
		if _, saveErr := s.deployments.Save(s.config, failure); saveErr != nil {
			return Result{}, fmt.Errorf("%w; record deployment outcome: %v", err, saveErr)
		}
		return Result{}, err
	}

	if err := s.ingress.Reconcile(ctx, environment, m, deploymentSwarmService); err != nil {
		return Result{}, fmt.Errorf("reconcile ingress route: %w", err)
	}
	if blueGreen {
		previousStack := activeDeployment.StackName
		if previousStack == "" {
			previousStack = stackName(environment, m.Name)
		}
		if err := s.removeStack(ctx, previousStack); err != nil {
			return Result{}, fmt.Errorf("remove previous active stack %q: %w", previousStack, err)
		}
	}

	deploymentPath, err := s.deployments.Save(s.config, Deployment{
		App:            m.Name,
		CreatedAt:      time.Now().UTC(),
		Environment:    environment,
		Outcome:        outcome,
		ReleaseImage:   releaseMetadata.RegistryImage,
		ReleaseTag:     releaseMetadata.Tag,
		StackName:      deploymentStack,
		ServiceName:    deploymentSwarmService,
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
		ServiceName:    deploymentSwarmService,
		Executed:       true,
		Verified:       true,
		RunningTasks:   runningTasks,
		SwarmOutcome:   outcome,
		ReleaseImage:   releaseMetadata.RegistryImage,
		ReleaseTag:     releaseMetadata.Tag,
		ManifestPath:   absPath,
		StackPath:      stackPath,
		EnvFilePath:    envFilePath,
		StackName:      deploymentStack,
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
	return s.run(ctx, environment, absPath, previous.ReleaseTag, previous.SecretBindings, RunOptions{})
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

func convergenceConfig(m manifest.Manifest) (time.Duration, time.Duration, error) {
	timeout, err := time.ParseDuration(m.Rollout.ConvergenceTimeout)
	if err != nil {
		return 0, 0, fmt.Errorf("parse rollout.convergence_timeout %q: %w", m.Rollout.ConvergenceTimeout, err)
	}

	monitor, err := time.ParseDuration(m.Rollout.Monitor)
	if err != nil {
		return 0, 0, fmt.Errorf("parse rollout.monitor %q: %w", m.Rollout.Monitor, err)
	}

	return timeout, monitor, nil
}

// quickRolloutMonitor uses the health-check start period as the shortest
// viable monitor window. The configured convergence timeout remains unchanged
// so task scheduling time cannot race the monitoring window.
func quickRolloutMonitor(m manifest.Manifest) (string, error) {
	startPeriod, err := time.ParseDuration(m.Healthcheck.StartPeriod)
	if err != nil {
		return "", fmt.Errorf("parse healthcheck.start_period %q: %w", m.Healthcheck.StartPeriod, err)
	}
	return startPeriod.String(), nil
}
