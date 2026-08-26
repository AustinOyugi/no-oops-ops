package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/AustinOyugi/no-oops-ops/internal/catalog"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

func (a *App) runRelease(ctx context.Context, args []string) error {
	environment, manifestPath, services, err := parseServiceArgs(args, "release", a.resolveApp)
	if err != nil {
		return err
	}
	for _, service := range services {
		if err := a.runReleaseService(ctx, environment, manifest.WithService(manifestPath, service)); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) runReleaseService(ctx context.Context, environment, manifestPath string) error {

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
		"git_source", manifest.Build.Source.Git != nil,
		"built", result.Built,
		"build_executed", result.Built,
	)

	return nil
}

func (a *App) resolveApp(name string) (string, error) {
	return catalog.Resolve(a.config.Workspace, name)
}

func parseServiceArgs(args []string, command string, resolveApp func(string) (string, error)) (string, string, []string, error) {
	if len(args) < 3 {
		return "", "", nil, fmt.Errorf("%s requires an environment, app name, and --service <name> or --all", command)
	}
	environment, appName := args[0], args[1]
	path, err := resolveApp(appName)
	if err != nil {
		return "", "", nil, err
	}
	var selected string
	all := false
	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--all":
			all = true
		case "--service":
			i++
			if i == len(args) {
				return "", "", nil, errors.New("--service requires a service name")
			}
			selected = args[i]
		default:
			return "", "", nil, fmt.Errorf("unknown %s option %q", command, args[i])
		}
	}
	if all == (selected != "") {
		return "", "", nil, errors.New("provide exactly one of --service or --all")
	}
	if all {
		names, err := manifest.DeploymentOrder(path)
		return environment, path, names, err
	}
	if _, err := manifest.LoadService(path, selected); err != nil {
		return "", "", nil, err
	}
	return environment, path, []string{selected}, nil
}
