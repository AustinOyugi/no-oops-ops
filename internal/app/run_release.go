package app

import (
	"context"
	"errors"
)

func (a *App) runRelease(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return errors.New("release requires an environment and manifest path")
	}

	environment := args[0]
	manifestPath := args[1]

	result, err := a.releaser.Run(ctx, environment, manifestPath)
	if err != nil {
		a.logger.ErrorContext(
			ctx,
			"release failed",
			"environment", environment,
			"manifest_path", manifestPath,
			"reason", err.Error(),
		)
		return err
	}

	manifest := result.Manifest

	a.logger.InfoContext(
		ctx,
		"release manifest",
		"path", result.ManifestPath,
		"environment", result.Environment,
		"name", manifest.Name,
		"image", result.Image,
		"tag", result.Tag,
		"registry_image", result.RegistryImage,
		"metadata_path", result.MetadataPath,
		"pushed", result.Pushed,
		"source_context", manifest.Source.Context,
		"source_dockerfile", manifest.Source.Dockerfile,
		"build_command", manifest.Source.Build.Command,
		"prebuild_configured", len(manifest.Source.Build.Command) > 0,
		"built", result.Built,
		"build_executed", result.Built,
	)

	return nil
}
