package config

import (
	"github.com/joho/godotenv"
	"os"
)

type Config struct {
	AppName        string
	StateDir       string
	InstallVersion string

	NetworkName string

	RegistryName string
	RegistryPort string
}

const defaultAppName = "noops"
const defaultInstallVersion = "dev"
const defaultStateDir = "/Users/odu/Documents/alien/code-innate/personal/no-oops-ops/.noops"

const defaultNetworkName = "noops-net"

const defaultRegistryName = "noops-registry"
const defaultRegistryPort = "5000"

func Load() (Config, error) {
	_ = godotenv.Load(".env.noops")

	return Config{
		AppName:        defaultAppName,
		StateDir:       envOrDefault("NOOPS_STATE_DIR", defaultStateDir),
		InstallVersion: envOrDefault("NOOPS_INSTALL_VERSION", defaultInstallVersion),
		NetworkName:    envOrDefault("NOOPS_NETWORK_NAME", defaultNetworkName),
		RegistryName:   envOrDefault("NOOPS_REGISTRY_NAME", defaultRegistryName),
		RegistryPort:   envOrDefault("NOOPS_REGISTRY_PORT", defaultRegistryPort),
	}, nil
}

func envOrDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
