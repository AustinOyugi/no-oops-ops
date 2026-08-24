package manifest

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func Load(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest %q: %w", path, err)
	}

	var compose ComposeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest %q: %w", path, err)
	}
	if len(compose.Services) == 0 {
		return Manifest{}, fmt.Errorf("decode Compose manifest %q: services is required", path)
	}
	m, err := composeManifest(compose)
	if err != nil {
		return Manifest{}, fmt.Errorf("decode Compose manifest %q: %w", path, err)
	}

	m.applyDefaults()

	if err := m.Validate(); err != nil {
		return Manifest{}, err
	}

	return m, nil
}

func composeManifest(compose ComposeFile) (Manifest, error) {
	if len(compose.Services) != 1 {
		return Manifest{}, fmt.Errorf("Compose-shaped manifests currently require exactly one service; found %d", len(compose.Services))
	}
	for name, service := range compose.Services {
		image, err := composeImage(service.Image)
		if err != nil {
			return Manifest{}, err
		}
		source := service.NoOps.Source
		if service.Build.Context != "" || service.Build.Dockerfile != "" {
			source.Context = service.Build.Context
			source.Dockerfile = service.Build.Dockerfile
			image.Build = manifestBoolPtr(true)
		}
		serviceConfig := service.NoOps.Service
		serviceConfig.Replicas = service.Deploy.Replicas
		serviceConfig.Network = firstNetwork(service.Networks)
		serviceConfig.Command = service.Command
		return Manifest{Name: name, Source: source, Image: image, Service: serviceConfig, Healthcheck: service.Healthcheck, Rollout: service.NoOps.Rollout, Expose: service.NoOps.Expose, Env: service.NoOps.Env, DependsOn: service.NoOps.DependsOn, Volumes: service.Volumes}, nil
	}
	panic("unreachable")
}

func composeImage(reference string) (Image, error) {
	if reference == "" {
		return Image{}, fmt.Errorf("service.image is required")
	}
	if strings.Contains(reference, "@") {
		return Image{}, fmt.Errorf("digest image references are not yet supported")
	}
	image := Image{Repository: reference, Build: manifestBoolPtr(false)}
	if colon := strings.LastIndex(reference, ":"); colon > strings.LastIndex(reference, "/") {
		image.Repository, image.Tag = reference[:colon], reference[colon+1:]
	}
	return image, nil
}

func firstNetwork(networks []string) string {
	if len(networks) == 0 {
		return ""
	}
	return networks[0]
}

func manifestBoolPtr(value bool) *bool { return &value }
