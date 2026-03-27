package app

import (
	"context"
	"errors"
	"log/slog"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/install"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/logging"
)

type App struct {
	logger    *slog.Logger
	config    config.Config
	installer *install.Installer
}

func New(cfg config.Config) (*App, error) {

	logger := logging.New()

	localHost := install.NewLocalHost(
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
	}, nil
}

func (a *App) Run(ctx context.Context) error {
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
