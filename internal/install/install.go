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
		result.Steps = append(result.Steps, StepResult{
			Name:   StepVerifyDocker,
			Status: StatusFailed,
			Error:  err.Error(),
		})
		return result, err
	}

	result.Steps = append(result.Steps, StepResult{
		Name:   StepVerifyDocker,
		Status: StatusCompleted,
	})

	if err := i.host.PrepareStateDir(ctx); err != nil {
		result.Steps = append(result.Steps, StepResult{
			Name:   StepPrepareStateDir,
			Status: StatusFailed,
			Error:  err.Error(),
		})
		return result, err
	}

	result.Steps = append(result.Steps, StepResult{
		Name:   StepPrepareStateDir,
		Status: StatusCompleted,
	})

	i.logger.InfoContext(ctx, "install flow complete")
	return result, nil
}
