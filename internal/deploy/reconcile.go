package deploy

import (
	"context"
	"sort"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

// removeStaleAppStacks reconciles historical blue/green stacks left behind by
// older releases. Stack ownership is deliberately narrow: a candidate must
// have this app/environment's stable stack name or its generated "-r" prefix.
func (s *Service) removeStaleAppStacks(ctx context.Context, environment, appName, activeStack string) {
	result, err := s.runner.Run(ctx, "docker", []string{"service", "ls", "--format", "{{.Name}}"}, command.RunOptions{})
	if err != nil {
		s.logger.WarnContext(ctx, "could not list services for stale stack cleanup", "environment", environment, "name", appName, "error", err)
		return
	}

	for _, stack := range staleAppStacks(string(result.Output), environment, appName, activeStack) {
		if err := s.removeStack(ctx, stack); err != nil {
			s.logger.WarnContext(ctx, "could not remove stale application stack", "environment", environment, "name", appName, "stack", stack, "error", err)
		}
	}
}

func staleAppStacks(serviceList, environment, appName, activeStack string) []string {
	stableStack := stackName(environment, appName)
	stacks := make(map[string]struct{})
	for _, service := range strings.Fields(serviceList) {
		stack, ok := appStackForService(service, stableStack)
		if !ok || stack == activeStack {
			continue
		}
		stacks[stack] = struct{}{}
	}

	result := make([]string, 0, len(stacks))
	for stack := range stacks {
		result = append(result, stack)
	}
	sort.Strings(result)
	return result
}

func appStackForService(service, stableStack string) (string, bool) {
	stack, _, found := strings.Cut(service, "_")
	if !found {
		return "", false
	}
	if stack == stableStack || strings.HasPrefix(stack, stableStack+"-r") {
		return stack, true
	}
	return "", false
}
