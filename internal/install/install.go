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

	if err := i.runStep(ctx, &result, StepVerifyDocker, i.host.VerifyDocker); err != nil {
		return result, err
	}

	if err := i.runStep(ctx, &result, StepEnsureSwarmInitialized, i.host.EnsureSwarmInitialized); err != nil {
		return result, err
	}

	if err := i.runStep(ctx, &result, StepEnsureSharedNetwork, i.host.EnsureSharedNetwork); err != nil {
		return result, err
	}

	if err := i.runStep(ctx, &result, StepEnsureRegistry, i.host.EnsureRegistry); err != nil {
		return result, err
	}

	if err := i.runStep(ctx, &result, StepPrepareStateDir, i.host.PrepareStateDir); err != nil {
		return result, err
	}

	if err := i.runStep(ctx, &result, StepInitializeLocalState, i.host.InitializeLocalState); err != nil {
		return result, err
	}

	if err := i.runStep(ctx, &result, StepWriteInstallMetadata, i.host.WriteInstallMetadata); err != nil {
		return result, err
	}

	i.logger.InfoContext(ctx, "install flow complete")
	return result, nil
}

func (i *Installer) runStep(
	ctx context.Context,
	result *Result,
	step Step,
	fn func(context.Context) error,
) error {
	result.SetStep(step, StatusRunning, "")

	if err := fn(ctx); err != nil {
		result.SetStep(step, StatusFailed, err.Error())
		return err
	}

	result.SetStep(step, StatusCompleted, "")
	return nil
}
