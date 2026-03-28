package status

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
)

type Host interface {
	InspectSwarmState(ctx context.Context) (string, error)
	InspectSharedNetwork(ctx context.Context) error
	InspectRegistryService(ctx context.Context) error
}

type Service struct {
	logger *slog.Logger
	config config.Config
	host   Host
}

func NewService(logger *slog.Logger, cfg config.Config, host Host) *Service {
	return &Service{
		logger: logger,
		config: cfg,
		host:   host,
	}
}

func (s *Service) Run(ctx context.Context) (Result, error) {
	s.logger.InfoContext(ctx, "starting status")

	result := Result{}

	metadata, err := s.readMetadata()
	if err != nil {
		return Result{}, err
	}
	result.Metadata = metadata

	state, err := s.host.InspectSwarmState(ctx)
	if err != nil {
		result.AddComponent("swarm", ComponentStatusMissing, err.Error())
	} else {
		result.AddComponent("swarm", ComponentStatusReady, fmt.Sprintf("state=%s", state))
	}

	if err := s.host.InspectSharedNetwork(ctx); err != nil {
		result.AddComponent("shared_network", ComponentStatusMissing, err.Error())
	} else {
		result.AddComponent("shared_network", ComponentStatusReady, s.config.NetworkName)
	}

	if err := s.host.InspectRegistryService(ctx); err != nil {
		result.AddComponent("registry_service", ComponentStatusMissing, err.Error())
	} else {
		result.AddComponent("registry_service", ComponentStatusReady, result.Metadata.Registry.ServiceName)
	}

	result.AddComponent("registry_config", componentStatusFromFile(result.Metadata.Registry.ConfigPath), result.Metadata.Registry.ConfigPath)
	result.AddComponent("registry_stack", componentStatusFromFile(result.Metadata.Registry.StackPath), result.Metadata.Registry.StackPath)

	return result, nil
}

func (s *Service) readMetadata() (Metadata, error) {
	path := filepath.Join(s.config.StateDir, "install.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return Metadata{}, fmt.Errorf("read install metadata %q: %w", path, err)
	}

	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return Metadata{}, fmt.Errorf("decode install metadata %q: %w", path, err)
	}

	return metadata, nil
}

func componentStatusFromFile(path string) ComponentStatus {
	_, err := os.Stat(path)
	if err != nil {
		return ComponentStatusMissing
	}

	return ComponentStatusReady
}
