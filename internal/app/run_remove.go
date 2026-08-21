package app

import (
	"context"
	"errors"
)

func (a *App) runRemove(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("remove requires an environment and manifest path")
	}

	result, err := a.deployer.Remove(ctx, args[0], args[1])
	if err != nil {
		a.logger.ErrorContext(ctx, "remove failed", "environment", args[0], "manifest_path", args[1], "reason", err.Error())
		return err
	}

	a.logger.InfoContext(ctx, "application removed",
		"environment", result.Environment,
		"manifest_path", result.ManifestPath,
		"stack_name", result.StackName,
		"registry_images", result.RegistryImages,
		"state_path", result.StatePath,
	)
	return nil
}
