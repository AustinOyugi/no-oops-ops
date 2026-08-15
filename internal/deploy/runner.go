package deploy

import (
	"context"
	"fmt"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"strings"
	"time"
)

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
	timeout time.Duration,
	interval time.Duration,
) (int, error) {
	deadline := time.Now().Add(timeout)

	s.logger.InfoContext(
		ctx,
		"waiting for service readiness",
		"service", serviceName,
		"timeout", timeout.String(),
		"interval", interval.String(),
	)

	for {
		runningTasks, err := s.runningTaskCount(ctx, serviceName)
		if err != nil {
			return 0, err
		}

		s.logger.InfoContext(
			ctx,
			"readiness poll",
			"service", serviceName,
			"running_tasks", runningTasks,
		)

		if runningTasks > 0 {

			s.logger.InfoContext(
				ctx,
				"service ready",
				"service", serviceName,
				"running_tasks", runningTasks,
			)

			return runningTasks, nil
		}

		if time.Now().After(deadline) {
			diagnostics, diagErr := s.taskDiagnostics(ctx, serviceName)
			if diagErr != nil {
				s.logger.ErrorContext(
					ctx,
					"service readiness timed out",
					"service", serviceName,
					"timeout", timeout.String(),
				)
				return 0, fmt.Errorf("service %q did not reach a running state within %s", serviceName, timeout)
			}

			s.logger.ErrorContext(
				ctx,
				"service readiness timed out",
				"service", serviceName,
				"timeout", timeout.String(),
				"diagnostics", diagnostics,
			)
			return 0, fmt.Errorf(
				"service %q did not reach a running state within %s: %s",
				serviceName,
				timeout,
				diagnostics,
			)
		}

		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(interval):
		}
	}
}

func (s *Service) taskDiagnostics(ctx context.Context, serviceName string) (string, error) {
	result, err := s.runner.Run(
		ctx,
		"docker",
		[]string{
			"service",
			"ps",
			"--no-trunc",
			"--format",
			"{{.CurrentState}}|{{.Error}}",
			serviceName,
		},
		command.RunOptions{},
	)
	if err != nil {
		return "", fmt.Errorf("inspect task diagnostics for service %q: %w", serviceName, err)
	}

	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(string(result.Output)), "\n") {
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "; "), nil
}
