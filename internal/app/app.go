package app

import (
	"context"
	"log/slog"

	"github.com/AustinOyugi/no-oops-ops/internal/config"

	"github.com/AustinOyugi/no-oops-ops/internal/platform/logging"
)

type App struct {
	logger *slog.Logger
	config config.Config
}

func New(cfg config.Config) *App {
	return &App{
		logger: logging.New(),
		config: cfg,
	}
}

func (a *App) Run(ctx context.Context) error {
	a.logger.InfoContext(ctx, "starting noops", "app_name", a.config.AppName)
	return nil
}
