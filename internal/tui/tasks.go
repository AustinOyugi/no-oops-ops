package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

type Task struct {
	ID, Node, State, Error string
	StartedAt              time.Time
}

// Tasks includes retained task history so failed replacements remain visible.
// Inspect supplies the exact running-state timestamp rather than parsing Docker's
// human-readable relative time.
func Tasks(ctx context.Context, service string) ([]Task, error) {
	output, err := exec.CommandContext(ctx, "docker", "service", "ps", "--no-trunc", "--format", "{{json .}}", service).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("Docker tasks: %w: %s", err, strings.TrimSpace(string(output)))
	}
	tasks, err := parseTasks(string(output))
	if err != nil {
		return nil, err
	}
	args := []string{"inspect", "--type", "task"}
	for _, task := range tasks {
		if task.State == "running" {
			args = append(args, task.ID)
		}
	}
	if len(args) > 3 {
		output, err = exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("Docker task timestamps: %w: %s", err, strings.TrimSpace(string(output)))
		}
		var inspected []struct {
			ID     string
			Status struct {
				State     string
				Timestamp time.Time
			}
		}
		if err := json.Unmarshal(output, &inspected); err != nil {
			return nil, fmt.Errorf("decode task timestamps: %w", err)
		}
		timestamps := map[string]time.Time{}
		for _, task := range inspected {
			if task.Status.State == "running" {
				timestamps[task.ID] = task.Status.Timestamp
			}
		}
		for i := range tasks {
			tasks[i].StartedAt = timestamps[tasks[i].ID]
		}
	}
	return tasks, nil
}

func parseTasks(output string) ([]Task, error) {
	var tasks []Task
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		var item struct{ ID, Node, CurrentState, Error string }
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("decode Docker task: %w", err)
		}
		state := "unknown"
		if fields := strings.Fields(item.CurrentState); len(fields) > 0 {
			state = strings.ToLower(fields[0])
		}
		tasks = append(tasks, Task{ID: item.ID, Node: item.Node, State: state, Error: item.Error})
	}
	// Put current running instances before retained history, keeping order stable.
	sort.SliceStable(tasks, func(i, j int) bool {
		if (tasks[i].State == "running") != (tasks[j].State == "running") {
			return tasks[i].State == "running"
		}
		return tasks[i].ID < tasks[j].ID
	})
	return tasks, nil
}

func uptime(task Task, now time.Time) string {
	if task.State != "running" || task.StartedAt.IsZero() {
		return "—"
	}
	age := now.Sub(task.StartedAt)
	if age < 0 {
		age = 0
	}
	minutes := int(age / time.Minute)
	if minutes >= 1440 {
		return fmt.Sprintf("%dd %dh", minutes/1440, minutes%1440/60)
	}
	if minutes >= 60 {
		return fmt.Sprintf("%dh %dm", minutes/60, minutes%60)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%ds", int(age/time.Second))
}
