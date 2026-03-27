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

type installStep struct {
	name Step
	run  func(context.Context) error
}

func (i *Installer) Run(ctx context.Context) (Result, error) {
	i.logger.InfoContext(ctx, "starting install")

	result := Result{}

	steps := []installStep{
		{name: StepVerifyDocker, run: i.host.VerifyDocker},
		{name: StepEnsureSwarmInitialized, run: i.host.EnsureSwarmInitialized},
		{name: StepEnsureSharedNetwork, run: i.host.EnsureSharedNetwork},
		{name: StepPrepareStateDir, run: i.host.PrepareStateDir},
		{name: StepInitializeLocalState, run: i.host.InitializeLocalState},
		{name: StepWriteRegistryConfig, run: i.host.WriteRegistryConfig},
		{name: StepWriteRegistryStack, run: i.host.WriteRegistryStack},
		{name: StepEnsureRegistry, run: i.host.EnsureRegistry},
		{name: StepWriteInstallMetadata, run: i.host.WriteInstallMetadata},
	}

	for _, step := range steps {
		if err := i.runStep(ctx, &result, step.name, step.run); err != nil {
			return result, err
		}
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
