package local

import (
	"context"
	"fmt"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

func (h *Host) InspectServiceReadiness(ctx context.Context, service string) (int, int, string, error) {
	result, err := h.runner.Run(ctx, "docker", []string{"service", "ps", "--no-trunc", "--format", "{{.DesiredState}}|{{.CurrentState}}|{{.Error}}", service}, command.RunOptions{})
	if err != nil {
		return 0, 0, "", fmt.Errorf("inspect service %q tasks: %w", service, err)
	}
	var desired, running int
	var taskError string
	for _, line := range strings.Split(strings.TrimSpace(string(result.Output)), "\n") {
		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 2 {
			continue
		}
		if taskError == "" && len(parts) == 3 && strings.TrimSpace(parts[2]) != "" {
			taskError = strings.TrimSpace(parts[2])
		}
		desiredState := strings.TrimSpace(parts[0])
		if !isActiveDesiredState(desiredState) {
			continue
		}
		desired++
		if strings.HasPrefix(strings.TrimSpace(parts[1]), "Running") {
			running++
		}
	}
	return desired, running, taskError, nil
}

func isActiveDesiredState(state string) bool {
	switch state {
	case "Running", "Ready", "Accepted", "Preparing", "Starting":
		return true
	default:
		return false
	}
}
