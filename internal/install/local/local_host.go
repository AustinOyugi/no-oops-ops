package local

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/AustinOyugi/no-oops-ops/internal/install"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Host struct {
	runner           *command.Runner
	logger           *slog.Logger
	stateDir         string
	installVersion   string
	swarmInitialized bool
	swarmNodeState   string
	swarmManagerAddr string
	networkName      string
	registryName     string
	registryPort     string
}

func NewHost(
	logger *slog.Logger,
	stateDir string,
	installVersion string,
	networkName string,
	registryName string,
	registryPort string) *Host {
	return &Host{
		runner:         command.NewRunner(logger),
		logger:         logger,
		stateDir:       stateDir,
		installVersion: installVersion,
		networkName:    networkName,
		registryName:   registryName,
		registryPort:   registryPort,
	}
}

func (h *Host) inspectSwarmManagerAddress(ctx context.Context) string {
	result, err := h.runner.Run(
		ctx,
		"docker",
		[]string{"info", "--format", "{{.Swarm.NodeAddr}}"},
		command.RunOptions{},
	)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(result.Output))
}

func (h *Host) VerifyDocker(ctx context.Context) error {
	h.logger.InfoContext(ctx, "checking docker installation")

	result, err := h.runner.Run(ctx, "docker", []string{"version"}, command.RunOptions{})

	if err != nil {
		return install.PrerequisiteError{
			Check: install.StepVerifyDocker,
			Err:   fmt.Errorf("verify docker: %w: %s", err, strings.TrimSpace(string(result.Output))),
		}
	}

	return nil
}

func (h *Host) EnsureSwarmInitialized(ctx context.Context) error {
	h.logger.InfoContext(ctx, "ensuring swarm is initialized")

	result, err := h.runner.Run(
		ctx,
		"docker",
		[]string{"info", "--format", "{{.Swarm.LocalNodeState}}"},
		command.RunOptions{},
	)
	if err != nil {
		return install.PrerequisiteError{
			Check: install.StepEnsureSwarmInitialized,
			Err:   fmt.Errorf("inspect swarm state: %w: %s", err, strings.TrimSpace(string(result.Output))),
		}
	}

	state := strings.TrimSpace(string(result.Output))
	if state == "active" {
		h.swarmNodeState = state
		h.swarmInitialized = true
		h.swarmManagerAddr = h.inspectSwarmManagerAddress(ctx)
		return nil
	}

	initResult, err := h.runner.Run(
		ctx,
		"docker",
		[]string{"swarm", "init"},
		command.RunOptions{},
	)
	if err != nil {
		return install.PrerequisiteError{
			Check: install.StepEnsureSwarmInitialized,
			Err:   fmt.Errorf("initialize swarm: %w: %s", err, strings.TrimSpace(string(initResult.Output))),
		}
	}
	h.swarmManagerAddr = h.inspectSwarmManagerAddress(ctx)
	h.swarmInitialized = true
	h.swarmNodeState = "active"
	return nil
}

func (h *Host) EnsureSharedNetwork(ctx context.Context) error {
	h.logger.InfoContext(ctx, "ensuring shared network", "network", h.networkName)

	_, err := h.runner.Run(
		ctx,
		"docker",
		[]string{"network", "inspect", h.networkName},
		command.RunOptions{},
	)
	if err == nil {
		return nil
	}

	result, err := h.runner.Run(
		ctx,
		"docker",
		[]string{"network", "create", "--driver", "overlay", h.networkName},
		command.RunOptions{},
	)
	if err != nil {
		return install.PrerequisiteError{
			Check: install.StepEnsureSharedNetwork,
			Err:   fmt.Errorf("create shared network %q: %w: %s", h.networkName, err, strings.TrimSpace(string(result.Output))),
		}
	}

	return nil
}

func (h *Host) EnsureRegistry(ctx context.Context) error {
	h.logger.InfoContext(
		ctx,
		"ensuring registry",
		"name", h.registryName,
		"port", h.registryPort,
	)

	_, err := h.runner.Run(
		ctx,
		"docker",
		[]string{"service", "inspect", h.registryName},
		command.RunOptions{},
	)
	if err == nil {
		return nil
	}

	result, err := h.runner.Run(
		ctx,
		"docker",
		[]string{
			"service", "create",
			"--name", h.registryName,
			"--network", h.networkName,
			"--publish", fmt.Sprintf("%s:5000", h.registryPort),
			"registry:2",
		},
		command.RunOptions{
			StreamOutput: true,
			Stdout:       os.Stdout,
			Stderr:       os.Stderr,
		},
	)

	if err != nil {
		return install.PrerequisiteError{
			Check: install.StepEnsureRegistry,
			Err:   fmt.Errorf("create registry service %q: %w: %s", h.registryName, err, strings.TrimSpace(string(result.Output))),
		}
	}

	return nil
}

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
