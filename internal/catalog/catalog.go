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
	Name       string `yaml:"name"`
	HTTPPort   int    `yaml:"http_port"`
	HTTPSPort  int    `yaml:"https_port"`
	Cloudflare bool   `yaml:"cloudflare"`
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
