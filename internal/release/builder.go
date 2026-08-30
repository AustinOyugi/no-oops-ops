package release

import (
	"context"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

func (s *Service) buildImage(ctx context.Context, image string, dockerfile string, contextDir string, resources manifest.BuildResources, secrets []BuildSecretBinding) error {
	return s.buildImageIsolated(ctx, image, dockerfile, contextDir, resources, secrets)
}
