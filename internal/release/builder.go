package release

import (
	"context"
	"fmt"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"os"
	"sort"
	"strconv"
	"strings"
)

func (s *Service) buildImage(ctx context.Context, image string, dockerfile string, contextDir string, resources manifest.BuildResources, buildArgs map[string]string) error {
	args := []string{"build", "-t", image, "-f", dockerfile}
	if resources.Memory != "" {
		args = append(args, "--memory", resources.Memory)
	}
	if resources.CPUs != "" {
		cpus, _ := strconv.ParseFloat(resources.CPUs, 64)
		args = append(args, "--cpu-period", "100000", "--cpu-quota", strconv.FormatInt(int64(cpus*100000), 10))
	}
	for _, key := range sortedBuildArgKeys(buildArgs) {
		args = append(args, "--build-arg", key+"="+buildArgs[key])
	}
	args = append(args, contextDir)
	result, err := s.runner.Run(
		ctx,
		"docker",
		args,
		command.RunOptions{
			LogCommand:   true,
			Workdir:      contextDir,
			StreamOutput: true,
			Stdout:       os.Stdout,
			Stderr:       os.Stderr,
		},
	)
	if err != nil {
		return fmt.Errorf("build image %q: %w: %s", image, err, strings.TrimSpace(string(result.Output)))
	}

	return nil
}

func sortedBuildArgKeys(buildArgs map[string]string) []string {
	keys := make([]string, 0, len(buildArgs))
	for key := range buildArgs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
