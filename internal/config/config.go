package config

import (
	"errors"
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

	NginxName      string
	NginxHTTPPort  string
	NginxHTTPSPort string
}

const defaultAppName = "noops"

const defaultNetworkName = "noops-net"

const defaultRegistryName = "noops-registry"
const defaultRegistryPort = "5000"

const defaultNginxName = "noops-nginx"
const defaultNginxHTTPPort = "80"
const defaultNginxHTTPSPort = "443"

var Version = "dev"

func Load() (Config, error) {

	configDir := filepath.Join(xdg.ConfigHome, defaultAppName)
	stateDir := filepath.Join(xdg.StateHome, defaultAppName)
	dataDir := filepath.Join(xdg.DataHome, defaultAppName)

	err := godotenv.Load(filepath.Join(configDir, ".env.noops"))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return Config{}, err
		}
	}

	return Config{
		AppName:        defaultAppName,
		StateDir:       envOrDefault("NOOPS_STATE_DIR", stateDir),
		DataDir:        envOrDefault("NOOPS_DATA_DIR", dataDir),
		InstallVersion: Version,
		NetworkName:    envOrDefault("NOOPS_NETWORK_NAME", defaultNetworkName),
		RegistryName:   envOrDefault("NOOPS_REGISTRY_NAME", defaultRegistryName),
		RegistryPort:   envOrDefault("NOOPS_REGISTRY_PORT", defaultRegistryPort),
		NginxName:      envOrDefault("NOOPS_NGINX_NAME", defaultNginxName),
		NginxHTTPPort:  envOrDefault("NOOPS_NGINX_HTTP_PORT", defaultNginxHTTPPort),
		NginxHTTPSPort: envOrDefault("NOOPS_NGINX_HTTPS_PORT", defaultNginxHTTPSPort),
	}, nil
}

func envOrDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
