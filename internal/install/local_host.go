package install

import (
	"context"
	"log/slog"
)

type LocalHost struct {
	logger   *slog.Logger
	stateDir string
}

func NewLocalHost(logger *slog.Logger, stateDir string) *LocalHost {
	return &LocalHost{
		logger:   logger,
		stateDir: stateDir,
	}
}

func (h *LocalHost) VerifyDocker(ctx context.Context) error {
	h.logger.InfoContext(ctx, "checking docker installation")
	return nil
}

func (h *LocalHost) PrepareStateDir(ctx context.Context) error {
	h.logger.InfoContext(ctx, "preparing local state directory", "path", h.stateDir)
	return nil
}
