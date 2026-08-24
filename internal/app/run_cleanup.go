package app

import (
	"context"
	"errors"
	"strconv"

	"github.com/AustinOyugi/no-oops-ops/internal/cleanup"
)

func (a *App) runCleanup(ctx context.Context, args []string) error {
	options := cleanup.Options{Keep: 2}
	for len(args) > 0 {
		switch args[0] {
		case "--apply":
			options.Apply = true
			args = args[1:]
		case "--keep":
			if len(args) < 2 {
				return errors.New("cleanup --keep requires a value")
			}
			keep, err := strconv.Atoi(args[1])
			if err != nil || keep < 0 {
				return errors.New("cleanup --keep must be a non-negative integer")
			}
			options.Keep = keep
			args = args[2:]
		default:
			return errors.New("cleanup accepts only --apply and --keep <count>")
		}
	}
	plan, err := a.cleaner.Run(ctx, options)
	if err != nil {
		return err
	}
	for _, image := range plan.Images {
		a.logger.InfoContext(ctx, "cleanup registry image candidate", "image", image)
	}
	for _, path := range plan.ReleasePaths {
		a.logger.InfoContext(ctx, "cleanup release record candidate", "path", path)
	}
	for _, path := range plan.DeploymentPaths {
		a.logger.InfoContext(ctx, "cleanup deployment record candidate", "path", path)
	}
	a.logger.InfoContext(ctx, "cleanup plan", "apply", options.Apply, "protected_images", plan.Protected, "release_records", len(plan.ReleasePaths), "deployment_records", len(plan.DeploymentPaths), "registry_images", len(plan.Images))
	return nil
}
