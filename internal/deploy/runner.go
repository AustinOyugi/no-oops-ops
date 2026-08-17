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

func (s *Service) waitForRunningTasks(
	ctx context.Context,
	serviceName string,
	desiredTasks int,
	timeout time.Duration,
	interval time.Duration,
) (int, error) {
	deadline := time.Now().Add(timeout)

	s.logger.InfoContext(
		ctx,
		"waiting for service readiness",
		"service", serviceName,
		"desired_tasks", desiredTasks,
		"timeout", timeout.String(),
		"interval", interval.String(),
	)

	for {
		runningTasks, err := s.runningTaskCount(ctx, serviceName)
		if err != nil {
			return 0, err
		}

		if readinessSatisfied(runningTasks, desiredTasks) {
			healthStatuses, err := s.containerHealthStatuses(ctx, serviceName)
			if err != nil {
				return 0, err
			}

			s.logger.InfoContext(
				ctx,
				"readiness poll",
				"service", serviceName,
				"running_tasks", runningTasks,
				"desired_tasks", desiredTasks,
				"health_statuses", healthStatuses,
			)

			if healthChecksSatisfied(healthStatuses, desiredTasks) {
				s.logger.InfoContext(
					ctx,
					"service ready",
					"service", serviceName,
					"running_tasks", runningTasks,
					"desired_tasks", desiredTasks,
					"health_statuses", healthStatuses,
				)
				return runningTasks, nil
			}
		} else {
			s.logger.InfoContext(
				ctx,
				"readiness poll",
				"service", serviceName,
				"running_tasks", runningTasks,
				"desired_tasks", desiredTasks,
			)
		}

		if time.Now().After(deadline) {
			diagnostics, diagErr := s.taskDiagnostics(ctx, serviceName)
			if diagErr != nil {
				s.logger.ErrorContext(
					ctx,
					"service readiness timed out",
					"service", serviceName,
					"running_tasks", runningTasks,
					"desired_tasks", desiredTasks,
					"timeout", timeout.String(),
				)
				return 0, fmt.Errorf("service %q reached %d/%d running tasks within %s", serviceName, runningTasks, desiredTasks, timeout)
			}

			s.logger.ErrorContext(
				ctx,
				"service readiness timed out",
				"service", serviceName,
				"running_tasks", runningTasks,
				"desired_tasks", desiredTasks,
				"timeout", timeout.String(),
				"diagnostics", diagnostics,
			)
			return 0, fmt.Errorf(
				"service %q reached %d/%d running tasks within %s: %s",
				serviceName,
				runningTasks,
				desiredTasks,
				timeout,
				formatTaskDiagnostics(diagnostics),
			)
		}

		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(interval):
		}
	}
}

func (s *Service) containerHealthStatuses(ctx context.Context, serviceName string) ([]string, error) {
	containers, err := s.runner.Run(
		ctx,
		"docker",
		[]string{
			"ps",
			"--filter", "label=com.docker.swarm.service.name=" + serviceName,
			"--format", "{{.ID}}",
		},
		command.RunOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("find containers for service %q: %w", serviceName, err)
	}

	containerIDs := outputLines(string(containers.Output))
	if len(containerIDs) == 0 {
		return []string{}, nil
	}

	args := append([]string{"inspect", "--format", "{{if .State.Health}}{{.State.Health.Status}}{{else}}missing{{end}}"}, containerIDs...)
	health, err := s.runner.Run(ctx, "docker", args, command.RunOptions{})
	if err != nil {
		return nil, fmt.Errorf("inspect health for service %q: %w", serviceName, err)
	}

	return outputLines(string(health.Output)), nil
}

func healthChecksSatisfied(statuses []string, desiredTasks int) bool {
	if len(statuses) < desiredTasks {
		return false
	}

	for _, status := range statuses {
		if status != "healthy" {
			return false
		}
	}

	return true
}

func outputLines(output string) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func readinessSatisfied(runningTasks int, desiredTasks int) bool {
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
