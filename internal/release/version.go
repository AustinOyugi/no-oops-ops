package release

import (
	"context"
	"fmt"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"strings"
)

func (s *Service) tagImage(ctx context.Context, sourceImage string, targetImage string) error {
	result, err := s.runner.Run(
		ctx,
		"docker",
		[]string{
			"tag",
			sourceImage,
			targetImage,
		},
		command.RunOptions{
			LogCommand: true,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"tag image %q as %q: %w: %s",
			sourceImage,
			targetImage,
			err,
			strings.TrimSpace(string(result.Output)),
		)
	}

	return nil
}
