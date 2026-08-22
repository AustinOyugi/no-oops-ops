package deploy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

type TaskDiagnostic struct {
	ID           string `json:"id"`
	Node         string `json:"node"`
	DesiredState string `json:"desired_state"`
	CurrentState string `json:"current_state"`
	Error        string `json:"error"`
}

type SwarmOutcome string

const (
	SwarmOutcomeCompleted      SwarmOutcome = "completed"
	SwarmOutcomeRolledBack     SwarmOutcome = "rolled_back"
	SwarmOutcomePaused         SwarmOutcome = "paused"
	SwarmOutcomeRollbackPaused SwarmOutcome = "rollback_paused"
	SwarmOutcomeTimedOut       SwarmOutcome = "timed_out"
	SwarmOutcomeFailed         SwarmOutcome = "failed"
)

const swarmObservationInterval = 2 * time.Second

type swarmConvergenceError struct {
	Outcome     SwarmOutcome
	Diagnostics []TaskDiagnostic
	Reason      string
}

func (e *swarmConvergenceError) Error() string {
	if len(e.Diagnostics) == 0 {
		return e.Reason
	}
	return fmt.Sprintf("%s: %s", e.Reason, formatTaskDiagnostics(e.Diagnostics))
}

func (s *Service) deployStack(ctx context.Context, stackPath string, stackName string) error {
	_, err := s.runner.Run(
		ctx,
		"docker",
		[]string{
			"stack",
			"deploy",
			"--compose-file",
			stackPath,
			stackName,
		},
		command.RunOptions{
			LogCommand: true,
		},
	)
	if err != nil {
		return fmt.Errorf("deploy stack %q: %w", stackName, err)
	}

	return nil
}

func (s *Service) verifyService(ctx context.Context, serviceName string) error {
	_, err := s.runner.Run(
		ctx,
		"docker",
		[]string{
			"service",
			"inspect",
			serviceName,
		},
		command.RunOptions{},
	)
	if err != nil {
		return fmt.Errorf("verify service %q: %w", serviceName, err)
	}

	return nil
}

func (s *Service) runningTaskCount(ctx context.Context, serviceName string) (int, error) {
	result, err := s.runner.Run(
		ctx,
		"docker",
		[]string{
			"service",
			"ps",
			"--filter",
			"desired-state=running",
			"--format",
			"{{.CurrentState}}",
			serviceName,
		},
		command.RunOptions{},
	)
	if err != nil {
		return 0, fmt.Errorf("inspect running tasks for service %q: %w", serviceName, err)
	}

	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(result.Output)), "\n") {
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "Running") {
			count++
		}
	}

	return count, nil
}

// waitForSwarmConvergence observes Swarm's update state instead of inspecting
// individual container health. Swarm owns health checks, task replacement, and
// rollback; No Oops Ops records the resulting deployment outcome.
func (s *Service) waitForSwarmConvergence(
	ctx context.Context,
	serviceName string,
	expectedImage string,
	desiredTasks int,
	timeout time.Duration,
	initialMonitor time.Duration,
) (SwarmOutcome, int, error) {
	deadline := time.Now().Add(timeout)
	var stableSince time.Time

	s.logger.InfoContext(
		ctx,
		"waiting for Swarm convergence",
		"service", serviceName,
		"desired_tasks", desiredTasks,
		"timeout", timeout.String(),
		"monitor", initialMonitor.String(),
	)

	for {
		state, message, image, err := s.serviceUpdateStatus(ctx, serviceName)
		if err != nil {
			return "", 0, err
		}

		switch state {
		case "completed":
			if strings.HasPrefix(image, expectedImage) {
				return SwarmOutcomeCompleted, desiredTasks, nil
			}
		case "rollback_completed":
			return SwarmOutcomeRolledBack, 0, s.convergenceError(ctx, serviceName, SwarmOutcomeRolledBack, message)
		case "paused":
			return SwarmOutcomePaused, 0, s.convergenceError(ctx, serviceName, SwarmOutcomePaused, message)
		case "rollback_paused":
			return SwarmOutcomeRollbackPaused, 0, s.convergenceError(ctx, serviceName, SwarmOutcomeRollbackPaused, message)
		}

		runningTasks, err := s.runningTaskCount(ctx, serviceName)
		if err != nil {
			return "", 0, err
		}

		if state == "" && allDesiredTasksRunning(runningTasks, desiredTasks) {
			if stableSince.IsZero() {
				stableSince = time.Now()
			}
			if time.Since(stableSince) >= initialMonitor {
				return SwarmOutcomeCompleted, runningTasks, nil
			}
		} else {
			stableSince = time.Time{}
		}

		s.logger.InfoContext(ctx, "Swarm convergence poll", "service", serviceName, "update_state", state, "service_image", image, "running_tasks", runningTasks, "desired_tasks", desiredTasks)

		if time.Now().After(deadline) {
			return SwarmOutcomeTimedOut, runningTasks, s.convergenceError(ctx, serviceName, SwarmOutcomeTimedOut, fmt.Sprintf("service did not converge within %s", timeout))
		}

		select {
		case <-ctx.Done():
			return "", 0, ctx.Err()
		case <-time.After(swarmObservationInterval):
		}
	}
}

func (s *Service) serviceUpdateStatus(ctx context.Context, serviceName string) (string, string, string, error) {
	result, err := s.runner.Run(ctx, "docker", []string{"service", "inspect", "--format", "{{if .UpdateStatus}}{{.UpdateStatus.State}}{{end}}|{{if .UpdateStatus}}{{.UpdateStatus.Message}}{{end}}|{{.Spec.TaskTemplate.ContainerSpec.Image}}", serviceName}, command.RunOptions{})
	if err != nil {
		return "", "", "", fmt.Errorf("inspect Swarm update status for service %q: %w", serviceName, err)
	}

	state, message, image := parseServiceUpdateStatus(string(result.Output))
	return state, message, image, nil
}

func parseServiceUpdateStatus(output string) (string, string, string) {
	parts := strings.SplitN(strings.TrimSpace(output), "|", 3)
	for len(parts) < 3 {
		parts = append(parts, "")
	}
	return parts[0], parts[1], parts[2]
}

func (s *Service) convergenceError(ctx context.Context, serviceName string, outcome SwarmOutcome, reason string) error {
	diagnostics, err := s.taskDiagnostics(ctx, serviceName)
	if err != nil {
		return fmt.Errorf("inspect diagnostics after Swarm outcome %q: %w", outcome, err)
	}
	return &swarmConvergenceError{Outcome: outcome, Diagnostics: diagnostics, Reason: reason}
}

func allDesiredTasksRunning(runningTasks int, desiredTasks int) bool {
	return runningTasks >= desiredTasks
}

func (s *Service) taskDiagnostics(ctx context.Context, serviceName string) ([]TaskDiagnostic, error) {
	result, err := s.runner.Run(
		ctx,
		"docker",
		[]string{
			"service",
			"ps",
			"--no-trunc",
			"--format",
			"{{.ID}}|{{.Node}}|{{.DesiredState}}|{{.CurrentState}}|{{.Error}}",
			serviceName,
		},
		command.RunOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("inspect task diagnostics for service %q: %w", serviceName, err)
	}

	return parseTaskDiagnostics(string(result.Output)), nil
}

func parseTaskDiagnostics(output string) []TaskDiagnostic {
	var diagnostics []TaskDiagnostic
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}

		fields := strings.SplitN(line, "|", 5)
		if len(fields) != 5 {
			diagnostics = append(diagnostics, TaskDiagnostic{CurrentState: line})
			continue
		}

		diagnostics = append(diagnostics, TaskDiagnostic{
			ID:           fields[0],
			Node:         fields[1],
			DesiredState: fields[2],
			CurrentState: fields[3],
			Error:        fields[4],
		})
	}

	return diagnostics
}

func formatTaskDiagnostics(diagnostics []TaskDiagnostic) string {
	parts := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		parts = append(parts, fmt.Sprintf("task=%s node=%s desired=%s current=%s error=%s", diagnostic.ID, diagnostic.Node, diagnostic.DesiredState, diagnostic.CurrentState, diagnostic.Error))
	}

	return strings.Join(parts, "; ")
}
