// Package catalog resolves application aliases from a workspace apps.yml file.
package catalog

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type File struct {
	Version  string         `yaml:"version"`
	Settings Settings       `yaml:"settings"`
	Apps     map[string]App `yaml:"apps"`
}

type App struct {
	Manifest string `yaml:"manifest"`
}

type Settings struct {
	Platform Platform `yaml:"platform"`
	Upgrade  Upgrade  `yaml:"upgrade"`
	State    State    `yaml:"state"`
}

// State selects the runtime store without duplicating application manifests.
type State struct {
	Directory    string            `yaml:"directory"`
	Environments map[string]string `yaml:"environments"`
}

type Upgrade struct {
	Repository string `yaml:"repository"`
}

type Platform struct {
	Network  Network         `yaml:"network"`
	Registry Registry        `yaml:"registry"`
	Ingress  Ingress         `yaml:"ingress"`
	Networks EnvironmentNets `yaml:"networks"`
}

type Network struct {
	Name string `yaml:"name"`
}

type Registry struct {
	Name string `yaml:"name"`
	Port int    `yaml:"port"`
}

type Ingress struct {
	Networks       []string                   `yaml:"networks"`
	Name           string                     `yaml:"name"`
	HTTPPort       int                        `yaml:"http_port"`
	HTTPSPort      int                        `yaml:"https_port"`
	Cloudflare     bool                       `yaml:"cloudflare"`
	Environments   map[string]IngressOverride `yaml:"environments"`
	legacyNetworks map[string][]string
}

// IngressOverride uses pointers where zero values must override shared settings.
type IngressOverride struct {
	Name       *string   `yaml:"name"`
	HTTPPort   *int      `yaml:"http_port"`
	HTTPSPort  *int      `yaml:"https_port"`
	Cloudflare *bool     `yaml:"cloudflare"`
	Networks   *[]string `yaml:"networks"`
}

// UnmarshalYAML also accepts the original environment-to-networks mapping.
func (i *Ingress) UnmarshalYAML(node *yaml.Node) error {
	var decoded struct {
		Name         string                     `yaml:"name"`
		HTTPPort     int                        `yaml:"http_port"`
		HTTPSPort    int                        `yaml:"https_port"`
		Cloudflare   bool                       `yaml:"cloudflare"`
		Environments map[string]IngressOverride `yaml:"environments"`
		Networks     yaml.Node                  `yaml:"networks"`
	}
	if err := node.Decode(&decoded); err != nil {
		return err
	}
	*i = Ingress{Name: decoded.Name, HTTPPort: decoded.HTTPPort, HTTPSPort: decoded.HTTPSPort, Cloudflare: decoded.Cloudflare, Environments: decoded.Environments}
	if decoded.Networks.Kind == yaml.MappingNode {
		return decoded.Networks.Decode(&i.legacyNetworks)
	}
	if decoded.Networks.Kind != 0 {
		return decoded.Networks.Decode(&i.Networks)
	}
	return nil
}

// ForEnvironment resolves shared defaults followed by environment overrides.
func (i Ingress) ForEnvironment(environment string) Ingress {
	if environment == "" {
		return i
	}
	if networks, ok := i.legacyNetworks[environment]; ok {
		i.Networks = networks
	}
	override := i.Environments[environment]
	if override.Name != nil {
		i.Name = *override.Name
	}
	if override.HTTPPort != nil {
		i.HTTPPort = *override.HTTPPort
	}
	if override.HTTPSPort != nil {
		i.HTTPSPort = *override.HTTPSPort
	}
	if override.Cloudflare != nil {
		i.Cloudflare = *override.Cloudflare
	}
	if override.Networks != nil {
		i.Networks = *override.Networks
	}
	return i
}

type EnvironmentNets struct {
	Default      string            `yaml:"default"`
	Environments map[string]string `yaml:"environments"`
}

func Load(workspace string) (File, error) {
	path := filepath.Join(workspace, "apps.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, fmt.Errorf("read app catalog %q: %w", path, err)
	}
	var file File
	if err := yaml.Unmarshal(data, &file); err != nil {
		return File{}, fmt.Errorf("decode app catalog %q: %w", path, err)
	}
	return file, nil
}

func Resolve(workspace, name string) (string, error) {
	path := filepath.Join(workspace, "apps.yml")
	file, err := Load(workspace)
	if err != nil {
		return "", err
	}
	app, ok := file.Apps[name]
	if !ok {
		return "", fmt.Errorf("app %q is not declared in %q", name, path)
	}
	if app.Manifest == "" {
		return "", fmt.Errorf("app %q has no manifest in %q", name, path)
	}
	manifestPath := filepath.Clean(filepath.Join(workspace, app.Manifest))
	if _, err := os.Stat(manifestPath); err != nil {
		return "", fmt.Errorf("app %q manifest %q: %w", name, manifestPath, err)
	}
	return manifestPath, nil
}
