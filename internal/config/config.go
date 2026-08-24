package config

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/adrg/xdg"
	"github.com/joho/godotenv"
	"os"
	"path/filepath"
	"strings"
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
	ACMEEmail      string
	ConfigPath     string
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
		ACMEEmail:      os.Getenv("NOOPS_ACME_EMAIL"),
		ConfigPath:     filepath.Join(configDir, ".env.noops"),
	}, nil
}

func (c *Config) RequireACMEEmail(in *bufio.Reader, out *os.File) error {
	if c.ACMEEmail != "" {
		return nil
	}
	if _, err := fmt.Fprint(out, "ACME email (used for Let's Encrypt certificate expiry notices): "); err != nil {
		return err
	}
	email, err := in.ReadString('\n')
	if err != nil && len(email) == 0 {
		return fmt.Errorf("read ACME email: %w", err)
	}
	email = strings.TrimSpace(email)
	if !strings.Contains(email, "@") || strings.ContainsAny(email, "\r\n") {
		return fmt.Errorf("a valid ACME email is required")
	}
	if err := os.MkdirAll(filepath.Dir(c.ConfigPath), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	f, err := os.OpenFile(c.ConfigPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open config file: %w", err)
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "\nNOOPS_ACME_EMAIL=%s\n", email); err != nil {
		return fmt.Errorf("store ACME email: %w", err)
	}
	c.ACMEEmail = email
	return nil
}

func envOrDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
