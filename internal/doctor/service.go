package doctor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
)

type Host interface {
	VerifyDocker(ctx context.Context) error
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
	s.logger.InfoContext(ctx, "starting doctor")

	result := Result{}

	if err := s.host.VerifyDocker(ctx); err != nil {
		result.Add("docker", StatusFail, err.Error())
	} else {
		result.Add("docker", StatusOK, "docker is available")
	}

	state, err := s.host.InspectSwarmState(ctx)
	if err != nil {
		result.Add("swarm", StatusFail, err.Error())
	} else if state != "active" {
		result.Add("swarm", StatusFail, fmt.Sprintf("unexpected swarm state: %s", state))
	} else {
		result.Add("swarm", StatusOK, "swarm is active")
	}

	if err := s.host.InspectSharedNetwork(ctx); err != nil {
		result.Add("shared_network", StatusFail, err.Error())
	} else {
		result.Add("shared_network", StatusOK, fmt.Sprintf("network %s exists", s.config.NetworkName))
	}

	if err := s.host.InspectRegistryService(ctx); err != nil {
		result.Add("registry_service", StatusFail, err.Error())
	} else {
		result.Add("registry_service", StatusOK, fmt.Sprintf("service %s exists", s.config.RegistryName+"_registry"))
	}

	s.checkFile(&result, "install_metadata", filepath.Join(s.config.StateDir, "install.json"))
	s.checkFile(&result, "registry_config", filepath.Join(s.config.StateDir, "registry", "config.yml"))
	s.checkFile(&result, "registry_stack", filepath.Join(s.config.StateDir, "registry", "stack.yml"))

	return result, nil
}

func (s *Service) checkFile(result *Result, name string, path string) {
	_, err := os.Stat(path)
	if err != nil {
		result.Add(name, StatusFail, err.Error())
		return
	}

	result.Add(name, StatusOK, fmt.Sprintf("%s exists", path))
}
