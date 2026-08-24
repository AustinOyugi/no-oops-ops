// Package catalog resolves application aliases from a workspace apps.yml file.
package catalog

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type File struct {
	Apps map[string]App `yaml:"apps"`
}

type App struct {
	Manifest string `yaml:"manifest"`
}

func Resolve(workspace, name string) (string, error) {
	path := filepath.Join(workspace, "apps.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read app catalog %q: %w", path, err)
	}
	var file File
	if err := yaml.Unmarshal(data, &file); err != nil {
		return "", fmt.Errorf("decode app catalog %q: %w", path, err)
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
