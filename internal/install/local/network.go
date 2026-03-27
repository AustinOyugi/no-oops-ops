package local

import (
	"context"
	"fmt"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/install"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

func (h *Host) EnsureSharedNetwork(ctx context.Context) error {
	h.logger.InfoContext(ctx, "ensuring shared network", "network", h.networkName)

	_, err := h.runner.Run(
		ctx,
		"docker",
		[]string{"network", "inspect", h.networkName},
		command.RunOptions{},
	)
	if err == nil {
		return nil
	}

	result, err := h.runner.Run(
		ctx,
		"docker",
		[]string{"network", "create", "--driver", "overlay", h.networkName},
		command.RunOptions{},
	)
	if err != nil {
		return install.PrerequisiteError{
			Check: install.StepEnsureSharedNetwork,
			Err:   fmt.Errorf("create shared network %q: %w: %s", h.networkName, err, strings.TrimSpace(string(result.Output))),
		}
	}

	return nil
}
