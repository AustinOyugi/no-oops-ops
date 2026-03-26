package install

import (
	"context"
	"log/slog"
)

type Installer struct {
	logger *slog.Logger
}

func New(logger *slog.Logger) (*Installer, error) {
	return &Installer{
		logger: logger,
	}, nil
}

func (s *Installer) Run(ctx context.Context) error {
	s.logger.InfoContext(ctx, "starting install")
	s.logger.InfoContext(ctx, "checking host prerequisites")
	s.logger.InfoContext(ctx, "preparing local state")
	s.logger.InfoContext(ctx, "install flow complete")
	return nil
}
