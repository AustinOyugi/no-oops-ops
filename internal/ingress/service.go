// Package ingress manages the nginx routes for publicly exposed applications.
package ingress

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

const (
	dirMode  = 0o700
	fileMode = 0o600
)

type Route struct {
	Environment string `json:"environment"`
	App         string `json:"app"`
	Domain      string `json:"domain"`
	PathPrefix  string `json:"path_prefix"`
	Service     string `json:"service"`
	Port        int    `json:"port"`
}

type Service struct {
	logger *slog.Logger
	config config.Config
	runner *command.Runner
}

func NewService(logger *slog.Logger, cfg config.Config) *Service {
	return &Service{logger: logger, config: cfg, runner: command.NewRunner(logger)}
}

func (s *Service) Reconcile(ctx context.Context, environment string, m manifest.Manifest) error {
	routes, err := s.loadRoutes()
	if err != nil {
		return err
	}

	updated, changed, err := updateRoute(routes, environment, m)
	if err != nil {
		return err
	}
	if !changed {
		return s.reload(ctx)
	}
	if err := s.writeConfig(updated); err != nil {
		return err
	}
	if err := s.writeRoutes(updated); err != nil {
		return err
	}
	return s.reload(ctx)
}

func (s *Service) Remove(ctx context.Context, environment, app string) error {
	routes, err := s.loadRoutes()
	if err != nil {
		return err
	}

	updated := make([]Route, 0, len(routes))
	for _, route := range routes {
		if route.Environment == environment && route.App == app {
			continue
		}
		updated = append(updated, route)
	}
	if len(updated) == len(routes) {
		return s.reload(ctx)
	}
	if err := s.writeConfig(updated); err != nil {
		return err
	}
	if err := s.writeRoutes(updated); err != nil {
		return err
	}
	return s.reload(ctx)
}

func (s *Service) ingressDir() string { return filepath.Join(s.config.StateDir, "nginx") }
func (s *Service) routesPath() string { return filepath.Join(s.ingressDir(), "routes.json") }
func (s *Service) configPath() string { return filepath.Join(s.ingressDir(), "conf", "routes.conf") }

func (s *Service) loadRoutes() ([]Route, error) {
	data, err := os.ReadFile(s.routesPath())
	if os.IsNotExist(err) {
		return []Route{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read ingress routes %q: %w", s.routesPath(), err)
	}
	var routes []Route
	if err := json.Unmarshal(data, &routes); err != nil {
		return nil, fmt.Errorf("decode ingress routes %q: %w", s.routesPath(), err)
	}
	return routes, nil
}

func (s *Service) writeRoutes(routes []Route) error {
	data, err := json.MarshalIndent(routes, "", "  ")
	if err != nil {
		return fmt.Errorf("encode ingress routes: %w", err)
	}
	return atomicWrite(s.routesPath(), append(data, '\n'))
}

func (s *Service) writeConfig(routes []Route) error {
	data, err := RenderConfig(routes)
	if err != nil {
		return err
	}
	return atomicWrite(s.configPath(), data)
}

func (s *Service) reload(ctx context.Context) error {
	serviceName := s.config.NginxName + "_nginx"
	s.logger.InfoContext(ctx, "reconciling nginx ingress", "service", serviceName)
	result, err := s.runner.Run(ctx, "docker", []string{"service", "update", "--force", serviceName}, command.RunOptions{LogCommand: true})
	if err != nil {
		return fmt.Errorf("reload nginx service %q: %w: %s", serviceName, err, strings.TrimSpace(string(result.Output)))
	}
	return nil
}

func updateRoute(routes []Route, environment string, m manifest.Manifest) ([]Route, bool, error) {
	updated := make([]Route, 0, len(routes)+1)
	for _, route := range routes {
		if route.Environment == environment && route.App == m.Name {
			continue
		}
		updated = append(updated, route)
	}

	if !m.Expose.Enabled {
		return updated, len(updated) != len(routes), nil
	}
	if err := validateExposure(m); err != nil {
		return nil, false, err
	}
	route := Route{
		Environment: environment,
		App:         m.Name,
		Domain:      m.Expose.Domain,
		PathPrefix:  m.Expose.PathPrefix,
		Service:     swarmServiceName(environment, m.Name),
		Port:        m.Service.InternalPort,
	}
	for _, existing := range updated {
		if existing.Domain == route.Domain && existing.PathPrefix == route.PathPrefix {
			return nil, false, fmt.Errorf("ingress route %s%s is already owned by %s/%s", route.Domain, route.PathPrefix, existing.Environment, existing.App)
		}
	}
	updated = append(updated, route)
	sortRoutes(updated)
	return updated, true, nil
}

func validateExposure(m manifest.Manifest) error {
	if m.Expose.Domain == "" {
		return fmt.Errorf("expose.domain is required when expose.enabled is true")
	}
	if strings.ContainsAny(m.Expose.Domain, " /\\?#{};\"") {
		return fmt.Errorf("expose.domain contains unsupported characters")
	}
	if !strings.HasPrefix(m.Expose.PathPrefix, "/") || strings.ContainsAny(m.Expose.PathPrefix, " ?#{};\"") {
		return fmt.Errorf("expose.path_prefix must be an absolute HTTP path without query or fragment")
	}
	return nil
}

func swarmServiceName(environment, app string) string {
	stack := environment + "-" + app
	return stack + "_" + stack
}

func sortRoutes(routes []Route) {
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Domain != routes[j].Domain {
			return routes[i].Domain < routes[j].Domain
		}
		return routes[i].PathPrefix < routes[j].PathPrefix
	})
}

func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		return fmt.Errorf("create ingress state directory %q: %w", filepath.Dir(path), err)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary ingress file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(fileMode); err != nil {
		temp.Close()
		return fmt.Errorf("set ingress file permissions: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write temporary ingress file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary ingress file: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace ingress file %q: %w", path, err)
	}
	return nil
}
