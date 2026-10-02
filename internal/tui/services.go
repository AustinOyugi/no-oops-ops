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
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"gopkg.in/yaml.v3"
)

type Row struct {
	ID, Environment, App, Service, Replicas, State string
	CreatedAt                                      time.Time
	Untracked                                      bool
}
type owner struct{ environment, app string }

// Generated manifests and deployment history provide exact service ownership.
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
	// Deployment history survives removal of a generated blue/green manifest.
	history, _ := filepath.Glob(filepath.Join(cfg.StateDir, "apps", "*", "*", "deployments", "*.json"))
	for _, path := range history {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var record struct {
			ServiceName string `json:"service_name"`
			StackName   string `json:"stack_name"`
		}
		if json.Unmarshal(data, &record) != nil {
			continue
		}
		envDir := filepath.Dir(filepath.Dir(path))
		o := owner{filepath.Base(envDir), filepath.Base(filepath.Dir(envDir))}
		name := record.ServiceName
		if name == "" && record.StackName != "" {
			suffix := "app"
			if record.StackName == o.environment+"-"+o.app {
				suffix = record.StackName
			}
			name = record.StackName + "_" + suffix
		}
		if name != "" {
			if _, known := result[name]; !known {
				result[name] = o
			}
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
	rows, err := parseServices(string(output), owners)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	resolveCatalogServices(cfg, rows)
	args := []string{"service", "inspect", "--format", `{"Name":{{json .Spec.Name}},"CreatedAt":{{json .CreatedAt}}}`}
	for _, row := range rows {
		args = append(args, row.Service)
	}
	output, err = exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		return rows, nil // Services may disappear between listing and inspection.
	}
	if err := applyServiceAges(rows, string(output)); err != nil {
		return rows, nil // Keep discovery visible even when optional ages are unavailable.
	}
	return rows, nil
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
			o = owner{"—", "untracked"}
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
		rows = append(rows, Row{ID: service.ID, Environment: o.environment, App: o.app, Service: service.Name, Replicas: service.Replicas, State: state, Untracked: !ok})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Service < rows[j].Service })
	return rows, nil
}

func applyServiceAges(rows []Row, output string) error {
	dates := map[string]time.Time{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		var item struct {
			Name      string
			CreatedAt time.Time
		}
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return fmt.Errorf("decode service age: %w", err)
		}
		dates[item.Name] = item.CreatedAt
	}
	for i := range rows {
		rows[i].CreatedAt = dates[rows[i].Service]
	}
	return nil
}
func serviceAge(row Row, now time.Time) string {
	if row.CreatedAt.IsZero() {
		return "—"
	}
	return uptime(Task{State: "running", StartedAt: row.CreatedAt}, now)
}
