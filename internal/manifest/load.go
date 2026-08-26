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

// DeploymentOrder returns a stable topological order using x-noops.depends_on.
// Compose depends_on is intentionally not treated as readiness: Swarm does not
// provide that guarantee.
func DeploymentOrder(path string) ([]string, error) {
	names, err := Services(path)
	if err != nil {
		return nil, err
	}
	known := make(map[string]bool, len(names))
	for _, name := range names {
		known[name] = true
	}
	deps := make(map[string][]string, len(names))
	for _, name := range names {
		m, err := LoadService(path, name)
		if err != nil {
			return nil, err
		}
		for _, dep := range m.DependsOn {
			if !known[dep] {
				return nil, fmt.Errorf("service %q depends on undeclared service %q", name, dep)
			}
			deps[name] = append(deps[name], dep)
		}
		slices.Sort(deps[name])
	}
	state := make(map[string]int, len(names))
	ordered := make([]string, 0, len(names))
	var visit func(string) error
	visit = func(name string) error {
		switch state[name] {
		case 1:
			return fmt.Errorf("x-noops.depends_on contains a cycle at service %q", name)
		case 2:
			return nil
		}
		state[name] = 1
		for _, dep := range deps[name] {
			if err := visit(dep); err != nil {
				return err
			}
		}
		state[name] = 2
		ordered = append(ordered, name)
		return nil
	}
	for _, name := range names {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	return ordered, nil
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
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest %q: %w", path, err)
	}
	if len(compose.Services) == 0 {
		return Manifest{}, fmt.Errorf("decode Compose manifest %q: services is required", path)
	}
	m, err := composeManifestService(compose, selected)
	if err != nil {
		return Manifest{}, fmt.Errorf("decode Compose manifest %q: %w", path, err)
	}
	m.Compose = selectComposeDocument(&document, selected)
	m.Path = path
	if service := selectedServiceNode(m.Compose); service != nil {
		if command := composeCommand(service, "command"); command != nil {
			m.Service.Command = command
		}
		if entrypoint := composeCommand(service, "entrypoint"); entrypoint != nil {
			m.Service.Entrypoint = entrypoint
		}
	}

	m.applyDefaults()

	if err := m.Validate(); err != nil {
		return Manifest{}, err
	}

	return m, nil
}

func selectedServiceNode(document *yaml.Node) *yaml.Node {
	root := document
	if root != nil && root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		root = root.Content[0]
	}
	services := mapValue(root, "services")
	if services == nil || len(services.Content) != 2 {
		return nil
	}
	return services.Content[1]
}

// Compose accepts shell-form command values. Preserve that contract by making
// the shell explicit when the secret wrapper needs to replay it.
func composeCommand(service *yaml.Node, key string) []string {
	node := mapValue(service, key)
	if node == nil {
		return nil
	}
	if node.Kind == yaml.ScalarNode {
		return []string{"/bin/sh", "-c", node.Value}
	}
	if node.Kind != yaml.SequenceNode {
		return nil
	}
	values := make([]string, 0, len(node.Content))
	for _, value := range node.Content {
		values = append(values, value.Value)
	}
	return values
}

// selectComposeDocument retains all top-level Compose definitions but emits
// only the selected service. This keeps independently deployed services from
// being changed as a side effect of a deploy.
func selectComposeDocument(document *yaml.Node, selected string) *yaml.Node {
	if document == nil {
		return nil
	}
	clone := cloneNode(document)
	root := clone
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		root = root.Content[0]
	}
	services := mapValue(root, "services")
	if services == nil {
		return clone
	}
	for i := len(services.Content) - 2; i >= 0; i -= 2 {
		if services.Content[i].Value != selected {
			services.Content = append(services.Content[:i], services.Content[i+2:]...)
		}
	}
	return clone
}

func cloneNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	copy := *node
	copy.Content = make([]*yaml.Node, len(node.Content))
	for i, child := range node.Content {
		copy.Content[i] = cloneNode(child)
	}
	return &copy
}

func mapValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
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
		ingress := service.NoOps.Ingress
		if !hasIngress(ingress) {
			ingress = service.NoOps.Expose
		}
		normalizeIngress(&ingress)
		return Manifest{Name: name, Source: source, Image: image, Service: serviceConfig, Healthcheck: service.Healthcheck, Rollout: service.NoOps.Rollout, Expose: ingress, Env: service.NoOps.Env, Build: service.NoOps.Build, DependsOn: service.NoOps.DependsOn, Volumes: service.Volumes}, nil
	}
	panic("unreachable")
}

func hasIngress(ingress Expose) bool {
	return ingress.Enabled || ingress.Domain != "" || len(ingress.Domains) > 0 || ingress.BlueGreen != nil || ingress.TLS || ingress.TLSCertificate != "" || ingress.PathPrefix != ""
}

func normalizeIngress(ingress *Expose) {
	if ingress.Domain == "" && len(ingress.Domains) > 0 {
		ingress.Domain = ingress.Domains[0]
	}
	// Domains are an unambiguous request for a managed route, so the compact
	// `ingress: {domains: [...]}` form is sufficient.
	if ingress.Domain != "" {
		ingress.Enabled = true
	}
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
	image := Image{Repository: reference, SourceReference: reference, Build: manifestBoolPtr(false)}
	if repository, _, digest := strings.Cut(reference, "@"); digest {
		image.Repository = repository
		return image, nil
	}
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
