package local

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/install"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

func (h *Host) inspectRegistryService(ctx context.Context) bool {
	err := h.InspectRegistryService(ctx)
	return err == nil
}

func (h *Host) InspectRegistryService(ctx context.Context) error {
	result, err := h.runner.Run(
		ctx,
		"docker",
		[]string{"service", "inspect", h.registryService},
		command.RunOptions{},
	)
	if err != nil {
		return fmt.Errorf("inspect registry service %q: %w: %s", h.registryService, err, strings.TrimSpace(string(result.Output)))
	}

	return nil
}

func (h *Host) EnsureRegistry(ctx context.Context) error {
	h.logger.InfoContext(
		ctx,
		"ensuring registry",
		"name", h.registryName,
		"port", h.registryPort,
	)

	if h.inspectRegistryService(ctx) {
		h.registryReady = true
		return nil
	}

	result, err := h.runner.Run(
		ctx,
		"docker",
		[]string{
			"stack", "deploy",
			"--detach=true",
			"--compose-file", h.registryStackPath(),
			h.registryName,
		},
		command.RunOptions{
			StreamOutput: true,
			LogCommand:   true,
			Stdout:       os.Stdout,
			Stderr:       os.Stderr,
		},
	)
	if err != nil {
		return install.PrerequisiteError{
			Check: install.StepEnsureRegistry,
			Err:   fmt.Errorf("deploy registry stack %q: %w: %s", h.registryName, err, strings.TrimSpace(string(result.Output))),
		}
	}

	h.registryReady = h.inspectRegistryService(ctx)

	return nil
}
