package release

import (
	"context"
	"fmt"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"os"
	"strings"
)

func (s *Service) buildImage(ctx context.Context, image string, dockerfile string, contextDir string) error {
	result, err := s.runner.Run(
		ctx,
		"docker",
		[]string{
			"build",
			"-t",
			image,
			"-f",
			dockerfile,
			contextDir,
		},
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

func (s *Service) runBuildCommand(ctx context.Context, contextDir string, commandArgs []string) error {
	if len(commandArgs) == 0 {
		return nil
	}

	name := commandArgs[0]
	args := commandArgs[1:]

	result, err := s.runner.Run(
		ctx,
		name,
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
		return fmt.Errorf(
			"run build command %q: %w: %s",
			strings.Join(commandArgs, " "),
			err,
			strings.TrimSpace(string(result.Output)),
		)
	}

	return nil
}
