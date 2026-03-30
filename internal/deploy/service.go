package deploy

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

type Service struct {
	logger *slog.Logger
}

func NewService(logger *slog.Logger) *Service {
	return &Service{
		logger: logger,
	}
}

func (s *Service) Run(ctx context.Context, path string) (Result, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return Result{}, fmt.Errorf("resolve manifest path %q: %w", path, err)
	}

	s.logger.InfoContext(ctx, "starting deploy", "manifest", absPath)

	m, err := manifest.Load(absPath)
	if err != nil {
		return Result{}, err
	}

	return Result{
		ManifestPath: absPath,
		Manifest:     m,
	}, nil
}
