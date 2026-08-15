package doctor

import (
	"context"
	"path/filepath"
)

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
			run: func(context.Context) Check {
				return s.checkFile(
					"install_metadata",
					filepath.Join(s.config.StateDir, "install.json"),
					"Run noops install to create installation metadata")
			}},
		{
			name:        "registry_config",
			requires:    []string{"install_metadata"},
			skipMessage: "installation artifacts are absent",
			remediation: "Run noops install", run: func(context.Context) Check {
				return s.checkFile(
					"registry_config",
					filepath.Join(s.config.StateDir, "registry", "config.yml"),
					"Run noops install to recreate the registry configuration")
			},
		},
		{
			name:        "registry_stack",
			requires:    []string{"install_metadata"},
			skipMessage: "installation artifacts are absent",
			remediation: "Run noops install",
			run: func(context.Context) Check {
				return s.checkFile(
					"registry_stack",
					filepath.Join(s.config.StateDir, "registry", "stack.yml"),
					"Run noops install to recreate the registry stack")
			},
		},
	}
}
