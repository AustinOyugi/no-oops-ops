package deploy

import (
	"bytes"
	_ "embed"
	"fmt"
	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	appDirMode       = 0o700
	stackFileMode    = 0o600
	envFileMode      = 0o600
	appStackTemplate = "internal/deploy/templates/app-stack.yml.tmpl"
)

//go:embed templates/app-stack.yml.tmpl
var appStackTemplateContents string

type stackTemplateData struct {
	ServiceName             string
	Image                   string
	Network                 string
	Replicas                int
	HealthcheckTest         []string
	HealthcheckInterval     string
	HealthcheckTimeout      string
	HealthcheckRetries      int
	HealthcheckStartPeriod  string
	Parallelism             int
	RolloutDelay            string
	RolloutOrder            string
	RolloutMonitor          string
	MaxFailureRatio         float64
	FailureAction           string
	RollbackDelay           string
	RollbackOrder           string
	RollbackMonitor         string
	RollbackParallelism     int
	RollbackMaxFailureRatio float64
	RollbackFailureAction   string
	RestartCondition        string
	RestartDelay            string
	RestartMaxAttempts      int
	RestartWindow           string
	Secrets                 []SecretBinding
	Volumes                 []string
	NamedVolumes            []string
	UseWrapper              bool
	WrapperImage            string
	OriginalCommand         string
	SecretMappings          string
}

type SecretBinding struct {
	EnvKey     string `json:"env_key"`
	SecretName string `json:"secret_name"`
	SwarmName  string `json:"swarm_name"`
	Version    int    `json:"version"`
}

func appDir(cfg config.Config, name string, environment string) string {
	return filepath.Join(cfg.StateDir, "apps", name, environment)
}

func stackPath(cfg config.Config, name string, environment string) string {
	return filepath.Join(appDir(cfg, name, environment), "stack.yml")
}

func releaseStackPath(cfg config.Config, name string, environment string, stack string) string {
	return filepath.Join(appDir(cfg, name, environment), "stack-"+stack+".yml")
}

func envPath(cfg config.Config, name string, environment string) string {
	return filepath.Join(appDir(cfg, name, environment), ".env")
}

func serviceName(environment string, appName string) string {
	return environment + "-" + appName
}

func stackName(environment string, appName string) string {
	return environment + "-" + appName
}

func swarmServiceName(environment string, appName string) string {
	return stackName(environment, appName) + "_" + serviceName(environment, appName)
}

func releaseStackName(environment, appName, tag string) string {
	var out strings.Builder
	for _, char := range strings.ToLower(tag) {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' {
			out.WriteRune(char)
		}
	}
	return environment + "-" + appName + "-r" + out.String()
}

// candidateStackName identifies one blue/green attempt. The timestamp keeps
// candidates distinct when the same immutable release is deployed repeatedly.
func candidateStackName(environment, appName, tag string, createdAt time.Time) string {
	deploymentID := fmt.Sprintf("%d", createdAt.UTC().UnixNano())
	return releaseStackName(environment, appName, tag+"-"+deploymentID)
}

func writeEnvMap(cfg config.Config, appName string, environment string, values map[string]string) (string, error) {
	dir := appDir(cfg, appName, environment)
	if err := os.MkdirAll(dir, appDirMode); err != nil {
		return "", fmt.Errorf("create app dir %q: %w", dir, err)
	}

	path := envPath(cfg, appName, environment)

	var out bytes.Buffer
	for key, value := range values {
		if _, err := fmt.Fprintf(&out, "%s=%s\n", key, value); err != nil {
			return "", fmt.Errorf("render env file %q: %w", path, err)
		}
	}

	if err := os.WriteFile(path, out.Bytes(), envFileMode); err != nil {
		return "", fmt.Errorf("write env file %q: %w", path, err)
	}

	return path, nil
}

func writeStack(cfg config.Config, environment string, m manifest.Manifest, image string, secrets []SecretBinding, wrapperCfg WrapperConfig) (string, error) {
	return writeStackForService(cfg, environment, m, image, secrets, wrapperCfg, serviceName(environment, m.Name), stackPath(cfg, m.Name, environment))
}

func writeStackForService(cfg config.Config, environment string, m manifest.Manifest, image string, secrets []SecretBinding, wrapperCfg WrapperConfig, service, path string) (string, error) {
	dir := appDir(cfg, m.Name, environment)
	if err := os.MkdirAll(dir, appDirMode); err != nil {
		return "", fmt.Errorf("create app dir %q: %w", dir, err)
	}

	// Compose manifests are the application contract. Patch the selected raw
	// Compose document instead of recreating it from a limited Go schema.
	if m.Compose != nil {
		rendered, err := renderComposeStack(m, image, secrets, wrapperCfg, service, envPath(cfg, m.Name, environment))
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(path, append(rendered, '\n'), stackFileMode); err != nil {
			return "", fmt.Errorf("write stack file %q: %w", path, err)
		}
		return path, nil
	}

	stackImage := image
	if wrapperCfg.UseWrapper {
		stackImage = wrapperCfg.WrapperImage
	}

	rendered, err := renderStackTemplate(stackTemplateData{
		ServiceName:             service,
		Image:                   stackImage,
		Network:                 m.Service.Network,
		Replicas:                m.Service.Replicas,
		HealthcheckTest:         m.Healthcheck.Test,
		HealthcheckInterval:     m.Healthcheck.Interval,
		HealthcheckTimeout:      m.Healthcheck.Timeout,
		HealthcheckRetries:      m.Healthcheck.Retries,
		HealthcheckStartPeriod:  m.Healthcheck.StartPeriod,
		Parallelism:             m.Rollout.Parallelism,
		RolloutDelay:            m.Rollout.Delay,
		RolloutOrder:            m.Rollout.Order,
		RolloutMonitor:          m.Rollout.Monitor,
		MaxFailureRatio:         m.Rollout.MaxFailureRatio,
		FailureAction:           m.Rollout.FailureAction,
		RollbackDelay:           m.Rollout.Rollback.Delay,
		RollbackOrder:           m.Rollout.Rollback.Order,
		RollbackMonitor:         m.Rollout.Rollback.Monitor,
		RollbackParallelism:     m.Rollout.Rollback.Parallelism,
		RollbackMaxFailureRatio: m.Rollout.Rollback.MaxFailureRatio,
		RollbackFailureAction:   m.Rollout.Rollback.FailureAction,
		RestartCondition:        m.Rollout.RestartCondition,
		RestartDelay:            m.Rollout.RestartDelay,
		RestartMaxAttempts:      m.Rollout.RestartMaxAttempts,
		RestartWindow:           m.Rollout.RestartWindow,
		Secrets:                 secrets,
		Volumes:                 m.Volumes,
		NamedVolumes:            namedVolumes(m.Volumes),
		UseWrapper:              wrapperCfg.UseWrapper,
		WrapperImage:            wrapperCfg.WrapperImage,
		OriginalCommand:         jsonStringSlice(append(wrapperCfg.EffectiveExec.Entrypoint, wrapperCfg.EffectiveExec.Cmd...)),
		SecretMappings:          secretMappingsValue(wrapperCfg.SecretMappings),
	})
	if err != nil {
		return "", err
	}

	rendered = append(rendered, '\n')

	if err := os.WriteFile(path, rendered, stackFileMode); err != nil {
		return "", fmt.Errorf("write stack file %q: %w", path, err)
	}

	return path, nil
}

// renderComposeStack changes only No Oops-owned deployment details. Every
// other Compose value is retained as YAML, including fields unknown to this
// version of No Oops Ops.
func renderComposeStack(m manifest.Manifest, image string, bindings []SecretBinding, wrapper WrapperConfig, serviceName, generatedEnv string) ([]byte, error) {
	raw, err := yaml.Marshal(m.Compose)
	if err != nil {
		return nil, fmt.Errorf("copy Compose manifest: %w", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decode Compose manifest: %w", err)
	}
	root := documentRoot(&doc)
	services := mappingValue(root, "services")
	if services == nil || len(services.Content) != 2 {
		return nil, fmt.Errorf("selected Compose service is missing")
	}
	serviceKey, selected := services.Content[0], services.Content[1]
	serviceKey.Value = serviceName
	removeNoOpsMetadata(root)
	setMapping(selected, "image", scalar(image))
	normalizeServicePaths(selected, filepath.Dir(m.Path))
	appendEnvFile(selected, generatedEnv)
	if wrapper.UseWrapper {
		applyWrapper(selected, wrapper, bindings)
	}
	appendSecrets(selected, bindings)
	appendTopLevelSecrets(root, bindings)
	normalizeTopLevelConfigPaths(root, filepath.Dir(m.Path))
	return yaml.Marshal(&doc)
}

func removeNoOpsMetadata(node *yaml.Node) {
	if node == nil {
		return
	}
	if node.Kind == yaml.MappingNode {
		for i := len(node.Content) - 2; i >= 0; i -= 2 {
			if node.Content[i].Value == "x-noops" {
				node.Content = append(node.Content[:i], node.Content[i+2:]...)
				continue
			}
			removeNoOpsMetadata(node.Content[i+1])
		}
		return
	}
	for _, child := range node.Content {
		removeNoOpsMetadata(child)
	}
}

func documentRoot(doc *yaml.Node) *yaml.Node {
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0]
	}
	return doc
}
func scalar(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}
func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}
func removeMapping(mapping *yaml.Node, key string) {
	if mapping == nil {
		return
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
			return
		}
	}
}
func setMapping(mapping *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1] = value
			return
		}
	}
	mapping.Content = append(mapping.Content, scalar(key), value)
}

func appendEnvFile(service *yaml.Node, generated string) {
	node := mappingValue(service, "env_file")
	if node == nil {
		setMapping(service, "env_file", &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{scalar(generated)}})
		return
	}
	if node.Kind == yaml.ScalarNode {
		node.Kind, node.Tag, node.Value, node.Content = yaml.SequenceNode, "!!seq", "", []*yaml.Node{scalar(node.Value)}
	}
	if node.Kind == yaml.SequenceNode {
		node.Content = append(node.Content, scalar(generated))
	}
}

func appendSecrets(service *yaml.Node, bindings []SecretBinding) {
	if len(bindings) == 0 {
		return
	}
	node := mappingValue(service, "secrets")
	if node == nil {
		node = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		setMapping(service, "secrets", node)
	}
	if node.Kind != yaml.SequenceNode {
		return
	}
	for _, b := range bindings {
		node.Content = append(node.Content, &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{scalar("source"), scalar(b.SwarmName), scalar("target"), scalar(b.EnvKey), scalar("mode"), &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "0444"}}})
	}
}

func appendTopLevelSecrets(root *yaml.Node, bindings []SecretBinding) {
	if len(bindings) == 0 {
		return
	}
	node := mappingValue(root, "secrets")
	if node == nil {
		node = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		setMapping(root, "secrets", node)
	}
	if node.Kind != yaml.MappingNode {
		return
	}
	for _, b := range bindings {
		if mappingValue(node, b.SwarmName) == nil {
			node.Content = append(node.Content, scalar(b.SwarmName), &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{scalar("external"), &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"}}})
		}
	}
}

func applyWrapper(service *yaml.Node, wrapper WrapperConfig, bindings []SecretBinding) {
	setMapping(service, "image", scalar(wrapper.WrapperImage))
	setMapping(service, "entrypoint", &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{scalar("/bin/sh"), scalar("/bootstrap.sh")}})
	command := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, v := range append(wrapper.EffectiveExec.Entrypoint, wrapper.EffectiveExec.Cmd...) {
		command.Content = append(command.Content, scalar(v))
	}
	setMapping(service, "command", command)
	env := mappingValue(service, "environment")
	if env == nil {
		env = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		setMapping(service, "environment", env)
	}
	if env.Kind != yaml.MappingNode {
		return
	}
	setMapping(env, "NOOPS_SECRET_MAPPINGS", scalar(secretMappingsValue(wrapper.SecretMappings)))
	for _, b := range bindings {
		setMapping(env, b.EnvKey+"_FILE", scalar("/run/secrets/"+b.EnvKey))
	}
}

func normalizeServicePaths(service *yaml.Node, base string) {
	if node := mappingValue(service, "env_file"); node != nil {
		normalizePathNode(node, base)
	}
	volumes := mappingValue(service, "volumes")
	if volumes == nil || volumes.Kind != yaml.SequenceNode {
		return
	}
	for _, volume := range volumes.Content {
		if volume.Kind == yaml.ScalarNode {
			source, rest, ok := strings.Cut(volume.Value, ":")
			if ok && (strings.HasPrefix(source, "./") || strings.HasPrefix(source, "../")) {
				volume.Value = filepath.Join(base, source) + ":" + rest
			}
		} else if volume.Kind == yaml.MappingNode {
			source := mappingValue(volume, "source")
			typ := mappingValue(volume, "type")
			if source != nil && (typ == nil || typ.Value == "bind") && !filepath.IsAbs(source.Value) {
				source.Value = filepath.Join(base, source.Value)
			}
		}
	}
}
func normalizeTopLevelConfigPaths(root *yaml.Node, base string) {
	for _, key := range []string{"configs", "secrets"} {
		section := mappingValue(root, key)
		if section == nil || section.Kind != yaml.MappingNode {
			continue
		}
		for i := 1; i < len(section.Content); i += 2 {
			file := mappingValue(section.Content[i], "file")
			if file != nil && !filepath.IsAbs(file.Value) {
				file.Value = filepath.Join(base, file.Value)
			}
		}
	}
}
func normalizePathNode(node *yaml.Node, base string) {
	if node.Kind == yaml.ScalarNode && !filepath.IsAbs(node.Value) {
		node.Value = filepath.Join(base, node.Value)
		return
	}
	if node.Kind == yaml.SequenceNode {
		for _, value := range node.Content {
			normalizePathNode(value, base)
		}
	}
}

// namedVolumes returns the named sources from Docker's short volume syntax.
// Host paths are bind mounts and must not be declared in the stack's top-level
// volumes section.
func namedVolumes(mounts []string) []string {
	seen := make(map[string]struct{})
	volumes := make([]string, 0, len(mounts))

	for _, mount := range mounts {
		source, _, hasTarget := strings.Cut(mount, ":")
		if !hasTarget || isBindMountSource(source) {
			continue
		}
		if _, exists := seen[source]; exists {
			continue
		}
		seen[source] = struct{}{}
		volumes = append(volumes, source)
	}

	return volumes
}

func isBindMountSource(source string) bool {
	return source == "" ||
		strings.HasPrefix(source, "/") ||
		strings.HasPrefix(source, "./") ||
		strings.HasPrefix(source, "../") ||
		strings.HasPrefix(source, "~/") ||
		strings.Contains(source, "/")
}

func renderStackTemplate(data stackTemplateData) ([]byte, error) {
	tpl, err := template.New(appStackTemplate).Funcs(template.FuncMap{
		"yamlString": yamlString,
	}).Parse(appStackTemplateContents)
	if err != nil {
		return nil, fmt.Errorf("parse stack template %q: %w", appStackTemplate, err)
	}

	var out bytes.Buffer
	if err := tpl.Execute(&out, data); err != nil {
		return nil, fmt.Errorf("execute stack template %q: %w", appStackTemplate, err)
	}

	return out.Bytes(), nil
}

func yamlString(value string) string {
	data, err := yaml.Marshal(value)
	if err != nil {
		panic(err)
	}
	return strings.TrimSuffix(string(data), "\n")
}
