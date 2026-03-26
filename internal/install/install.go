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

func (i *Installer) Run(ctx context.Context) (Result, error) {
	i.logger.InfoContext(ctx, "starting install")

	result := Result{}

	if err := i.host.VerifyDocker(ctx); err != nil {
		return Result{}, err
	}

	result.Steps = append(result.Steps, "verify_docker")

	if err := i.host.PrepareStateDir(ctx); err != nil {
		return Result{}, err
	}

	result.Steps = append(result.Steps, "prepare_state_dir")

	i.logger.InfoContext(ctx, "install flow complete")
	return result, nil
}
