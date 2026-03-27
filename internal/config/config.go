package config

import (
	"github.com/joho/godotenv"
	"os"
)

type Config struct {
	AppName        string
	StateDir       string
	InstallVersion string
}

const defaultAppName = "noops"
const defaultInstallVersion = "dev"
const defaultStateDir = "/Users/odu/Documents/alien/code-innate/personal/no-oops-ops/.noops"

func Load() (Config, error) {
	_ = godotenv.Load(".env.noops")

	return Config{
		AppName:        defaultAppName,
		StateDir:       envOrDefault("NOOPS_STATE_DIR", defaultStateDir),
		InstallVersion: envOrDefault("NOOPS_INSTALL_VERSION", defaultInstallVersion),
	}, nil
}

func envOrDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
