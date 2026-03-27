package local

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AustinOyugi/no-oops-ops/internal/install"
)

func (h *Host) registryDir() string {
	return filepath.Join(h.stateDir, "registry")
}

func (h *Host) registryConfigPath() string {
	return filepath.Join(h.registryDir(), "config.yml")
}

func (h *Host) WriteRegistryConfig(ctx context.Context) error {
	dir := h.registryDir()
	path := h.registryConfigPath()

	h.logger.InfoContext(ctx, "writing registry config", "path", path)

	if err := os.MkdirAll(dir, stateDirMode); err != nil {
		return install.PrerequisiteError{
			Check: install.StepWriteRegistryConfig,
			Err:   fmt.Errorf("create registry config dir %q: %w", dir, err),
		}
	}

	config := "version: 0.1\nstorage:\n  delete:\n    enabled: true\nhttp:\n  addr: :5000\n"

	if err := os.WriteFile(path, []byte(config), installMetadataFileMode); err != nil {
		return install.PrerequisiteError{
			Check: install.StepWriteRegistryConfig,
			Err:   fmt.Errorf("write registry config %q: %w", path, err),
		}
	}

	return nil
}
