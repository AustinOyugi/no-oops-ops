package app

import (
	"context"
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

	localHost := install.NewLocalHost(logger)

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
		return err
	}

	a.logger.InfoContext(ctx, "install completed", "steps", result.Steps)
	return nil
}
