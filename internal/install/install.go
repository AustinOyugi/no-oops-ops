package install

import (
	"context"
	"fmt"
	"log/slog"
)

type Installer struct {
	logger *slog.Logger
	host   Host
}

func New(logger *slog.Logger, host Host) (*Installer, error) {

	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	if host == nil {
		return nil, fmt.Errorf("host is required")
	}

	return &Installer{
		logger: logger,
		host:   host,
	}, nil
}

func (i *Installer) Run(ctx context.Context) error {
	i.logger.InfoContext(ctx, "starting install")

	if err := i.host.VerifyDocker(ctx); err != nil {
		return err
	}

	if err := i.host.PrepareStateDir(ctx); err != nil {
		return err
	}

	i.logger.InfoContext(ctx, "install flow complete")
	return nil
}
