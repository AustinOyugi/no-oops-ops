package manifest

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Services returns the declared Compose service names in stable order.
func Services(path string) ([]string, error) {
	filePath, _ := splitServicePath(path)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read manifest %q: %w", path, err)
	}
	var compose ComposeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil, fmt.Errorf("decode manifest %q: %w", path, err)
	}
	if len(compose.Services) == 0 {
		return nil, fmt.Errorf("decode Compose manifest %q: services is required", path)
	}
	names := make([]string, 0, len(compose.Services))
	for name := range compose.Services {
		names = append(names, name)
	}
	slices.Sort(names)
	return names, nil
}

func Load(path string) (Manifest, error) {
	filePath, selected := splitServicePath(path)
	if selected != "" {
		return LoadService(filePath, selected)
	}
	services, err := Services(filePath)
	if err != nil {
		return Manifest{}, err
	}
	if len(services) != 1 {
		return Manifest{}, fmt.Errorf("Compose manifest %q contains %d services; select one with LoadService", path, len(services))
	}
	return LoadService(filePath, services[0])
}

// WithService identifies one service while retaining a manifest path accepted
// by the existing release and deploy APIs.
func WithService(path, service string) string { return path + "#" + service }

// LoadService selects one service from a Compose-shaped manifest.
func LoadService(path, selected string) (Manifest, error) {
	path, _ = splitServicePath(path)
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
	m, err := composeManifestService(compose, selected)
	if err != nil {
		return Manifest{}, fmt.Errorf("decode Compose manifest %q: %w", path, err)
	}

	m.applyDefaults()

	if err := m.Validate(); err != nil {
		return Manifest{}, err
	}

	return m, nil
}

func splitServicePath(path string) (string, string) {
	file, service, found := strings.Cut(path, "#")
	if !found {
		return path, ""
	}
	return file, service
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

func composeManifestService(compose ComposeFile, selected string) (Manifest, error) {
	service, ok := compose.Services[selected]
	if !ok {
		return Manifest{}, fmt.Errorf("service %q is not declared", selected)
	}
	return composeManifest(ComposeFile{Services: map[string]ComposeService{selected: service}})
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
