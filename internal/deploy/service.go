package deploy

import (
	"context"
	"errors"
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
	"github.com/AustinOyugi/no-oops-ops/internal/state"
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

func (s *Service) run(ctx context.Context, environment string, path string, optionalReleaseVersion string, pinnedSecrets []SecretBinding, options RunOptions) (result Result, operationErr error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return Result{}, fmt.Errorf("resolve manifest path %q: %w", path, err)
	}

	s.logger.InfoContext(ctx, "starting deploy", "environment", environment, "version", optionalReleaseVersion)

	m, err := manifest.Load(absPath)
	if err != nil {
		return Result{}, err
	}
	m = m.ForEnvironment(environment)
	unlock, err := s.operationLock(ctx, m.Name, environment)
	if err != nil {
		return Result{}, err
	}
	defer unlock()
	if err := s.recoverJournal(ctx, m.Name, environment); err != nil {
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
	envFile, err := LoadOptionalEnvFile(envFilePath)
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
	immutableImage := releaseMetadata.ImmutableImage()

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

	progress := RolloutProgress{App: m.Name, Environment: environment, OldService: activeDeployment.ServiceName, NewService: deploymentSwarmService, OldRelease: activeDeployment.ReleaseTag, NewRelease: releaseTag, Traffic: "old", StartedAt: time.Now().UTC()}
	if progress.OldService == "" && activeDeployment.StackName != "" {
		progress.OldService = activeDeployment.StackName + "_" + serviceName(environment, m.Name)
		if activeDeployment.StackName != stackName(environment, m.Name) {
			progress.OldService = activeDeployment.StackName + "_app"
		}
	}
	report := func(stage string) {
		if !blueGreen {
			return
		}
		progress.Stage = stage
		progress.UpdatedAt = time.Now().UTC()
		if err := saveRolloutProgress(s.config, progress); err != nil {
			s.logger.WarnContext(ctx, "write rollout progress", "error", err)
		}
	}
	if blueGreen {
		report("preparing")
		defer func() {
			if operationErr != nil {
				progress.FailedStage = progress.Stage
				progress.Error = operationErr.Error()
				progress.Finished = true
				report("failed")
			}
		}()
	}

	var wrapperCfg WrapperConfig

	if resolutionMode == "env" && len(secretBindings) > 0 {
		if err := pullImage(ctx, s.runner, immutableImage); err != nil {
			return Result{}, fmt.Errorf("pull application image: %w", err)
		}
		imgMeta, err := inspectImage(ctx, s.runner, immutableImage)
		if err != nil {
			return Result{}, fmt.Errorf("inspect application image: %w", err)
		}
		if len(m.Service.Entrypoint) > 0 {
			imgMeta.Entrypoint = m.Service.Entrypoint
		}
		wrapperCfg = BuildWrapperConfig(resolutionMode, immutableImage, imgMeta, m.Service.Command, secretBindings)
		if !wrapperCfg.UseWrapper {
			return Result{}, fmt.Errorf("application image %q has neither an entrypoint nor a command", immutableImage)
		}
		wrappedImage, err := s.buildWrappedImage(ctx, immutableImage, m.Name)
		if err != nil {
			return Result{}, fmt.Errorf("build wrapped application image: %w", err)
		}
		wrapperCfg.WrapperImage = wrappedImage
	}

	deployedImage := immutableImage
	if wrapperCfg.UseWrapper {
		deployedImage = wrapperCfg.WrapperImage
	}

	network := s.config.EnvironmentNetwork(environment)
	if err := s.ensureNetwork(ctx, network); err != nil {
		return Result{}, err
	}
	stackPath, err := writeStackForService(s.config, environment, m, immutableImage, secretBindings, wrapperCfg, network, deploymentService, deploymentStackPath)
	if err != nil {
		return Result{}, err
	}

	journal := operationJournal{Kind: "deploy", Stage: "started", StartedAt: time.Now().UTC(), StackName: deploymentStack, BlueGreen: blueGreen, ReleaseTag: releaseTag}
	if err := saveJournal(s.config, m.Name, environment, journal); err != nil {
		return Result{}, err
	}
	completed := false
	defer func() {
		if completed {
			if err := clearJournal(s.config, m.Name, environment); err != nil {
				s.logger.ErrorContext(ctx, "clear completed operation journal", "error", err)
			}
		}
	}()
	report("starting_candidate")
	if err := s.deployStack(ctx, stackPath, deploymentStack); err != nil {
		return Result{}, err
	}
	journal.Stage = "stack_deployed"
	if err := saveJournal(s.config, m.Name, environment, journal); err != nil {
		return Result{}, err
	}

	if err := s.verifyService(ctx, deploymentSwarmService); err != nil {
		return Result{}, s.cleanupFailedCandidate(ctx, blueGreen, deploymentStack, err)
	}

	timeout, monitor, err := convergenceConfig(m)
	if err != nil {
		return Result{}, s.cleanupFailedCandidate(ctx, blueGreen, deploymentStack, err)
	}

	report("waiting_readiness")
	outcome, runningTasks, err := s.waitForSwarmConvergence(
		ctx,
		deploymentSwarmService,
		deployedImage,
		m.Service.Replicas,
		timeout,
		monitor,
	)
	if err != nil {
		if outcome == SwarmOutcomeTimedOut && !blueGreen {
			if recoveryErr := s.stopTimedOutRollout(ctx, deploymentSwarmService); recoveryErr != nil {
				err = fmt.Errorf("%w; stop timed-out rollout: %v", err, recoveryErr)
			}
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
			ReleaseImage:   immutableImage,
			ReleaseTag:     releaseMetadata.Tag,
			StackName:      deploymentStack,
			ServiceName:    deploymentSwarmService,
			SecretBindings: secretBindings,
		}
		if _, saveErr := s.deployments.Save(s.config, failure); saveErr != nil {
			err = fmt.Errorf("%w; record deployment outcome: %v", err, saveErr)
		}
		return Result{}, s.cleanupFailedCandidate(ctx, blueGreen, deploymentStack, err)
	}

	report("ready")
	if m.Expose.Enabled {
		if err := s.ingress.EnsureNetwork(ctx, network); err != nil {
			return Result{}, s.cleanupFailedCandidate(ctx, blueGreen, deploymentStack, fmt.Errorf("connect ingress to environment network: %w", err))
		}
	}
	progress.Traffic = "unknown"
	report("promoting")
	if err := s.ingress.Reconcile(ctx, environment, m, deploymentSwarmService); err != nil {
		return Result{}, s.cleanupFailedCandidate(ctx, blueGreen, deploymentStack, fmt.Errorf("reconcile ingress route: %w", err))
	}
	progress.Traffic = "new"
	report("ingress_reconciled")
	journal.Stage = "ingress_reconciled"
	if err := saveJournal(s.config, m.Name, environment, journal); err != nil {
		return Result{}, err
	}

	deployment := Deployment{
		App:            m.Name,
		CreatedAt:      time.Now().UTC(),
		Environment:    environment,
		Outcome:        outcome,
		ReleaseImage:   immutableImage,
		ReleaseTag:     releaseMetadata.Tag,
		StackName:      deploymentStack,
		ServiceName:    deploymentSwarmService,
		SecretBindings: secretBindings,
	}
	deploymentPath, err := s.deployments.Save(s.config, deployment)
	if err != nil {
		return Result{}, err
	}
	journal.Stage = "deployment_saved"
	if err := saveJournal(s.config, m.Name, environment, journal); err != nil {
		return Result{}, err
	}
	err = s.releases.SetLatest(s.config, m.Name, release.ActiveRelease{Tag: releaseTag, IsAvailable: true}, environment)
	if err != nil {
		return Result{}, err
	}
	completed = true

	// The new service is recorded and ingress already targets it. Reconcile all
	// older stacks belonging to this app/environment, including candidates left
	// behind by earlier No Oops versions.
	report("cleaning_old_stacks")
	s.removeStaleAppStacks(ctx, environment, m.Name, deploymentStack)
	progress.Finished = true
	report("completed")

	return Result{
		DeploymentPath: deploymentPath,
		Environment:    environment,
		ServiceName:    deploymentSwarmService,
		Executed:       true,
		Verified:       true,
		RunningTasks:   runningTasks,
		SwarmOutcome:   outcome,
		ReleaseImage:   immutableImage,
		ReleaseTag:     releaseMetadata.Tag,
		ManifestPath:   absPath,
		StackPath:      stackPath,
		EnvFilePath:    envFilePath,
		StackName:      deploymentStack,
		EnvPath:        envPath,
		Manifest:       m,
	}, nil
}

func (s *Service) operationLock(ctx context.Context, app, environment string) (func(), error) {
	unlockApp, err := state.AcquireLock(ctx, filepath.Join(appDir(s.config, app, environment), "operation.lock"))
	if err != nil {
		return nil, err
	}
	unlockRegistry, err := state.AcquireLock(ctx, filepath.Join(s.config.StateDir, "registry.lock"))
	if err != nil {
		unlockApp()
		return nil, err
	}
	return func() { unlockRegistry(); unlockApp() }, nil
}

// cleanupFailedCandidate removes a blue/green stack whenever it has been
// created but cannot be promoted. The original error remains first so callers
// see why the deployment failed, while a cleanup problem is still actionable.
func (s *Service) cleanupFailedCandidate(ctx context.Context, blueGreen bool, stack string, cause error) error {
	if !blueGreen {
		return cause
	}
	var recoveryErr *ingress.RecoveryError
	if errors.As(cause, &recoveryErr) {
		return fmt.Errorf("%w; retained candidate stack %q because ingress recovery requires attention", cause, stack)
	}
	if err := s.removeStack(ctx, stack); err != nil {
		return fmt.Errorf("%w; remove failed blue-green candidate stack %q: %v", cause, stack, err)
	}
	return cause
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
	if envFile == "" {
		return ""
	}
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
