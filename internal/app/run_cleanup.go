package app

import (
	"context"

	"github.com/AustinOyugi/no-oops-ops/internal/cleanup"
)

// Cleanup removes expired deployment artifacts according to options.
func (a *App) Cleanup(ctx context.Context, options cleanup.Options) error {
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
	a.logger.InfoContext(ctx, "cleanup plan", "apply", options.Apply, "orphaned", options.Orphaned, "protected_images", plan.Protected, "release_records", len(plan.ReleasePaths), "deployment_records", len(plan.DeploymentPaths), "registry_images", len(plan.Images))
	return nil
}
