package local

import (
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"log/slog"
)

type Host struct {
	runner           *command.Runner
	logger           *slog.Logger
	stateDir         string
	dataDir          string
	installVersion   string
	swarmInitialized bool
	swarmNodeState   string
	swarmManagerAddr string
	networkName      string
	registryName     string
	registryPort     string
	registryService  string
	registryReady    bool
	nginxName        string
	nginxHTTPPort    string
	nginxHTTPSPort   string
	nginxService     string
	nginxReady       bool
}

func NewHost(
	logger *slog.Logger,
	stateDir string,
	dataDir string,
	installVersion string,
	networkName string,
	registryName string,
	registryPort string,
	nginxName string,
	nginxHTTPPort string,
	nginxHTTPSPort string) *Host {
	return &Host{
		runner:          command.NewRunner(logger),
		logger:          logger,
		stateDir:        stateDir,
		dataDir:         dataDir,
		installVersion:  installVersion,
		networkName:     networkName,
		registryName:    registryName,
		registryPort:    registryPort,
		registryService: registryName + "_registry",
		nginxName:       nginxName,
		nginxHTTPPort:   nginxHTTPPort,
		nginxHTTPSPort:  nginxHTTPSPort,
		nginxService:    nginxName + "_nginx",
	}
}
