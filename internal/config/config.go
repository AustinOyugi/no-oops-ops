package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/catalog"
	"github.com/AustinOyugi/no-oops-ops/internal/workspace"
	"gopkg.in/yaml.v3"
)

type Config struct {
	AppName          string
	Workspace        string
	StateDir         string
	DataDir          string
	InstallVersion   string
	RuntimeDir       string
	Environment      string
	StateDirExplicit bool

	NetworkName               string
	EnvironmentNetworkDefault string
	EnvironmentNetworks       map[string]string

	RegistryName string
	RegistryPort string

	NginxName       string
	NginxHTTPPort   string
	NginxHTTPSPort  string
	NginxCloudflare bool
	ACMEEmail       string
	ConfigPath      string
}

const defaultAppName = "noops"

const defaultNetworkName = "noops-platform"

const defaultRegistryName = "noops-registry"
const defaultRegistryPort = "5000"

const defaultNginxName = "noops-nginx"
const defaultNginxHTTPPort = "80"
const defaultNginxHTTPSPort = "443"

var Version = "dev"

var environmentNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

func Load(root string) (Config, error) {
	return LoadWithOptions(root, Options{})
}

type Options struct {
	Environment string
	StateDir    string
}

// StateDirectory resolves installation storage before initialization, too.
func StateDirectory(root string, options Options) (string, error) {
	if options.Environment != "" && !environmentNamePattern.MatchString(options.Environment) {
		return "", fmt.Errorf("invalid environment %q", options.Environment)
	}
	directory := options.StateDir
	if directory == "" {
		apps, err := catalog.Load(root)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		directory = apps.Settings.State.Directory
		if selected := apps.Settings.State.Environments[options.Environment]; options.Environment != "" && selected != "" {
			directory = selected
		}
	}
	if strings.Contains(directory, "{environment}") && options.Environment == "" {
		return "", fmt.Errorf("state directory %q requires an environment; use --environment", directory)
	}
	return strings.ReplaceAll(directory, "{environment}", options.Environment), nil
}

func OpenWorkspace(root string, options Options) (workspace.Paths, error) {
	directory, err := StateDirectory(root, options)
	if err != nil {
		return workspace.Paths{}, err
	}
	return workspace.OpenAt(root, directory)
}

func LoadWithOptions(root string, options Options) (Config, error) {
	paths, err := OpenWorkspace(root, options)
	if err != nil {
		return Config{}, err
	}
	configPath := filepath.Join(paths.Store, workspace.ConfigName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, fmt.Errorf("read workspace config %q: %w", configPath, err)
	}
	var file workspaceConfig
	if err := yaml.Unmarshal(data, &file); err != nil {
		return Config{}, fmt.Errorf("decode workspace config %q: %w", configPath, err)
	}
	apps, err := catalog.Load(paths.Root)
	if err != nil {
		return Config{}, err
	}
	if apps.Version == "" {
		return Config{}, fmt.Errorf("app catalog %q is missing version; expected %q", paths.Root+"/apps.yml", Version)
	}
	if apps.Version != Version {
		return Config{}, fmt.Errorf("app catalog %q uses version %q, but noops is version %q", paths.Root+"/apps.yml", apps.Version, Version)
	}
	platform := apps.Settings.Platform
	networkName := defaultString(platform.Network.Name, defaultNetworkName)
	registryName := defaultString(platform.Registry.Name, defaultRegistryName)
	registryPort := defaultInt(platform.Registry.Port, defaultRegistryPort)
	ingressName := defaultString(platform.Ingress.Name, defaultNginxName)
	httpPort := defaultInt(platform.Ingress.HTTPPort, defaultNginxHTTPPort)
	httpsPort := defaultInt(platform.Ingress.HTTPSPort, defaultNginxHTTPSPort)
	environmentNetworkDefault := defaultString(platform.Networks.Default, "noops-{environment}")
	return Config{
		AppName:                   defaultAppName,
		Workspace:                 paths.Root,
		StateDir:                  paths.StateDir,
		DataDir:                   paths.DataDir,
		InstallVersion:            Version,
		RuntimeDir:                paths.Store,
		Environment:               options.Environment,
		StateDirExplicit:          options.StateDir != "",
		NetworkName:               networkName,
		EnvironmentNetworkDefault: environmentNetworkDefault,
		EnvironmentNetworks:       platform.Networks.Environments,
		RegistryName:              registryName,
		RegistryPort:              registryPort,
		NginxName:                 ingressName,
		NginxHTTPPort:             httpPort,
		NginxHTTPSPort:            httpsPort,
		NginxCloudflare:           platform.Ingress.Cloudflare,
		ACMEEmail:                 file.ACMEEmail,
		ConfigPath:                configPath,
	}, nil
}

func (c Config) EnvironmentNetwork(environment string) string {
	if name := c.EnvironmentNetworks[environment]; name != "" {
		return name
	}
	return strings.ReplaceAll(c.EnvironmentNetworkDefault, "{environment}", environment)
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func defaultInt(value int, fallback string) string {
	if value == 0 {
		return fallback
	}
	return strconv.Itoa(value)
}

type workspaceConfig struct {
	ACMEEmail string `yaml:"acme_email"`
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
	content, err := workspace.RenderConfig(email)
	if err != nil {
		return fmt.Errorf("render workspace config: %w", err)
	}
	f, err := os.OpenFile(c.ConfigPath, os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open config file: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(content); err != nil {
		return fmt.Errorf("store ACME email: %w", err)
	}
	c.ACMEEmail = email
	return nil
}
