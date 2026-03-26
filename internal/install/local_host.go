package install

import (
	"context"
	"log/slog"
)

type LocalHost struct {
	logger *slog.Logger
}

func NewLocalHost(logger *slog.Logger) *LocalHost {
	return &LocalHost{
		logger: logger,
	}
}

func (h *LocalHost) VerifyDocker(ctx context.Context) error {
	h.logger.InfoContext(ctx, "checking docker installation")
	return nil
}

func (h *LocalHost) PrepareStateDir(ctx context.Context) error {
	h.logger.InfoContext(ctx, "preparing local state directory")
	return nil
}
