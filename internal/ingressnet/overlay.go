package ingressnet

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

var networkNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// EnsureOverlay reuses a Swarm overlay or creates it when absent. External
// networks remain independent of the managed ingress stack.
func EnsureOverlay(ctx context.Context, runner Runner, name string) error {
	if !networkNamePattern.MatchString(name) {
		return fmt.Errorf("invalid ingress network name %q", name)
	}
	exists, err := inspectOverlay(ctx, runner, name)
	if err != nil || exists {
		return err
	}
	result, err := runner.Run(ctx, "docker", []string{"network", "create", "--driver", "overlay", name}, command.RunOptions{LogCommand: true})
	if err == nil {
		return nil
	}
	if exists, inspectErr := inspectOverlay(ctx, runner, name); inspectErr == nil && exists {
		return nil
	}
	return fmt.Errorf("create ingress overlay network %q: %w: %s", name, err, strings.TrimSpace(string(result.Output)))
}

func inspectOverlay(ctx context.Context, runner Runner, name string) (bool, error) {
	result, err := runner.Run(ctx, "docker", []string{"network", "inspect", "--format", "{{.Driver}}|{{.Scope}}", name}, command.RunOptions{})
	if err != nil {
		output := strings.ToLower(string(result.Output))
		if strings.Contains(output, "not found") || strings.Contains(output, "no such network") {
			return false, nil
		}
		return false, fmt.Errorf("inspect ingress network %q: %w: %s", name, err, result.Output)
	}
	if strings.TrimSpace(string(result.Output)) != "overlay|swarm" {
		return true, fmt.Errorf("ingress network %q must be a Swarm overlay network; found %q", name, strings.TrimSpace(string(result.Output)))
	}
	return true, nil
}
