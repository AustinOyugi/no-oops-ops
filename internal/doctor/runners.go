package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

func (s *Service) checkDocker(ctx context.Context) Check {
	if err := s.host.VerifyDocker(ctx); err != nil {
		return Check{
			Name:        "docker",
			Status:      StatusFail,
			Message:     "docker is unavailable",
			Remediation: "Start Docker and run noops doctor again"}
	}

	return Check{
		Name:    "docker",
		Status:  StatusOK,
		Message: "docker is available"}
}

func (s *Service) checkSwarm(ctx context.Context) Check {
	state, err := s.host.InspectSwarmState(ctx)
	if err != nil {
		return Check{
			Name:        "swarm",
			Status:      StatusFail,
			Message:     "could not determine Docker Swarm state",
			Remediation: "Run docker swarm init, then run noops install"}
	}

	if state != "active" {
		return Check{
			Name:        "swarm",
			Status:      StatusFail,
			Message:     fmt.Sprintf("Docker Swarm is %s", state),
			Remediation: "Run noops install to initialize Docker Swarm"}
	}

	return Check{
		Name:    "swarm",
		Status:  StatusOK,
		Message: "swarm is active"}
}

func (s *Service) checkSwarmManager(ctx context.Context) Check {
	if err := s.host.InspectSwarmManager(ctx); err != nil {
		return Check{
			Name:        "swarm_manager",
			Status:      StatusFail,
			Message:     "this node is not a Docker Swarm manager",
			Remediation: "Run noops install on a Swarm manager",
		}
	}

	return Check{
		Name:    "swarm_manager",
		Status:  StatusOK,
		Message: "this node is a Docker Swarm manager",
	}
}

func (s *Service) checkSharedNetwork(ctx context.Context) Check {
	if err := s.host.InspectSharedNetwork(ctx); err != nil {
		return Check{
			Name:        "shared_network",
			Status:      StatusFail,
			Message:     fmt.Sprintf("network %s is missing", s.config.NetworkName),
			Remediation: "Run noops install to create the shared network"}
	}

	return Check{
		Name:    "shared_network",
		Status:  StatusOK,
		Message: fmt.Sprintf("network %s exists", s.config.NetworkName)}
}

func (s *Service) checkRegistryService(ctx context.Context) Check {
	if err := s.host.InspectRegistryService(ctx); err != nil {
		return Check{
			Name:        "registry_service",
			Status:      StatusFail,
			Message:     fmt.Sprintf("service %s is missing", s.config.RegistryName+"_registry"),
			Remediation: "Run noops install to deploy the registry"}
	}

	return Check{
		Name:    "registry_service",
		Status:  StatusOK,
		Message: fmt.Sprintf("service %s exists", s.config.RegistryName+"_registry")}
}

func (s *Service) checkRegistryReachability(ctx context.Context) Check {
	if err := s.host.InspectRegistryReachability(ctx); err != nil {
		return Check{
			Name:        "registry_reachable",
			Status:      StatusFail,
			Message:     "registry API is unreachable",
			Remediation: "Run noops install to redeploy the registry",
		}
	}

	return Check{
		Name:    "registry_reachable",
		Status:  StatusOK,
		Message: "registry API is reachable",
	}
}

func (s *Service) checkInstallMetadata(context.Context) Check {
	return s.checkFile(
		"install_metadata",
		filepath.Join(s.config.StateDir, "install.json"),
		"Run noops install to create installation metadata",
	)
}

func (s *Service) checkRegistryConfig(context.Context) Check {
	return s.checkFile(
		"registry_config",
		filepath.Join(s.config.StateDir, "registry", "config.yml"),
		"Run noops install to recreate the registry configuration",
	)
}

func (s *Service) checkRegistryStack(context.Context) Check {
	return s.checkFile(
		"registry_stack",
		filepath.Join(s.config.StateDir, "registry", "stack.yml"),
		"Run noops install to recreate the registry stack",
	)
}

func (s *Service) checkFile(name string, path string, remediation string) Check {
	_, err := os.Stat(path)

	if err != nil {
		if os.IsNotExist(err) {
			return Check{
				Name:        name,
				Status:      StatusFail,
				Message:     fmt.Sprintf("%s is missing", path),
				Remediation: remediation}
		} else {
			return Check{
				Name:        name,
				Status:      StatusFail,
				Message:     fmt.Sprintf("cannot read %s", path),
				Remediation: remediation}
		}
	}

	return Check{
		Name:    name,
		Status:  StatusOK,
		Message: fmt.Sprintf("%s exists", path)}
}
