package local

import (
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"log/slog"
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
	registryService  string
	registryReady    bool
}

func NewHost(
	logger *slog.Logger,
	stateDir string,
	installVersion string,
	networkName string,
	registryName string,
	registryPort string) *Host {
	return &Host{
		runner:          command.NewRunner(logger),
		logger:          logger,
		stateDir:        stateDir,
		installVersion:  installVersion,
		networkName:     networkName,
		registryName:    registryName,
		registryPort:    registryPort,
		registryService: registryName + "_registry",
	}
}
