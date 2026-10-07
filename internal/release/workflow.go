package release

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"github.com/AustinOyugi/no-oops-ops/internal/state"
)

func (s *Service) acquireReleaseLocks(ctx context.Context, app, environment string) (func(), error) {
	operation, err := state.AcquireLock(ctx, filepath.Join(s.config.StateDir, "apps", app, environment, "operation.lock"))
	if err != nil {
		return nil, err
	}

	registry, err := state.AcquireLock(ctx, filepath.Join(s.config.StateDir, "registry.lock"))
	if err != nil {
		operation()
		return nil, err
	}

	build, err := s.acquireBuildLock(ctx)
	if err != nil {
		registry()
		operation()
		return nil, err
	}

	return func() { build(); registry(); operation() }, nil
}

func (s *Service) releaseBuild(ctx context.Context, environment, manifestPath string, m manifest.Manifest, options Options) (Result, error) {
	contextDir, dockerfile, git, cleanup, err := s.resolveBuildSource(ctx, environment, manifestPath, m)
	if err != nil {
		return Result{}, err
	}
	defer cleanup()

	tag, err := deterministicTag(m, environment, contextDir, dockerfile, git)
	if err != nil {
		return Result{}, err
	}

	if err := s.ensureTagUnused(ctx, m.Name, environment, tag, options); err != nil {
		return Result{}, err
	}

	image, registry := fmt.Sprintf("%s:%s", m.Image.Repository, tag), registryImage(s.config, fmt.Sprintf("%s:%s", m.Image.Repository, tag))
	buildCtx, cancel, err := buildContext(ctx, m.Build.Timeout)
	if err != nil {
		return Result{}, err
	}
	defer cancel()

	cleanupEnv, err := s.materializeBuildEnvironment(manifestPath, m, environment, contextDir)
	if err != nil {
		return Result{}, err
	}
	defer cleanupEnv()

	secrets, err := s.buildSecretBindings(buildCtx, manifestPath, m, environment)
	if err != nil {
		return Result{}, err
	}

	if err := s.buildImage(buildCtx, registry, dockerfile, contextDir, m.Build.Resources, m.Build.NoCache, secrets); err != nil {
		return Result{}, err
	}

	m.Source.Context, m.Source.Dockerfile = contextDir, dockerfile

	return s.publish(ctx, manifestPath, environment, m, image, registry, tag, git)
}

func (s *Service) releaseExternalImage(ctx context.Context, environment, manifestPath string, m manifest.Manifest, options Options) (Result, error) {
	source := m.Image.SourceReference
	if source == "" {
		source = fmt.Sprintf("%s:%s", m.Image.Repository, m.Image.Tag)
	}

	tag, err := sourceTagForReference(source, m, environment)
	if err != nil {
		return Result{}, err
	}

	if err := s.ensureTagUnused(ctx, m.Name, environment, tag, options); err != nil {
		return Result{}, err
	}

	image := fmt.Sprintf("%s:%s", m.Image.Repository, tag)
	registry := registryImage(s.config, image)
	if err := s.buildPulledImage(ctx, registry, source); err != nil {
		return Result{}, err
	}

	return s.publish(ctx, manifestPath, environment, m, image, registry, tag, nil)
}

func (s *Service) publish(ctx context.Context, manifestPath, environment string, m manifest.Manifest, image, registry, tag string, git *GitMetadata) (Result, error) {
	if err := s.pushImage(ctx, registry); err != nil {
		return Result{}, err
	}

	digest, err := s.resolvePushedDigest(ctx, registry)
	if err != nil {
		return Result{}, err
	}

	metadataPath, err := saveMetadataHistory(s.config, m.Name, Metadata{App: m.Name, Build: m.Image.ShouldBuild(), CreateAt: time.Now().UTC(), Environment: environment, Image: image, RegistryImage: registry, Digest: digest, Git: git, SourceTag: sourceTag(m), Tag: tag})
	if err != nil {
		return Result{}, err
	}

	return Result{Environment: environment, MetadataPath: metadataPath, ManifestPath: manifestPath, Image: image, RegistryImage: digest, Built: true, Tag: tag, Pushed: true, Manifest: m}, nil
}

func (s *Service) resolveBuildSource(ctx context.Context, environment, manifestPath string, m manifest.Manifest) (string, string, *GitMetadata, func(), error) {
	base, cleanup := filepath.Dir(manifestPath), func() {}

	var git *GitMetadata
	if m.Build.Source.Git == nil {
		return resolveSourcePath(base, m.Source.Context), resolveSourcePath(base, m.Source.Dockerfile), nil, cleanup, nil
	}

	checkout, metadata, releaseCleanup, err := s.gitBuildContext(ctx, environment, m.Build)
	if err != nil {
		return "", "", nil, cleanup, err
	}

	base, cleanup, git = checkout, releaseCleanup, &metadata
	contextDir, err := resolveGitSourcePath(base, m.Source.Context)
	if err != nil {
		cleanup()
		return "", "", nil, func() {}, err
	}

	dockerfile, err := resolveGitSourcePath(base, m.Source.Dockerfile)
	if err != nil {
		cleanup()
		return "", "", nil, func() {}, err
	}

	return contextDir, dockerfile, git, cleanup, nil
}

func buildContext(ctx context.Context, timeout string) (context.Context, context.CancelFunc, error) {
	if timeout == "" {
		return ctx, func() {}, nil
	}

	limit, err := time.ParseDuration(timeout)
	if err != nil {
		return nil, nil, fmt.Errorf("parse build timeout: %w", err)
	}

	buildCtx, cancel := context.WithTimeout(ctx, limit)

	return buildCtx, cancel, nil
}

func (s *Service) materializeBuildEnvironment(manifestPath string, m manifest.Manifest, environment, contextDir string) (func(), error) {
	values, err := buildEnvironmentValues(manifestPath, m, environment)
	if err != nil {
		return nil, err
	}

	cleanup, err := materializeBuildEnvironment(contextDir, m.Env.Build, values)
	if err != nil {
		return nil, err
	}

	return func() {
		if err := cleanup(); err != nil {
			s.logger.Warn("restore build environment", "error", err)
		}
	}, nil
}
