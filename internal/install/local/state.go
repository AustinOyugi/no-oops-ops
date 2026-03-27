package local

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/install"
)

const stateDirMode = 0o700
const installMetadataFileMode = 0o600

func (h *Host) PrepareStateDir(ctx context.Context) error {
	h.logger.InfoContext(ctx, "preparing local state directory", "path", h.stateDir)

	err := os.MkdirAll(h.stateDir, stateDirMode)
	if err != nil {
		return install.PrerequisiteError{
			Check: install.StepPrepareStateDir,
			Err:   fmt.Errorf("create state dir %q: %w", h.stateDir, err),
		}
	}

	return nil
}

func (h *Host) stateDataDir() string {
	return filepath.Join(h.stateDir, "data")
}

func (h *Host) InitializeLocalState(ctx context.Context) error {
	path := h.stateDataDir()

	h.logger.InfoContext(ctx, "initializing local state", "path", path)

	if err := os.MkdirAll(path, stateDirMode); err != nil {
		return install.PrerequisiteError{
			Check: install.StepInitializeLocalState,
			Err:   fmt.Errorf("initialize local state %q: %w", path, err),
		}
	}

	return nil
}

func (h *Host) installMetadataPath() string {
	return filepath.Join(h.stateDir, "install.json")
}

func (h *Host) WriteInstallMetadata(ctx context.Context) error {
	path := h.installMetadataPath()

	h.logger.InfoContext(ctx, "writing install metadata", "path", path)

	data, err := json.MarshalIndent(metadata{
		Version:     h.installVersion,
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
		Swarm: swarmMetadata{
			Initialized:    h.swarmInitialized,
			LocalNodeState: h.swarmNodeState,
			ManagerAddress: h.swarmManagerAddr,
		},
		Network: networkMetadata{
			Name: h.networkName,
		},
	}, "", "  ")
	if err != nil {
		return install.PrerequisiteError{
			Check: install.StepWriteInstallMetadata,
			Err:   fmt.Errorf("marshal install metadata: %w", err),
		}
	}

	data = append(data, '\n')

	if err := os.WriteFile(path, data, installMetadataFileMode); err != nil {
		return install.PrerequisiteError{
			Check: install.StepWriteInstallMetadata,
			Err:   fmt.Errorf("write install metadata %q: %w", path, err),
		}
	}

	return nil
}

func (h *Host) readInstallMetadata(ctx context.Context) (metadata, error) {
	_ = ctx

	path := h.installMetadataPath()

	h.logger.InfoContext(ctx, "reading install metadata", "path", path)

	return readMetadata(path)
}
