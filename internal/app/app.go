package app

import (
	"context"
	"errors"
	"log/slog"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/doctor"
	"github.com/AustinOyugi/no-oops-ops/internal/install"
	"github.com/AustinOyugi/no-oops-ops/internal/install/local"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/logging"
)

type App struct {
	logger    *slog.Logger
	config    config.Config
	installer *install.Installer
	doctor    *doctor.Service
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
		doctor:    doctor.NewService(logger, cfg, localHost),
	}, nil
}

func (a *App) Run(ctx context.Context, args []string) error {
	if len(args) > 0 && args[0] == "doctor" {
		return a.runDoctor(ctx)
	}

	if len(args) > 0 && args[0] == "install" {
		return a.runInstall(ctx)
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
