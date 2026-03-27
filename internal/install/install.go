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

	result.SetStep(StepVerifyDocker, StatusRunning, "")
	if err := i.host.VerifyDocker(ctx); err != nil {
		result.SetStep(StepVerifyDocker, StatusFailed, err.Error())
		return result, err
	}
	result.SetStep(StepVerifyDocker, StatusCompleted, "")

	result.SetStep(StepEnsureSwarmInitialized, StatusRunning, "")
	if err := i.host.EnsureSwarmInitialized(ctx); err != nil {
		result.SetStep(StepEnsureSwarmInitialized, StatusFailed, err.Error())
		return result, err
	}
	result.SetStep(StepEnsureSwarmInitialized, StatusCompleted, "")

	result.SetStep(StepEnsureSharedNetwork, StatusRunning, "")
	if err := i.host.EnsureSharedNetwork(ctx); err != nil {
		result.SetStep(StepEnsureSharedNetwork, StatusFailed, err.Error())
		return result, err
	}
	result.SetStep(StepEnsureSharedNetwork, StatusCompleted, "")

	result.SetStep(StepPrepareStateDir, StatusRunning, "")
	if err := i.host.PrepareStateDir(ctx); err != nil {

		result.SetStep(StepPrepareStateDir, StatusFailed, err.Error())
		return result, err
	}

	result.SetStep(StepPrepareStateDir, StatusCompleted, "")

	result.SetStep(StepInitializeLocalState, StatusRunning, "")
	if err := i.host.InitializeLocalState(ctx); err != nil {
		result.SetStep(StepInitializeLocalState, StatusFailed, err.Error())
		return result, err
	}
	result.SetStep(StepInitializeLocalState, StatusCompleted, "")

	result.SetStep(StepWriteInstallMetadata, StatusRunning, "")
	if err := i.host.WriteInstallMetadata(ctx); err != nil {
		result.SetStep(StepWriteInstallMetadata, StatusFailed, err.Error())
		return result, err
	}
	result.SetStep(StepWriteInstallMetadata, StatusCompleted, "")

	i.logger.InfoContext(ctx, "install flow complete")
	return result, nil
}
