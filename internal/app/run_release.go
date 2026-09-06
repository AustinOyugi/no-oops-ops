package app

import (
	"context"

	"github.com/AustinOyugi/no-oops-ops/internal/catalog"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"github.com/AustinOyugi/no-oops-ops/internal/release"
)

// Release builds selected services and optionally deploys those exact releases.
func (a *App) Release(ctx context.Context, target Target, deployAfterRelease bool) error {
	environment, manifestPath, services, err := a.resolveTarget(target, true, false)
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
	deployable := make([]string, 0, len(services))
	for _, service := range services {
		m, err := manifest.LoadService(manifestPath, service)
		if err != nil {
			return err
		}
		if m.ShouldDeploy() {
			deployable = append(deployable, service)
		} else {
			a.logger.InfoContext(ctx, "skipping release-only service deployment", "environment", environment, "service", service)
		}
	}
	if len(deployable) == 0 {
		return nil
	}
	if err := a.runDeployPreflight(ctx); err != nil {
		return err
	}
	for _, service := range deployable {
		if err := a.runDeployService(ctx, environment, manifest.WithService(manifestPath, service), releaseTags[service], false); err != nil {
			return err
		}
	}
	return nil
}

// ListReleases lists releases for selected services.
func (a *App) ListReleases(ctx context.Context, target Target) error {
	environment, manifestPath, services, err := a.resolveTarget(target, true, false)
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

func (a *App) resolveApp(name string) (string, error) {
	return catalog.Resolve(a.config.Workspace, name)
}
