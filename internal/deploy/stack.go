package deploy

import (
	"bytes"
	"fmt"
	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"os"
	"path/filepath"
	"text/template"
)

const (
	appDirMode       = 0o700
	stackFileMode    = 0o600
	envFileMode      = 0o600
	appStackTemplate = "internal/deploy/templates/app-stack.yml.tmpl"
)

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
}

func appDir(cfg config.Config, name string) string {
	return filepath.Join(cfg.StateDir, "apps", name)
}

func stackPath(cfg config.Config, name string) string {
	return filepath.Join(appDir(cfg, name), "stack.yml")
}

func envPath(cfg config.Config, name string) string {
	return filepath.Join(appDir(cfg, name), ".env")
}

func serviceName(environment string, appName string) string {
	return environment + "-" + appName
}

func writeEnvMap(cfg config.Config, appName string, values map[string]string) (string, error) {
	dir := appDir(cfg, appName)
	if err := os.MkdirAll(dir, appDirMode); err != nil {
		return "", fmt.Errorf("create app dir %q: %w", dir, err)
	}

	path := envPath(cfg, appName)

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

func writeStack(cfg config.Config, environment string, m manifest.Manifest) (string, error) {
	dir := appDir(cfg, m.Name)
	if err := os.MkdirAll(dir, appDirMode); err != nil {
		return "", fmt.Errorf("create app dir %q: %w", dir, err)
	}

	rendered, err := renderStackTemplate(stackTemplateData{
		ServiceName:            serviceName(environment, m.Name),
		Image:                  fmt.Sprintf("%s:%s", m.Image.Repository, m.Image.Tag),
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
	})
	if err != nil {
		return "", err
	}

	rendered = append(rendered, '\n')

	path := stackPath(cfg, m.Name)
	if err := os.WriteFile(path, rendered, stackFileMode); err != nil {
		return "", fmt.Errorf("write stack file %q: %w", path, err)
	}

	return path, nil
}

func renderStackTemplate(data stackTemplateData) ([]byte, error) {
	tplBytes, err := os.ReadFile(appStackTemplate)
	if err != nil {
		return nil, fmt.Errorf("read stack template %q: %w", appStackTemplate, err)
	}

	tpl, err := template.New(appStackTemplate).Parse(string(tplBytes))
	if err != nil {
		return nil, fmt.Errorf("parse stack template %q: %w", appStackTemplate, err)
	}

	var out bytes.Buffer
	if err := tpl.Execute(&out, data); err != nil {
		return nil, fmt.Errorf("execute stack template %q: %w", appStackTemplate, err)
	}

	return out.Bytes(), nil
}
