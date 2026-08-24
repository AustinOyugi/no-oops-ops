package app

import (
	"context"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

func (a *App) runRollback(ctx context.Context, args []string) error {
	environment, manifestPath, services, err := parseServiceArgs(args, "rollback", a.resolveApp)
	if err != nil {
		return err
	}
	for _, service := range services {
		if err := a.runRollbackService(ctx, environment, manifest.WithService(manifestPath, service)); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) runRollbackService(ctx context.Context, environment, manifestPath string) error {
	result, err := a.deployer.Rollback(ctx, environment, manifestPath)

	if err != nil {
		a.logger.ErrorContext(ctx, "rollback failed", "environment", environment,
			"manifest_path", manifestPath, "reason", err.Error())
		return err
	}

	a.logger.InfoContext(
		ctx,
		"rollback complete",
		"environment", result.Environment,
		"stack_name", result.StackName,
		"release_tag", result.ReleaseTag,
		"release_image", result.ReleaseImage,
		"deployment_path", result.DeploymentPath,
	)

	return nil
}
