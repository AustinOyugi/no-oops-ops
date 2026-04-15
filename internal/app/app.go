package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/deploy"
	"github.com/AustinOyugi/no-oops-ops/internal/doctor"
	"github.com/AustinOyugi/no-oops-ops/internal/install"
	"github.com/AustinOyugi/no-oops-ops/internal/install/local"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/logging"
	"github.com/AustinOyugi/no-oops-ops/internal/status"
)

type App struct {
	logger    *slog.Logger
	config    config.Config
	installer *install.Installer
	deployer  *deploy.Service
	doctor    *doctor.Service
	status    *status.Service
}

func New(cfg config.Config) (*App, error) {

	logger := logging.New()

	localHost := local.NewHost(
		logger, cfg.StateDir, cfg.InstallVersion,
		cfg.NetworkName, cfg.RegistryName, cfg.RegistryPort)

	installer, err := install.New(logger, localHost)

	if err != nil {
		return nil, err
	}

	return &App{
		logger:    logger,
		config:    cfg,
		installer: installer,
		deployer:  deploy.NewService(logger, cfg),
		doctor:    doctor.NewService(logger, cfg, localHost),
		status:    status.NewService(logger, cfg, localHost),
	}, nil
}

func (a *App) Run(ctx context.Context, args []string) error {
	if len(args) > 0 && args[0] == "doctor" {
		return a.runDoctor(ctx)
	}

	if len(args) > 0 && args[0] == "status" {
		return a.runStatus(ctx)
	}

	if len(args) > 0 && args[0] == "install" {
		return a.runInstall(ctx)
	}

	if len(args) > 0 && args[0] == "deploy" {
		return a.runDeploy(ctx, args[1:])
	}

	if len(args) > 0 {
		a.logger.ErrorContext(ctx, "unknown command", "command", args[0])
		return errors.New("unknown command")
	}

	return a.runInstall(ctx)
}

func (a *App) runInstall(ctx context.Context) error {
	a.logger.InfoContext(ctx, "starting noops", "app_name", a.config.AppName)
	result, err := a.installer.Run(ctx)

	if err != nil {
		var prereqErr install.PrerequisiteError
		if errors.As(err, &prereqErr) {
			a.logger.ErrorContext(
				ctx,
				"install prerequisite failed",
				"check", prereqErr.Check,
				"reason", prereqErr.Error(),
			)
		}

		return err
	}

	lastStep, ok := result.LastStep()
	if ok {
		a.logger.InfoContext(
			ctx,
			"install last step",
			"name", lastStep.Name,
			"status", lastStep.Status,
		)
	}

	a.logger.InfoContext(
		ctx,
		"install completed",
		"completed_steps", result.CompletedCount(),
		"failed", result.Failed(),
		"steps", result.Steps,
	)

	return nil
}

func (a *App) runDoctor(ctx context.Context) error {
	a.logger.InfoContext(ctx, "starting noops doctor", "app_name", a.config.AppName)

	result, err := a.doctor.Run(ctx)
	if err != nil {
		return err
	}

	for _, check := range result.Checks {
		if check.Status == doctor.StatusFail {
			a.logger.ErrorContext(ctx, "doctor check", "name", check.Name, "status", check.Status, "message", check.Message)
			continue
		}

		a.logger.InfoContext(ctx, "doctor check", "name", check.Name, "status", check.Status, "message", check.Message)
	}

	if result.Failed() {
		return errors.New("doctor failed")
	}

	a.logger.InfoContext(ctx, "doctor completed", "checks", len(result.Checks), "failed", result.Failed())
	return nil
}

func (a *App) runStatus(ctx context.Context) error {
	a.logger.InfoContext(ctx, "starting noops status", "app_name", a.config.AppName)

	result, err := a.status.Run(ctx)
	if err != nil {
		return err
	}

	a.logger.InfoContext(
		ctx,
		"status metadata",
		"version", result.Metadata.Version,
		"installed_at", result.Metadata.InstalledAt,
		"swarm_state", result.Metadata.Swarm.LocalNodeState,
		"network", result.Metadata.Network.Name,
		"registry", result.Metadata.Registry.Name,
		"registry_port", result.Metadata.Registry.Port,
	)

	for _, component := range result.Components {
		if component.Status == status.ComponentStatusMissing {
			a.logger.ErrorContext(
				ctx,
				"status component",
				"name", component.Name,
				"status", component.Status,
				"message", component.Message,
			)
			continue
		}

		a.logger.InfoContext(
			ctx,
			"status component",
			"name", component.Name,
			"status", component.Status,
			"message", component.Message,
		)
	}

	a.logger.InfoContext(ctx, "status completed", "components", len(result.Components))
	return nil
}

func (a *App) runDeploy(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("deploy requires a manifest path")
	}

	result, err := a.deployer.Run(ctx, args[0])
	if err != nil {
		return err
	}

	manifest := result.Manifest

	a.logger.InfoContext(
		ctx,
		"deploy manifest",
		"path", result.ManifestPath,
		"name", manifest.Name,
		"image", fmt.Sprintf("%s:%s", manifest.Image.Repository, manifest.Image.Tag),
		"internal_port", manifest.Service.InternalPort,
		"replicas", manifest.Service.Replicas,
		"network", manifest.Service.Network,
	)

	a.logger.InfoContext(
		ctx,
		"deploy healthcheck",
		"test", manifest.Healthcheck.Test,
		"interval", manifest.Healthcheck.Interval,
		"timeout", manifest.Healthcheck.Timeout,
		"retries", manifest.Healthcheck.Retries,
		"start_period", manifest.Healthcheck.StartPeriod,
	)

	a.logger.InfoContext(
		ctx,
		"deploy rollout",
		"order", manifest.Rollout.Order,
		"parallelism", manifest.Rollout.Parallelism,
		"delay", manifest.Rollout.Delay,
		"failure_action", manifest.Rollout.FailureAction,
		"restart_condition", manifest.Rollout.RestartCondition,
	)

	a.logger.InfoContext(
		ctx,
		"deploy inputs",
		"depends_on", manifest.DependsOn,
		"secrets", manifest.Secrets,
		"volumes", manifest.Volumes,
	)

	a.logger.InfoContext(
		ctx,
		"deploy env source",
		"env_file_path", result.EnvFilePath,
	)

	a.logger.InfoContext(
		ctx,
		"deploy artifact",
		"stack_path", result.StackPath,
	)

	a.logger.InfoContext(
		ctx,
		"deploy env artifact",
		"env_path", result.EnvPath,
	)

	return nil
}
