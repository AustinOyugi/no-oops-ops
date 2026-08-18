package config

import (
	"github.com/adrg/xdg"
	"github.com/joho/godotenv"
	"os"
	"path/filepath"
)

type Config struct {
	AppName        string
	StateDir       string
	DataDir        string
	InstallVersion string

	NetworkName string

	RegistryName string
	RegistryPort string
}

const defaultAppName = "noops"
const defaultInstallVersion = "dev"

const defaultNetworkName = "noops-net"

const defaultRegistryName = "noops-registry"
const defaultRegistryPort = "5000"

func Load() (Config, error) {

	configDir := filepath.Join(xdg.ConfigHome, defaultAppName)
	stateDir := filepath.Join(xdg.StateHome, defaultAppName)
	dataDir := filepath.Join(xdg.DataHome, defaultAppName)

	_ = godotenv.Load(filepath.Join(configDir, ".env.noops"))

	return Config{
		AppName:        defaultAppName,
		StateDir:       envOrDefault("NOOPS_STATE_DIR", stateDir),
		DataDir:        envOrDefault("NOOPS_DATA_DIR", dataDir),
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
