package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/AustinOyugi/no-oops-ops/internal/catalog"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"github.com/AustinOyugi/no-oops-ops/internal/release"
)

func (a *App) runRelease(ctx context.Context, args []string) error {
	if len(args) > 0 && args[0] == "list" {
		return a.runReleaseList(ctx, args[1:])
	}
	environment, manifestPath, services, deployAfterRelease, err := parseReleaseArgs(args, a.resolveApp)
	if err != nil {
		return err
	}
	releaseTags := make(map[string]string, len(services))
	for _, service := range services {
		result, err := a.runReleaseService(ctx, environment, manifest.WithService(manifestPath, service))
		if err != nil {
			return err
		}
		releaseTags[service] = result.Tag
	}
	if !deployAfterRelease {
		return nil
	}
	if err := a.runDeployPreflight(ctx); err != nil {
		return err
	}
	for _, service := range services {
		if err := a.runDeployService(ctx, environment, manifest.WithService(manifestPath, service), releaseTags[service], false); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) runReleaseList(ctx context.Context, args []string) error {
	environment, manifestPath, services, err := parseServiceArgs(args, "release list", a.resolveApp, true)
	if err != nil {
		return err
	}
	for _, service := range services {
		m, err := manifest.Load(manifest.WithService(manifestPath, service))
		if err != nil {
			return err
		}
		history, err := release.ListHistory(a.config, m.Name, environment)
		if err != nil {
			return err
		}
		if len(history) == 0 {
			a.logger.InfoContext(ctx, "release history", "environment", environment, "service", m.Name, "releases", 0)
			continue
		}
		for _, item := range history {
			a.logger.InfoContext(ctx, "release", "environment", item.Environment, "service", item.App, "tag", item.Tag, "image", item.RegistryImage, "created_at", item.CreateAt, "git_commit", gitCommit(item))
		}
	}
	return nil
}

func gitCommit(metadata release.Metadata) string {
	if metadata.Git == nil {
		return ""
	}
	return metadata.Git.Commit
}

func (a *App) runReleaseService(ctx context.Context, environment, manifestPath string) (release.Result, error) {

	result, err := a.releaser.Run(ctx, environment, manifestPath)
	if err != nil {
		a.logger.ErrorContext(
			ctx,
			"release failed",
			"environment", environment,
			"manifest_path", manifestPath,
			"reason", err.Error(),
		)
		return release.Result{}, err
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

	return result, nil
}

func parseReleaseArgs(args []string, resolveApp func(string) (string, error)) (environment, manifestPath string, services []string, deployAfterRelease bool, err error) {
	serviceArgs := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--deploy" {
			deployAfterRelease = true
			continue
		}
		serviceArgs = append(serviceArgs, arg)
	}
	environment, manifestPath, services, err = parseServiceArgs(serviceArgs, "release", resolveApp, true)
	return environment, manifestPath, services, deployAfterRelease, err
}

func (a *App) resolveApp(name string) (string, error) {
	return catalog.Resolve(a.config.Workspace, name)
}

func parseServiceArgs(args []string, command string, resolveApp func(string) (string, error), allowImplicitSingleService bool) (string, string, []string, error) {
	if len(args) < 2 {
		return "", "", nil, fmt.Errorf("%s requires an environment and app name", command)
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
	if all && selected != "" {
		return "", "", nil, errors.New("provide exactly one of --service or --all")
	}
	if selected == "" && allowImplicitSingleService {
		names, err := manifest.Services(path)
		if err != nil {
			return "", "", nil, err
		}
		if len(names) == 1 {
			return environment, path, names, nil
		}
		return "", "", nil, fmt.Errorf("%s requires --service <name> or --all when the manifest contains multiple services", command)
	}
	if all {
		names, err := manifest.DeploymentOrder(path)
		return environment, path, names, err
	}
	if selected == "" {
		return "", "", nil, errors.New("provide exactly one of --service or --all")
	}
	if _, err := manifest.LoadService(path, selected); err != nil {
		return "", "", nil, err
	}
	return environment, path, []string{selected}, nil
}
