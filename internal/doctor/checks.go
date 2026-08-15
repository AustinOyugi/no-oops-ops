package doctor

func (s *Service) checks() []checkDefinition {
	return []checkDefinition{
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
			name:        "shared_network",
			requires:    []string{"swarm"},
			skipMessage: "requires an active Docker Swarm",
			remediation: "Run noops install to initialize Docker Swarm",
			run:         s.checkSharedNetwork,
		},
		{
			name:        "registry_service",
			requires:    []string{"swarm"},
			skipMessage: "requires an active Docker Swarm",
			remediation: "Run noops install to initialize Docker Swarm",
			run:         s.checkRegistryService,
		},
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
	}
}
