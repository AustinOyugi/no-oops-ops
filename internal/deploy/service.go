package deploy

import (
	"context"
	"fmt"
	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"log/slog"
	"path/filepath"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

type Service struct {
	logger *slog.Logger
	config config.Config
}

func NewService(logger *slog.Logger, cfg config.Config) *Service {
	return &Service{
		logger: logger,
		config: cfg,
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

	stackPath, err := writeStack(s.config, m)
	if err != nil {
		return Result{}, err
	}

	return Result{
		ManifestPath: absPath,
		StackPath:    stackPath,
		Manifest:     m,
	}, nil
}
