package local

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

// Two serial updates can each spend two minutes draining old connections,
// plus their health monitor windows. Other platform services retain 45s waits.
const nginxReadyTimeout = 6 * time.Minute

func (h *Host) nginxServiceState(ctx context.Context) (int, string, error) {
	result, err := h.runner.Run(ctx, "docker", []string{"service", "inspect", "--format", "{{.Spec.Mode.Replicated.Replicas}}|{{if .UpdateStatus}}{{.UpdateStatus.State}}{{end}}", h.nginxService}, command.RunOptions{})
	if err != nil {
		return 0, "", fmt.Errorf("inspect nginx service: %w: %s", err, result.Output)
	}
	replicas, status, ok := strings.Cut(strings.TrimSpace(string(result.Output)), "|")
	count, err := strconv.Atoi(replicas)
	if !ok || err != nil {
		return 0, "", fmt.Errorf("invalid nginx service state: %q", result.Output)
	}
	return count, status, nil
}

func (h *Host) prepareNginxUpdate(ctx context.Context) error {
	result, err := h.runner.Run(ctx, "docker", []string{"service", "inspect", h.nginxService}, command.RunOptions{})
	if err != nil {
		if strings.Contains(strings.ToLower(string(result.Output)), "no such service") {
			return nil
		}
		return fmt.Errorf("inspect ingress before update: %w: %s", err, result.Output)
	}
	replicas, _, err := h.nginxServiceState(ctx)
	if err != nil {
		return err
	}
	if replicas < nginxReplicas {
		result, err := h.runner.Run(ctx, "docker", []string{"service", "scale", "--detach=true", h.nginxService + "=" + strconv.Itoa(nginxReplicas)}, command.RunOptions{LogCommand: true})
		if err != nil {
			return fmt.Errorf("prepare redundant nginx replicas: %w: %s", err, result.Output)
		}
		replicas = nginxReplicas
	}
	return h.waitForNginxHealthy(ctx, replicas, false)
}

func (h *Host) waitForNginxHealthy(ctx context.Context, replicas int, updated bool) error {
	checkCtx, cancel := context.WithTimeout(ctx, nginxReadyTimeout)
	defer cancel()
	for {
		ready, err := h.inspectNginxHealthy(checkCtx, replicas, updated)
		if err != nil {
			return err
		}
		if ready {
			return nil
		}
		select {
		case <-checkCtx.Done():
			return fmt.Errorf("wait for %d healthy nginx replicas: %w", replicas, checkCtx.Err())
		case <-time.After(time.Second):
		}
	}
}

func (h *Host) inspectNginxHealthy(ctx context.Context, expected int, updated bool) (bool, error) {
	replicas, status, err := h.nginxServiceState(ctx)
	if err != nil {
		return false, err
	}
	if updated {
		switch status {
		case "paused", "rollback_started", "rollback_paused", "rollback_completed":
			return false, fmt.Errorf("nginx update did not complete: %s", status)
		case "", "completed":
		default:
			return false, nil
		}
	}
	if replicas != expected {
		return false, nil
	}
	// Count desired-running tasks, not old containers still draining connections.
	tasks, err := h.runner.Run(ctx, "docker", []string{"service", "ps", "-q", "--filter", "desired-state=running", h.nginxService}, command.RunOptions{})
	if err != nil {
		return false, fmt.Errorf("inspect nginx tasks: %w: %s", err, tasks.Output)
	}
	ids := strings.Fields(string(tasks.Output))
	if len(ids) != expected {
		return false, nil
	}
	for _, task := range ids {
		result, err := h.runner.Run(ctx, "docker", []string{"inspect", "--type", "task", "--format", "{{.Status.State}}|{{if .Status.ContainerStatus}}{{.Status.ContainerStatus.ContainerID}}{{end}}", task}, command.RunOptions{})
		if err != nil {
			if strings.Contains(strings.ToLower(string(result.Output)), "no such") {
				return false, nil // A task was replaced between the two inspections.
			}
			return false, fmt.Errorf("inspect nginx task %q: %w: %s", task, err, result.Output)
		}
		state, container, ok := strings.Cut(strings.TrimSpace(string(result.Output)), "|")
		if !ok || state != "running" || container == "" {
			return false, nil
		}
		result, err = h.runner.Run(ctx, "docker", []string{"container", "inspect", "--format", "{{if .State.Health}}{{.State.Health.Status}}{{end}}", container}, command.RunOptions{})
		if err != nil {
			if strings.Contains(strings.ToLower(string(result.Output)), "no such") {
				return false, nil // A failed container may disappear during rollout.
			}
			return false, fmt.Errorf("inspect nginx container health %q: %w: %s", container, err, result.Output)
		}
		if strings.TrimSpace(string(result.Output)) != "healthy" {
			return false, nil
		}
	}
	return true, nil
}
