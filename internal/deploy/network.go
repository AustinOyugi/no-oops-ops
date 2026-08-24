package deploy

import (
	"context"
	"fmt"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

func (s *Service) ensureNetwork(ctx context.Context, network string) error {
	if network == "" {
		return fmt.Errorf("environment network is required")
	}
	if _, err := s.runner.Run(ctx, "docker", []string{"network", "inspect", network}, command.RunOptions{}); err == nil {
		return nil
	}
	result, err := s.runner.Run(ctx, "docker", []string{"network", "create", "--driver", "overlay", network}, command.RunOptions{LogCommand: true})
	if err != nil {
		return fmt.Errorf("create environment network %q: %w: %s", network, err, strings.TrimSpace(string(result.Output)))
	}
	return nil
}
