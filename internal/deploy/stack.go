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
	ServiceName            string
	Image                  string
	Network                string
	Replicas               int
	HealthcheckTest        []string
	HealthcheckInterval    string
	HealthcheckTimeout     string
	HealthcheckRetries     int
	HealthcheckStartPeriod string
	Parallelism            int
	RolloutDelay           string
	RolloutOrder           string
	FailureAction          string
	RestartCondition       string
	RestartDelay           string
	RestartMaxAttempts     int
	RestartWindow          string
	Secrets                []SecretBinding
	Volumes                []string
	NamedVolumes           []string
	UseWrapper             bool
	WrapperImage           string
	OriginalCommand        string
	SecretMappings         string
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
	dir := appDir(cfg, m.Name, environment)
	if err := os.MkdirAll(dir, appDirMode); err != nil {
		return "", fmt.Errorf("create app dir %q: %w", dir, err)
	}

	stackImage := image
	if wrapperCfg.UseWrapper {
		stackImage = wrapperCfg.WrapperImage
	}

	rendered, err := renderStackTemplate(stackTemplateData{
		ServiceName:            serviceName(environment, m.Name),
		Image:                  stackImage,
		Network:                m.Service.Network,
		Replicas:               m.Service.Replicas,
		HealthcheckTest:        m.Healthcheck.Test,
		HealthcheckInterval:    m.Healthcheck.Interval,
		HealthcheckTimeout:     m.Healthcheck.Timeout,
		HealthcheckRetries:     m.Healthcheck.Retries,
		HealthcheckStartPeriod: m.Healthcheck.StartPeriod,
		Parallelism:            m.Rollout.Parallelism,
		RolloutDelay:           m.Rollout.Delay,
		RolloutOrder:           m.Rollout.Order,
		FailureAction:          m.Rollout.FailureAction,
		RestartCondition:       m.Rollout.RestartCondition,
		RestartDelay:           m.Rollout.RestartDelay,
		RestartMaxAttempts:     m.Rollout.RestartMaxAttempts,
		RestartWindow:          m.Rollout.RestartWindow,
		Secrets:                secrets,
		Volumes:                m.Volumes,
		NamedVolumes:           namedVolumes(m.Volumes),
		UseWrapper:             wrapperCfg.UseWrapper,
		WrapperImage:           wrapperCfg.WrapperImage,
		OriginalCommand:        jsonStringSlice(append(wrapperCfg.EffectiveExec.Entrypoint, wrapperCfg.EffectiveExec.Cmd...)),
		SecretMappings:         secretMappingsValue(wrapperCfg.SecretMappings),
	})
	if err != nil {
		return "", err
	}

	rendered = append(rendered, '\n')

	path := stackPath(cfg, m.Name, environment)
	if err := os.WriteFile(path, rendered, stackFileMode); err != nil {
		return "", fmt.Errorf("write stack file %q: %w", path, err)
	}

	return path, nil
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
