package release

import (
	"context"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"github.com/AustinOyugi/no-oops-ops/internal/storage"
)

func (s *Service) buildImage(ctx context.Context, image string, dockerfile string, contextDir string, resources manifest.BuildResources, noCache bool, secrets []BuildSecretBinding) error {
	return storage.Build(ctx, s.runner, s.config.Storage, s.logger, func(buildCtx context.Context) error {
		return s.buildImageIsolated(buildCtx, image, dockerfile, contextDir, resources, noCache, secrets)
	})
}
