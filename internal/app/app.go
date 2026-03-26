package app

import (
	"context"
	"log/slog"

	"github.com/AustinOyugi/no-oops-ops/internal/platform/logging"
)

type App struct {
	logger *slog.Logger
}

func New() *App {
	return &App{
		logger: logging.New(),
	}
}

func (a *App) Run(ctx context.Context) error {
	_ = ctx
	a.logger.InfoContext(ctx, "starting noops")
	return nil
}
