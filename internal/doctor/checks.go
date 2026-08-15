package doctor

import "fmt"

func (s *Service) checks(profile Profile) ([]checkDefinition, error) {
	deployReadinessChecks := []checkDefinition{
		{
			name: "docker",
			run:  s.checkDocker,
		},
		{
			name:        "swarm",
			requires:    []string{"docker"},
			skipMessage: "requires Docker",
			remediation: "Start Docker first",
			run:         s.checkSwarm,
		},
		{
			name:        "swarm_manager",
			requires:    []string{"swarm"},
			skipMessage: "requires an active Docker Swarm",
			remediation: "Run noops install on a Swarm manager",
			run:         s.checkSwarmManager,
		},
		{
			name:        "shared_network",
			requires:    []string{"swarm_manager"},
			skipMessage: "requires a Docker Swarm manager",
			remediation: "Run noops install on a Swarm manager",
			run:         s.checkSharedNetwork,
		},
		{
			name:        "registry_service",
			requires:    []string{"swarm_manager"},
			skipMessage: "requires a Docker Swarm manager",
			remediation: "Run noops install on a Swarm manager",
			run:         s.checkRegistryService,
		},
		{
			name:        "registry_reachable",
			requires:    []string{"registry_service"},
			skipMessage: "requires the registry service",
			remediation: "Run noops install to deploy the registry",
			run:         s.checkRegistryReachability,
		},
	}

	if profile == ProfileDeployReadiness {
		return deployReadinessChecks, nil
	}
	if profile != ProfileFull {
		return nil, fmt.Errorf("unknown doctor profile %q", profile)
	}

	return append(deployReadinessChecks, []checkDefinition{
		{
			name: "install_metadata",
			run:  s.checkInstallMetadata,
		},
		{
			name:        "registry_config",
			requires:    []string{"install_metadata"},
			skipMessage: "installation artifacts are absent",
			remediation: "Run noops install",
			run:         s.checkRegistryConfig,
		},
		{
			name:        "registry_stack",
			requires:    []string{"install_metadata"},
			skipMessage: "installation artifacts are absent",
			remediation: "Run noops install",
			run:         s.checkRegistryStack,
		},
	}...), nil
}
