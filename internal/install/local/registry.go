package local

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/install"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

func (h *Host) EnsureRegistry(ctx context.Context) error {
	h.logger.InfoContext(
		ctx,
		"ensuring registry",
		"name", h.registryName,
		"port", h.registryPort,
	)

	_, err := h.runner.Run(
		ctx,
		"docker",
		[]string{"service", "inspect", h.registryName},
		command.RunOptions{},
	)
	if err == nil {
		return nil
	}

	result, err := h.runner.Run(
		ctx,
		"docker",
		[]string{
			"service", "create",
			"--name", h.registryName,
			"--network", h.networkName,
			"--publish", fmt.Sprintf("%s:5000", h.registryPort),
			"registry:2",
		},
		command.RunOptions{
			StreamOutput: true,
			Stdout:       os.Stdout,
			Stderr:       os.Stderr,
		},
	)
	if err != nil {
		return install.PrerequisiteError{
			Check: install.StepEnsureRegistry,
			Err:   fmt.Errorf("create registry service %q: %w: %s", h.registryName, err, strings.TrimSpace(string(result.Output))),
		}
	}

	return nil
}
