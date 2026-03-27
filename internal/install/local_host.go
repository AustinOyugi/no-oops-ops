package install

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type LocalHost struct {
	logger         *slog.Logger
	stateDir       string
	installVersion string
}

func NewLocalHost(logger *slog.Logger, stateDir string, installVersion string) *LocalHost {
	return &LocalHost{
		logger:         logger,
		stateDir:       stateDir,
		installVersion: installVersion,
	}
}

func (h *LocalHost) VerifyDocker(ctx context.Context) error {
	h.logger.InfoContext(ctx, "checking docker installation")

	cmd := exec.CommandContext(ctx, "docker", "version")
	output, err := cmd.CombinedOutput()

	if err != nil {
		return PrerequisiteError{
			Check: StepVerifyDocker,
			Err:   fmt.Errorf("verify docker: %w: %s", err, strings.TrimSpace(string(output))),
		}
	}

	return nil
}

const stateDirMode = 0o700
const installMetadataFileMode = 0o600

func (h *LocalHost) PrepareStateDir(ctx context.Context) error {
	h.logger.InfoContext(ctx, "preparing local state directory", "path", h.stateDir)
	err := os.MkdirAll(h.stateDir, stateDirMode)
	if err != nil {
		return PrerequisiteError{
			Check: StepPrepareStateDir,
			Err:   fmt.Errorf("create state dir %q: %w", h.stateDir, err),
		}
	}
	return nil
}

func (h *LocalHost) stateDataDir() string {
	return filepath.Join(h.stateDir, "data")
}

func (h *LocalHost) InitializeLocalState(ctx context.Context) error {
	path := h.stateDataDir()

	h.logger.InfoContext(ctx, "initializing local state", "path", path)

	if err := os.MkdirAll(path, stateDirMode); err != nil {
		return PrerequisiteError{
			Check: StepInitializeLocalState,
			Err:   fmt.Errorf("initialize local state %q: %w", path, err),
		}
	}

	return nil
}

func (h *LocalHost) installMetadataPath() string {
	return filepath.Join(h.stateDir, "install.json")
}

func (h *LocalHost) WriteInstallMetadata(ctx context.Context) error {
	path := h.installMetadataPath()

	h.logger.InfoContext(ctx, "writing install metadata", "path", path)

	data, err := json.MarshalIndent(metadata{
		Version: h.installVersion,
	}, "", "  ")

	if err != nil {
		return PrerequisiteError{
			Check: StepWriteInstallMetadata,
			Err:   fmt.Errorf("marshal install metadata: %w", err),
		}
	}

	data = append(data, '\n')

	if err := os.WriteFile(path, data, installMetadataFileMode); err != nil {
		return PrerequisiteError{
			Check: StepWriteInstallMetadata,
			Err:   fmt.Errorf("write install metadata %q: %w", path, err),
		}
	}

	return nil
}
