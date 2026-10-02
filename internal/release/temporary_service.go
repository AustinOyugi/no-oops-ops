package release

import (
	"context"
	"strings"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

// Cleanup must survive cancellation of the release, but cannot block forever
// when the Docker daemon is unavailable.
func (s *Service) removeTemporaryService(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := s.runner.Run(ctx, "docker", []string{"service", "rm", name}, command.RunOptions{})
	if err != nil {
		s.logger.Warn("could not remove temporary Swarm service", "service", name, "error", err, "output", strings.TrimSpace(string(result.Output)))
	}
}
