package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"gopkg.in/yaml.v3"
)

type Row struct{ ID, Environment, App, Service, Replicas, State string }
type owner struct{ environment, app string }

// Generated stack manifests identify this workspace's services, including
// blue/green candidates. Avoid guessing ownership from Docker name prefixes.
func managedServices(cfg config.Config) (map[string]owner, error) {
	result := map[string]owner{
		cfg.RegistryName + "_registry": {"platform", "registry"},
		cfg.NginxName + "_nginx":       {"platform", "ingress"},
		cfg.NginxName + "_certbot":     {"platform", "ingress"},
	}
	paths, err := filepath.Glob(filepath.Join(cfg.StateDir, "apps", "*", "*", "stack*.yml"))
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		envDir := filepath.Dir(path)
		o := owner{filepath.Base(envDir), filepath.Base(filepath.Dir(envDir))}
		stack := o.environment + "-" + o.app
		if name := filepath.Base(path); name != "stack.yml" {
			stack = strings.TrimSuffix(strings.TrimPrefix(name, "stack-"), ".yml")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var manifest struct {
			Services map[string]yaml.Node `yaml:"services"`
		}
		if err := yaml.Unmarshal(data, &manifest); err != nil {
			return nil, fmt.Errorf("read stack %s: %w", path, err)
		}
		for service := range manifest.Services {
			result[stack+"_"+service] = o
		}
	}
	return result, nil
}

func Services(ctx context.Context, cfg config.Config) ([]Row, error) {
	owners, err := managedServices(cfg)
	if err != nil {
		return nil, err
	}
	output, err := exec.CommandContext(ctx, "docker", "service", "ls", "--format", "{{json .}}").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("Docker: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return parseServices(string(output), owners)
}

func parseServices(output string, owners map[string]owner) ([]Row, error) {
	var rows []Row
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		var service struct{ ID, Name, Replicas string }
		if err := json.Unmarshal([]byte(line), &service); err != nil {
			return nil, fmt.Errorf("decode Docker service: %w", err)
		}
		o, ok := owners[service.Name]
		if !ok {
			continue
		}
		state := "unknown"
		var running, desired int
		if n, _ := fmt.Sscanf(service.Replicas, "%d/%d", &running, &desired); n == 2 {
			state = "degraded"
			if desired == 0 && running == 0 {
				state = "scaled down"
			} else if desired > 0 && running == desired {
				state = "running"
			}
		}
		rows = append(rows, Row{service.ID, o.environment, o.app, service.Name, service.Replicas, state})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Service < rows[j].Service })
	return rows, nil
}
