package app

import (
	"context"
	"errors"
)

func (a *App) runRollback(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return errors.New("rollback requires an environment and manifest path")
	}

	environment := args[0]
	manifestPath := args[1]

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
