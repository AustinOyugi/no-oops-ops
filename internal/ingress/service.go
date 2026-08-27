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
	Environment       string   `json:"environment"`
	App               string   `json:"app"`
	Domain            string   `json:"domain"`
	PathPrefix        string   `json:"path_prefix"`
	Service           string   `json:"service"`
	Port              int      `json:"port"`
	TLS               bool     `json:"tls"`
	TLSCertificate    string   `json:"tls_certificate,omitempty"`
	Domains           []string `json:"domains,omitempty"`
	Websocket         bool     `json:"websocket,omitempty"`
	ClientMaxBodySize string   `json:"client_max_body_size,omitempty"`
}

type Service struct {
	logger *slog.Logger
	config config.Config
	runner commandRunner
}

type commandRunner interface {
	Run(context.Context, string, []string, command.RunOptions) (command.Result, error)
}

func NewService(logger *slog.Logger, cfg config.Config) *Service {
	return &Service{logger: logger, config: cfg, runner: command.NewRunner(logger)}
}

// SetACMEEmail updates the email used by certificates issued during this
// process. The deploy command may obtain it interactively after services have
// already been constructed.
func (s *Service) SetACMEEmail(email string) {
	s.config.ACMEEmail = email
}

// EnsureNetwork connects the managed ingress service to an application
// environment network. The connection is retained in workspace state so later
// applications in the same environment do not update nginx again.
func (s *Service) EnsureNetwork(ctx context.Context, network string) error {
	networks, err := s.loadNetworks()
	if err != nil {
		return err
	}
	if networks[network] {
		return nil
	}
	service := s.config.NginxName + "_nginx"
	if _, err := s.runner.Run(ctx, "docker", []string{"service", "update", "--network-add", network, service}, command.RunOptions{LogCommand: true}); err != nil {
		return fmt.Errorf("attach ingress service %q to network %q: %w", service, network, err)
	}
	networks[network] = true
	data, err := json.MarshalIndent(networks, "", "  ")
	if err != nil {
		return fmt.Errorf("encode ingress networks: %w", err)
	}
	return atomicWrite(s.networksPath(), append(data, '\n'))
}

func (s *Service) Reconcile(ctx context.Context, environment string, m manifest.Manifest, upstreamService string) error {
	routes, err := s.loadRoutes()
	if err != nil {
		return err
	}

	updated, changed, err := updateRoute(routes, environment, m, upstreamService)
	if err != nil {
		return err
	}
	if !changed {
		if len(routes) == 0 {
			return nil
		}
		// Platform-wide ingress settings, including Cloudflare trusted proxy
		// configuration, may have changed even when this app's route did not.
		if err := s.writeConfig(routes); err != nil {
			return err
		}
		return s.reload(ctx)
	}
	if err := s.validateImportedCertificates(updated); err != nil {
		return err
	}
	if err := s.validateCloudflareRoutes(updated); err != nil {
		return err
	}
	// Write and load the HTTP configuration first. This makes the ACME
	// challenge endpoint reachable before asking Let's Encrypt for a
	// certificate. writeConfig only enables TLS for certificates that are
	// already present on disk.
	if err := s.writeConfig(updated); err != nil {
		return err
	}
	if err := s.writeRoutes(updated); err != nil {
		return err
	}
	if err := s.reload(ctx); err != nil {
		return err
	}
	issued, err := s.issueMissingCertificates(ctx, updated)
	if err != nil {
		return err
	}
	if !issued {
		return nil
	}
	if err := s.writeConfig(updated); err != nil {
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
		return nil
	}
	if err := s.writeConfig(updated); err != nil {
		return err
	}
	if err := s.writeRoutes(updated); err != nil {
		return err
	}
	return s.reload(ctx)
}

func (s *Service) ingressDir() string   { return filepath.Join(s.config.StateDir, "nginx") }
func (s *Service) routesPath() string   { return filepath.Join(s.ingressDir(), "routes.json") }
func (s *Service) networksPath() string { return filepath.Join(s.ingressDir(), "networks.json") }
func (s *Service) configPath() string   { return filepath.Join(s.ingressDir(), "conf", "routes.conf") }
func (s *Service) configDir() string    { return filepath.Join(s.ingressDir(), "conf") }
func (s *Service) acmeWebroot() string {
	return filepath.Join(s.config.DataDir, "nginx", "acme-webroot")
}

func (s *Service) loadNetworks() (map[string]bool, error) {
	data, err := os.ReadFile(s.networksPath())
	if os.IsNotExist(err) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read ingress networks %q: %w", s.networksPath(), err)
	}
	var networks map[string]bool
	if err := json.Unmarshal(data, &networks); err != nil {
		return nil, fmt.Errorf("decode ingress networks %q: %w", s.networksPath(), err)
	}
	return networks, nil
}
func (s *Service) certificateDir() string {
	return filepath.Join(s.config.DataDir, "nginx", "letsencrypt")
}
func (s *Service) importedCertificateDir() string {
	return filepath.Join(s.config.DataDir, "nginx", "certificates")
}

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
	files, err := RenderFiles(s.routesWithAvailableCertificates(routes))
	if err != nil {
		return err
	}
	for _, directory := range []string{"external", "internal"} {
		if err := os.RemoveAll(filepath.Join(s.configDir(), directory)); err != nil {
			return fmt.Errorf("clear generated nginx %s routes: %w", directory, err)
		}
	}
	if err := os.Remove(s.configPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove legacy nginx routes config: %w", err)
	}
	cloudflarePath := filepath.Join(s.configDir(), "cloudflare.conf")
	if s.config.NginxCloudflare {
		if err := atomicWrite(cloudflarePath, []byte(cloudflareRealIPConfig)); err != nil {
			return fmt.Errorf("write Cloudflare real IP config: %w", err)
		}
	} else if err := os.Remove(cloudflarePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove Cloudflare real IP config: %w", err)
	}
	var hasExternal, hasInternal bool
	for path := range files {
		hasExternal = hasExternal || strings.HasPrefix(path, "external/")
		hasInternal = hasInternal || strings.HasPrefix(path, "internal/")
	}
	defaultConfig, err := renderTemplate(configTemplateData{}, "default")
	if err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(s.configDir(), "default.conf"), defaultConfig); err != nil {
		return err
	}
	externalConfig, err := renderTemplate(configTemplateData{HasExternal: hasExternal}, "external-include")
	if err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(s.configDir(), "external.conf"), externalConfig); err != nil {
		return err
	}
	internalConfig, err := renderTemplate(configTemplateData{HasInternal: hasInternal}, "internal-server")
	if err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(s.configDir(), "internal.conf"), internalConfig); err != nil {
		return err
	}
	for path, data := range files {
		if err := atomicWrite(filepath.Join(s.configDir(), path), data); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) routesWithAvailableCertificates(routes []Route) []Route {
	configured := append([]Route(nil), routes...)
	for i := range configured {
		if configured[i].TLS {
			if configured[i].TLSCertificate != "" {
				_, err := os.Stat(filepath.Join(s.importedCertificateDir(), configured[i].TLSCertificate, "fullchain.pem"))
				configured[i].TLS = err == nil
				continue
			}
			_, err := os.Stat(filepath.Join(s.certificateDir(), "live", configured[i].Domain, "fullchain.pem"))
			configured[i].TLS = err == nil
		}
	}
	return configured
}

func (s *Service) validateImportedCertificates(routes []Route) error {
	for _, route := range routes {
		if route.TLSCertificate == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(s.importedCertificateDir(), route.TLSCertificate, "fullchain.pem")); err != nil {
			return fmt.Errorf("imported TLS certificate %q is unavailable: %w", route.TLSCertificate, err)
		}
		if _, err := os.Stat(filepath.Join(s.importedCertificateDir(), route.TLSCertificate, "privkey.pem")); err != nil {
			return fmt.Errorf("imported TLS private key for %q is unavailable: %w", route.TLSCertificate, err)
		}
	}
	return nil
}

func (s *Service) validateCloudflareRoutes(routes []Route) error {
	if !s.config.NginxCloudflare {
		return nil
	}
	for _, route := range routes {
		if route.TLS && route.TLSCertificate == "" {
			return fmt.Errorf("Cloudflare ingress requires tls_certificate for HTTPS route %q; import a Cloudflare Origin certificate with `noops certificate import`", route.Domain)
		}
	}
	return nil
}

func (s *Service) issueMissingCertificates(ctx context.Context, routes []Route) (bool, error) {
	issued := make(map[string]bool)
	for _, route := range routes {
		if !route.TLS || route.TLSCertificate != "" || issued[route.Domain] {
			continue
		}
		if _, err := os.Stat(filepath.Join(s.certificateDir(), "live", route.Domain, "fullchain.pem")); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return false, fmt.Errorf("inspect TLS certificate for %q: %w", route.Domain, err)
		}
		if s.config.ACMEEmail == "" {
			return false, fmt.Errorf("ACME email is required to issue a TLS certificate for %q", route.Domain)
		}

		s.logger.InfoContext(ctx, "issuing TLS certificate", "domain", route.Domain)
		result, err := s.runner.Run(ctx, "docker", []string{
			"run", "--rm",
			"--volume", s.acmeWebroot() + ":/var/www/certbot",
			"--volume", s.certificateDir() + ":/etc/letsencrypt",
			"certbot/certbot:latest",
			"certonly", "--webroot", "--webroot-path", "/var/www/certbot",
			"--email", s.config.ACMEEmail, "--agree-tos", "--non-interactive", "--keep-until-expiring",
			"--domains", route.Domain,
		}, command.RunOptions{LogCommand: true})
		if err != nil {
			return false, fmt.Errorf("issue TLS certificate for %q: %w: %s", route.Domain, err, strings.TrimSpace(string(result.Output)))
		}
		issued[route.Domain] = true
	}
	return len(issued) > 0, nil
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

func updateRoute(routes []Route, environment string, m manifest.Manifest, upstreamService string) ([]Route, bool, error) {
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
		Environment:       environment,
		App:               m.Name,
		Domain:            m.Expose.Domain,
		PathPrefix:        m.Expose.PathPrefix,
		Service:           upstreamService,
		Port:              m.Service.InternalPort,
		TLS:               m.Expose.TLS || m.Expose.TLSCertificate != "",
		TLSCertificate:    m.Expose.TLSCertificate,
		Domains:           append([]string(nil), m.Expose.Domains...),
		Websocket:         m.Expose.Proxy.Websocket,
		ClientMaxBodySize: m.Expose.Proxy.ClientMaxBodySize,
	}
	for _, existing := range updated {
		if existing.Domain == route.Domain && existing.PathPrefix == route.PathPrefix {
			return nil, false, fmt.Errorf("ingress route %s%s is already owned by %s/%s", route.Domain, route.PathPrefix, existing.Environment, existing.App)
		}
		if existing.Domain == route.Domain && existing.TLS != route.TLS {
			return nil, false, fmt.Errorf("ingress domain %q cannot mix TLS and non-TLS routes", route.Domain)
		}
		if existing.Domain == route.Domain && existing.TLSCertificate != route.TLSCertificate {
			return nil, false, fmt.Errorf("ingress domain %q cannot mix TLS certificate settings", route.Domain)
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
	for _, domain := range m.Expose.Domains {
		if domain == "" || strings.ContainsAny(domain, " /\\?#{};\"") {
			return fmt.Errorf("expose.domains contains an invalid domain")
		}
	}
	if m.Expose.TLS && m.Expose.TLSCertificate != "" {
		return fmt.Errorf("expose.tls and expose.tls_certificate cannot both be set")
	}
	if m.Expose.TLSCertificate != "" && !certificateName.MatchString(m.Expose.TLSCertificate) {
		return fmt.Errorf("expose.tls_certificate must contain only lowercase letters, numbers, and hyphens")
	}
	if m.Expose.Proxy.ClientMaxBodySize != "" && strings.ContainsAny(m.Expose.Proxy.ClientMaxBodySize, " ;\\\"") {
		return fmt.Errorf("expose.proxy.client_max_body_size contains unsupported characters")
	}
	if !strings.HasPrefix(m.Expose.PathPrefix, "/") || strings.ContainsAny(m.Expose.PathPrefix, " ?#{};\"") {
		return fmt.Errorf("expose.path_prefix must be an absolute HTTP path without query or fragment")
	}
	return nil
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
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {

		}
	}(tempPath)
	if err := temp.Chmod(fileMode); err != nil {
		err := temp.Close()
		if err != nil {
			return err
		}
		return fmt.Errorf("set ingress file permissions: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		err := temp.Close()
		if err != nil {
			return err
		}
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
