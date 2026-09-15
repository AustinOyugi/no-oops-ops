package cleanup

import (
	"context"
	"fmt"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

type liveInventory struct {
	services map[string]struct{}
	images   map[string]struct{}
}

func (s *Service) liveServices(ctx context.Context) (liveInventory, error) {
	out, err := s.runner.Run(ctx, "docker", []string{"service", "ls", "-q"}, command.RunOptions{})
	if err != nil {
		return liveInventory{}, err
	}
	result := liveInventory{services: map[string]struct{}{}, images: map[string]struct{}{}}
	for _, id := range strings.Fields(string(out.Output)) {
		service, err := s.runner.Run(ctx, "docker", []string{"service", "inspect", "--format", "{{.Spec.Name}}|{{.Spec.TaskTemplate.ContainerSpec.Image}}", id}, command.RunOptions{})
		if err != nil {
			return liveInventory{}, fmt.Errorf("inspect service %q: %w", id, err)
		}
		name, image, ok := strings.Cut(strings.TrimSpace(string(service.Output)), "|")
		if !ok || name == "" || image == "" {
			return liveInventory{}, fmt.Errorf("inspect service %q: missing name or image", id)
		}
		result.services[name] = struct{}{}
		result.images[image] = struct{}{}

		// During a rolling update the service spec has the new image while old
		// tasks may still be running. Protect every non-terminal task image.
		tasks, err := s.runner.Run(ctx, "docker", []string{"service", "ps", "--no-trunc", "--format", "{{.CurrentState}}|{{.Image}}", id}, command.RunOptions{})
		if err != nil {
			return liveInventory{}, fmt.Errorf("inspect tasks for service %q: %w", id, err)
		}
		for _, line := range strings.Split(string(tasks.Output), "\n") {
			state, taskImage, ok := strings.Cut(strings.TrimSpace(line), "|")
			if ok && taskImage != "" && !terminalTaskState(state) {
				result.images[taskImage] = struct{}{}
			}
		}
	}
	return result, nil
}

func terminalTaskState(state string) bool {
	for _, terminal := range []string{"Shutdown", "Complete", "Failed", "Rejected", "Orphaned", "Remove"} {
		if strings.HasPrefix(strings.TrimSpace(state), terminal) {
			return true
		}
	}
	return false
}

func imageKey(image string) string {
	image = strings.TrimSpace(image)
	if before, _, found := strings.Cut(image, "@"); found {
		return before
	}
	return image
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item]; !ok {
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}
