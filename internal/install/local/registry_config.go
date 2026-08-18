package local

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AustinOyugi/no-oops-ops/internal/install"
)

//go:embed assets/registry-config.yml
var registryConfigContents []byte

//go:embed templates/registry-stack.yml.tmpl
var registryStackTemplateContents string

func (h *Host) registryDir() string {
	return filepath.Join(h.stateDir, "registry")
}

func (h *Host) registryConfigPath() string {
	return filepath.Join(h.registryDir(), "config.yml")
}

func (h *Host) registryStackPath() string {
	return filepath.Join(h.registryDir(), "stack.yml")
}

func (h *Host) registryDataPath() string {
	return filepath.Join(h.dataDir, "data")
}

type registryStackTemplateData struct {
	RegistryPort string
	NetworkName  string
	ConfigPath   string
	DataPath     string
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

	if err := os.WriteFile(path, registryConfigContents, installMetadataFileMode); err != nil {
		return install.PrerequisiteError{
			Check: install.StepWriteRegistryConfig,
			Err:   fmt.Errorf("write registry config %q: %w", path, err),
		}
	}

	return nil
}

func (h *Host) WriteRegistryStack(ctx context.Context) error {
	path := h.registryStackPath()
	dataDir := h.registryDataPath()

	h.logger.InfoContext(ctx, "writing registry stack", "path", path)

	if err := os.MkdirAll(dataDir, stateDirMode); err != nil {
		return install.PrerequisiteError{
			Check: install.StepWriteRegistryStack,
			Err:   fmt.Errorf("create registry data dir %q: %w", dataDir, err),
		}
	}

	rendered, err := renderTemplate(
		"registry-stack.yml.tmpl",
		registryStackTemplateContents,
		registryStackTemplateData{
			RegistryPort: h.registryPort,
			NetworkName:  h.networkName,
			ConfigPath:   h.registryConfigPath(),
			DataPath:     dataDir,
		},
	)

	if err != nil {
		return install.PrerequisiteError{
			Check: install.StepWriteRegistryStack,
			Err:   fmt.Errorf("render registry stack: %w", err),
		}
	}

	rendered = append(rendered, '\n')

	if err := os.WriteFile(path, rendered, installMetadataFileMode); err != nil {
		return install.PrerequisiteError{
			Check: install.StepWriteRegistryStack,
			Err:   fmt.Errorf("write registry stack %q: %w", path, err),
		}
	}

	return nil
}
