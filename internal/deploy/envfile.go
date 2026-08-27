package deploy

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

type EnvFile struct {
	Sections []EnvSection `yaml:"sections"`
}

type EnvSection struct {
	Name  string    `yaml:"name"`
	Items []EnvItem `yaml:"items"`
}

type EnvItem struct {
	Key        string            `yaml:"key"`
	Value      string            `yaml:"value"`
	Values     map[string]string `yaml:"values"`
	FromSecret string            `yaml:"from_secret"`
}

func LoadEnvFile(path string) (EnvFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return EnvFile{}, fmt.Errorf("read env file %q: %w", path, err)
	}

	var envFile EnvFile
	if err := yaml.Unmarshal(data, &envFile); err != nil {
		return EnvFile{}, fmt.Errorf("decode env file %q: %w", path, err)
	}

	return envFile, nil
}

// LoadOptionalEnvFile returns an empty environment when a manifest does not
// declare x-noops.env.file. Static applications commonly need no deployment
// environment or secret bindings.
func LoadOptionalEnvFile(path string) (EnvFile, error) {
	if path == "" {
		return EnvFile{}, nil
	}
	return LoadEnvFile(path)
}
