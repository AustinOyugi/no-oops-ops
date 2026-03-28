package local

import (
	"context"
	"fmt"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/install"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

func (h *Host) inspectSwarmManagerAddress(ctx context.Context) string {
	result, err := h.runner.Run(
		ctx,
		"docker",
		[]string{"info", "--format", "{{.Swarm.NodeAddr}}"},
		command.RunOptions{},
	)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(result.Output))
}

func (h *Host) InspectSwarmState(ctx context.Context) (string, error) {
	result, err := h.runner.Run(
		ctx,
		"docker",
		[]string{"info", "--format", "{{.Swarm.LocalNodeState}}"},
		command.RunOptions{},
	)
	if err != nil {
		return "", fmt.Errorf("inspect swarm state: %w: %s", err, strings.TrimSpace(string(result.Output)))
	}

	return strings.TrimSpace(string(result.Output)), nil
}

func (h *Host) VerifyDocker(ctx context.Context) error {
	h.logger.InfoContext(ctx, "checking docker installation")

	result, err := h.runner.Run(ctx, "docker", []string{"version"}, command.RunOptions{})
	if err != nil {
		return install.PrerequisiteError{
			Check: install.StepVerifyDocker,
			Err:   fmt.Errorf("verify docker: %w: %s", err, strings.TrimSpace(string(result.Output))),
		}
	}

	return nil
}

func (h *Host) EnsureSwarmInitialized(ctx context.Context) error {
	h.logger.InfoContext(ctx, "ensuring swarm is initialized")

	state, err := h.InspectSwarmState(ctx)
	if err != nil {
		return install.PrerequisiteError{
			Check: install.StepEnsureSwarmInitialized,
			Err:   err,
		}
	}

	if state == "active" {
		h.swarmNodeState = state
		h.swarmInitialized = true
		h.swarmManagerAddr = h.inspectSwarmManagerAddress(ctx)
		return nil
	}

	initResult, err := h.runner.Run(
		ctx,
		"docker",
		[]string{"swarm", "init"},
		command.RunOptions{LogCommand: true},
	)
	if err != nil {
		return install.PrerequisiteError{
			Check: install.StepEnsureSwarmInitialized,
			Err:   fmt.Errorf("initialize swarm: %w: %s", err, strings.TrimSpace(string(initResult.Output))),
		}
	}

	h.swarmManagerAddr = h.inspectSwarmManagerAddress(ctx)
	h.swarmInitialized = true
	h.swarmNodeState = "active"

	return nil
}
