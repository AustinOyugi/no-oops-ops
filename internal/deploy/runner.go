package deploy

import (
	"context"
	"fmt"
	"os"
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

// rolloutMonitorState separates the time allowed to first converge from the
// configured observation window and its final readiness check. Once convergence has been
// reached, the convergence deadline must no longer cancel the monitor window.
type rolloutMonitorState struct {
	convergenceDeadline time.Time
	monitor             time.Duration
	converged           bool
	monitorDeadline     time.Time
}

func newRolloutMonitorState(now time.Time, convergenceTimeout, monitor time.Duration) rolloutMonitorState {
	return rolloutMonitorState{
		convergenceDeadline: now.Add(convergenceTimeout),
		monitor:             monitor,
	}
}

// observe records one rollout observation. It returns whether convergence has
// timed out, whether the service is currently being monitored, whether the
// monitor window completed, and whether this observation first converged.
func (r *rolloutMonitorState) observe(now time.Time, converged bool) (timedOut, monitoring, completed, justConverged bool) {
	if !r.converged {
		if !now.Before(r.convergenceDeadline) {
			return true, false, false, false
		}
		if !converged {
			return false, false, false, false
		}
		r.converged = true
		r.monitorDeadline = now.Add(r.monitor)
		justConverged = true
	}

	// The configured monitor is a single observation window. At its end,
	// missing tasks or an unfinished update fail instead of extending the wait.
	if !now.Before(r.monitorDeadline) {
		if !converged {
			return true, false, false, false
		}
		return false, true, true, justConverged
	}

	return false, true, false, justConverged
}

// swarmProgress is intentionally limited to fields that reflect rollout
// progress. The image is checked for correctness, but including its digest in
// every log entry makes normal deploy output needlessly noisy.
type swarmProgress struct {
	updateState  string
	runningTasks int
}

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
	monitorState := newRolloutMonitorState(time.Now(), timeout, initialMonitor)
	var lastProgress swarmProgress
	hasLastProgress := false

	s.logger.InfoContext(
		ctx,
		"waiting for Swarm convergence",
		"service", serviceName,
		"desired_tasks", desiredTasks,
		"timeout", timeout.String(),
		"monitor", initialMonitor.String(),
	)
	progressIndicator := newSwarmProgressIndicator(os.Stderr)
	progressIndicator.Start(serviceName, desiredTasks, initialMonitor)
	defer progressIndicator.Stop()

	for {
		// Allow one bounded final observation at the phase boundary so a
		// stable monitor can complete rather than cancelling its last probe.
		observationCtx, cancel := context.WithDeadline(ctx, monitorState.deadline().Add(5*time.Second))
		state, message, image, err := s.serviceUpdateStatus(observationCtx, serviceName)
		if err != nil {
			cancel()
			if ctx.Err() == nil && !time.Now().Before(monitorState.deadline()) {
				return SwarmOutcomeTimedOut, 0, s.convergenceError(ctx, serviceName, SwarmOutcomeTimedOut, "rollout observation deadline exceeded")
			}
			return "", 0, err
		}

		switch state {
		case "rollback_completed":
			cancel()
			progressIndicator.Stop()
			s.logger.WarnContext(ctx, "Swarm rollout rolled back", "service", serviceName, "reason", message)
			return SwarmOutcomeRolledBack, 0, s.convergenceError(ctx, serviceName, SwarmOutcomeRolledBack, message)
		case "paused":
			cancel()
			progressIndicator.Stop()
			s.logger.WarnContext(ctx, "Swarm rollout paused", "service", serviceName, "reason", message)
			return SwarmOutcomePaused, 0, s.convergenceError(ctx, serviceName, SwarmOutcomePaused, message)
		case "rollback_paused":
			cancel()
			progressIndicator.Stop()
			s.logger.WarnContext(ctx, "Swarm rollback paused", "service", serviceName, "reason", message)
			return SwarmOutcomeRollbackPaused, 0, s.convergenceError(ctx, serviceName, SwarmOutcomeRollbackPaused, message)
		}

		runningTasks, err := s.runningTaskCount(observationCtx, serviceName)
		cancel()
		if err != nil {
			if ctx.Err() == nil && !time.Now().Before(monitorState.deadline()) {
				return SwarmOutcomeTimedOut, 0, s.convergenceError(ctx, serviceName, SwarmOutcomeTimedOut, "rollout observation deadline exceeded")
			}
			return "", 0, err
		}
		converged := swarmConverged(state, image, expectedImage, runningTasks, desiredTasks)
		timedOut, monitoring, completed, justConverged := monitorState.observe(time.Now(), converged)
		progressIndicator.Update(state, runningTasks, desiredTasks, monitoring)

		if justConverged && !progressIndicator.active {
			s.logger.InfoContext(ctx, "Swarm desired task count reached; observing monitor window", "service", serviceName, "running_tasks", runningTasks, "desired_tasks", desiredTasks, "monitor", initialMonitor.String())
		}
		if completed {
			progressIndicator.Stop()
			s.logger.InfoContext(ctx, "Swarm convergence complete", "service", serviceName, "running_tasks", runningTasks, "desired_tasks", desiredTasks)
			return SwarmOutcomeCompleted, runningTasks, nil
		}
		if !converged {
			progress := swarmProgress{updateState: state, runningTasks: runningTasks}
			if !progressIndicator.active && (!hasLastProgress || progress != lastProgress) {
				s.logger.InfoContext(ctx, "Swarm rollout progress", "service", serviceName, "update_state", state, "running_tasks", runningTasks, "desired_tasks", desiredTasks)
			}
			lastProgress = progress
			hasLastProgress = true
		}

		if timedOut {
			progressIndicator.Stop()
			s.logger.WarnContext(ctx, "Swarm convergence timed out", "service", serviceName, "running_tasks", runningTasks, "desired_tasks", desiredTasks, "timeout", timeout.String())
			return SwarmOutcomeTimedOut, runningTasks, s.convergenceError(ctx, serviceName, SwarmOutcomeTimedOut, fmt.Sprintf("service did not converge within %s or remain stable for %s", timeout, initialMonitor))
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
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
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

func (r *rolloutMonitorState) deadline() time.Time {
	if r.converged {
		return r.monitorDeadline
	}
	return r.convergenceDeadline
}

func swarmConverged(state, image, expectedImage string, running, desired int) bool {
	return (state == "" || state == "completed") && (image == expectedImage || strings.HasPrefix(image, expectedImage+"@")) && allDesiredTasksRunning(running, desired)
}

// stopTimedOutRollout bounds recovery commands independently of observation.
func (s *Service) stopTimedOutRollout(ctx context.Context, service string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	result, err := s.runner.Run(ctx, "docker", []string{"service", "inspect", "--format", "{{if .PreviousSpec}}true{{else}}false{{end}}", service}, command.RunOptions{})
	if err != nil {
		return fmt.Errorf("inspect previous service spec: %w", err)
	}
	args := []string{"service", "scale", service + "=0"}
	switch strings.TrimSpace(string(result.Output)) {
	case "true":
		args = []string{"service", "update", "--detach", "--rollback", service}
	case "false":
	default:
		return fmt.Errorf("unexpected previous service spec response %q", strings.TrimSpace(string(result.Output)))
	}
	_, err = s.runner.Run(ctx, "docker", args, command.RunOptions{LogCommand: true})
	return err
}
