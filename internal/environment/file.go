// Package environment loads and resolves No-Oops environment manifests.
//
// It deliberately keeps ordinary values separate from secret references so
// callers can use public configuration during an image build without ever
// materialising a managed secret there.
package environment

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type File struct {
	Sections []Section `yaml:"sections"`
}

type Section struct {
	Name  string `yaml:"name"`
	Items []Item `yaml:"items"`
}

type Item struct {
	Key        string            `yaml:"key"`
	Value      string            `yaml:"value"`
	Values     map[string]string `yaml:"values"`
	FromSecret string            `yaml:"from_secret"`
}

type Resolved struct {
	SecretRefs []SecretRef
	Values     map[string]string
}

type SecretRef struct {
	Key        string
	SecretName string
}

func Load(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, fmt.Errorf("read env file %q: %w", path, err)
	}

	var file File
	if err := yaml.Unmarshal(data, &file); err != nil {
		return File{}, fmt.Errorf("decode env file %q: %w", path, err)
	}

	return file, nil
}

// LoadOptional returns an empty environment for apps that do not declare an
// environment file.
func LoadOptional(path string) (File, error) {
	if path == "" {
		return File{}, nil
	}
	return Load(path)
}

// Resolve selects ordinary values for an environment and, only for the
// supplied allow-list, managed-secret references.
func Resolve(file File, environment string, resolvable []string) Resolved {
	resolved := Resolved{Values: map[string]string{}}
	allowset := make(map[string]struct{}, len(resolvable))
	for _, key := range resolvable {
		allowset[key] = struct{}{}
	}

	for _, section := range file.Sections {
		for _, item := range section.Items {
			if item.Key == "" {
				continue
			}
			if item.FromSecret != "" {
				if _, ok := allowset[item.Key]; ok {
					resolved.SecretRefs = append(resolved.SecretRefs, SecretRef{Key: item.Key, SecretName: item.FromSecret})
				}
				continue
			}
			if value, ok := item.Values[environment]; ok {
				resolved.Values[item.Key] = value
				continue
			}
			if item.Value != "" {
				resolved.Values[item.Key] = item.Value
			}
		}
	}

	return resolved
}
