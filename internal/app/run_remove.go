package app

import (
	"context"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

func (a *App) runRemove(ctx context.Context, args []string) error {
	environment, manifestPath, services, err := parseServiceArgs(args, "remove", a.resolveApp, false)
	if err != nil {
		return err
	}
	for _, service := range services {
		if err := a.runRemoveService(ctx, environment, manifest.WithService(manifestPath, service)); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) runRemoveService(ctx context.Context, environment, manifestPath string) error {
	result, err := a.deployer.Remove(ctx, environment, manifestPath)
	if err != nil {
		a.logger.ErrorContext(ctx, "remove failed", "environment", environment, "manifest_path", manifestPath, "reason", err.Error())
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
