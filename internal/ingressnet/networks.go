// Package ingressnet reads the actual network attachments of managed ingress.
package ingressnet

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

type Runner interface {
	Run(context.Context, string, []string, command.RunOptions) (command.Result, error)
}

func Attached(ctx context.Context, runner Runner, service string) (map[string]bool, error) {
	result, err := runner.Run(ctx, "docker", []string{"service", "inspect", "--format", "{{json .Spec.TaskTemplate.Networks}}", service}, command.RunOptions{})
	if err != nil {
		if strings.Contains(string(result.Output), "no such service") || strings.Contains(string(result.Output), "not found") {
			return map[string]bool{}, nil
		}
		return nil, fmt.Errorf("inspect ingress network attachments: %w: %s", err, result.Output)
	}
	var attachments []struct{ Target string }
	if err := json.Unmarshal(result.Output, &attachments); err != nil {
		return nil, fmt.Errorf("decode ingress network attachments: %w", err)
	}
	networks := make(map[string]bool)
	for _, attachment := range attachments {
		result, err := runner.Run(ctx, "docker", []string{"network", "inspect", "--format", "{{.Name}}", attachment.Target}, command.RunOptions{})
		if err != nil {
			return nil, fmt.Errorf("inspect attached network %q: %w: %s", attachment.Target, err, result.Output)
		}
		name := strings.TrimSpace(string(result.Output))
		if name == "" {
			return nil, fmt.Errorf("attached network %q has no name", attachment.Target)
		}
		networks[name] = true
	}
	return networks, nil
}
