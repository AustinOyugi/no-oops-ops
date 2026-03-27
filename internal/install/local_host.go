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
	"time"
)

type LocalHost struct {
	logger           *slog.Logger
	stateDir         string
	installVersion   string
	swarmInitialized bool
	swarmNodeState   string
	swarmManagerAddr string
	networkName      string
}

func NewLocalHost(logger *slog.Logger, stateDir string, installVersion string, networkName string) *LocalHost {
	return &LocalHost{
		logger:         logger,
		stateDir:       stateDir,
		installVersion: installVersion,
		networkName:    networkName,
	}
}

func (h *LocalHost) inspectSwarmManagerAddress(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, "docker", "info", "--format", "{{.Swarm.NodeAddr}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
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

func (h *LocalHost) EnsureSwarmInitialized(ctx context.Context) error {
	h.logger.InfoContext(ctx, "ensuring swarm is initialized")

	cmd := exec.CommandContext(ctx, "docker", "info", "--format", "{{.Swarm.LocalNodeState}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return PrerequisiteError{
			Check: StepEnsureSwarmInitialized,
			Err:   fmt.Errorf("inspect swarm state: %w: %s", err, strings.TrimSpace(string(output))),
		}
	}

	state := strings.TrimSpace(string(output))
	if state == "active" {
		h.swarmNodeState = state
		h.swarmInitialized = true
		h.swarmManagerAddr = h.inspectSwarmManagerAddress(ctx)
		return nil
	}

	initCmd := exec.CommandContext(ctx, "docker", "swarm", "init")
	initOutput, err := initCmd.CombinedOutput()
	if err != nil {
		return PrerequisiteError{
			Check: StepEnsureSwarmInitialized,
			Err:   fmt.Errorf("initialize swarm: %w: %s", err, strings.TrimSpace(string(initOutput))),
		}
	}
	h.swarmManagerAddr = h.inspectSwarmManagerAddress(ctx)
	h.swarmInitialized = true
	h.swarmNodeState = "active"
	return nil
}

func (h *LocalHost) EnsureSharedNetwork(ctx context.Context) error {
	h.logger.InfoContext(ctx, "ensuring shared network", "network", h.networkName)

	inspectCmd := exec.CommandContext(ctx, "docker", "network", "inspect", h.networkName)
	if err := inspectCmd.Run(); err == nil {
		return nil
	}

	createCmd := exec.CommandContext(ctx, "docker", "network", "create", "--driver", "overlay", h.networkName)
	output, err := createCmd.CombinedOutput()
	if err != nil {
		return PrerequisiteError{
			Check: StepEnsureSharedNetwork,
			Err:   fmt.Errorf("create shared network %q: %w: %s", h.networkName, err, strings.TrimSpace(string(output))),
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

func (h *LocalHost) readInstallMetadata(ctx context.Context) (metadata, error) {
	_ = ctx

	path := h.installMetadataPath()

	h.logger.InfoContext(ctx, "reading install metadata", "path", path)

	return readMetadata(path)
}
