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
	"github.com/AustinOyugi/no-oops-ops/internal/ingressnet"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"github.com/AustinOyugi/no-oops-ops/internal/nginxconfig"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"github.com/AustinOyugi/no-oops-ops/internal/state"
)

const (
	dirMode  = 0o700
	fileMode = 0o600

	internalHost = "ingress.noops.internal"
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
	unlock, err := state.AcquireLock(ctx, filepath.Join(s.ingressDir(), "operation.lock"))
	if err != nil {
		return err
	}
	defer unlock()
	service := s.config.NginxName + "_nginx"
	networks, err := ingressnet.Attached(ctx, s.runner, service)
	if err != nil {
		return err
	}
	if networks[network] {
		return s.saveNetworks(networks)
	}
	attachment := "name=" + network + ",alias=" + internalHost
	if _, err := s.runner.Run(ctx, "docker", []string{"service", "update", "--network-add", attachment, service}, command.RunOptions{LogCommand: true}); err != nil {
		return fmt.Errorf("attach ingress service %q to network %q: %w", service, network, err)
	}
	networks[network] = true
	return s.saveNetworks(networks)
}

func (s *Service) saveNetworks(networks map[string]bool) error {
	data, err := json.MarshalIndent(networks, "", "  ")
	if err != nil {
		return fmt.Errorf("encode ingress networks: %w", err)
	}
	return atomicWrite(s.networksPath(), append(data, '\n'))
}

func (s *Service) Reconcile(ctx context.Context, environment string, m manifest.Manifest, upstreamService string) (operationErr error) {
	unlock, err := state.AcquireLock(ctx, filepath.Join(s.ingressDir(), "operation.lock"))
	if err != nil {
		return err
	}
	defer unlock()
	routes, err := s.loadRoutes()
	if err != nil {
		return err
	}

	rollback, err := s.rollbackOnFailure(ctx)
	if err != nil {
		return err
	}
	defer rollback(&operationErr)
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
		if err := s.writeConfig(ctx, routes); err != nil {
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
	if err := s.writeConfig(ctx, updated); err != nil {
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
	if err := s.writeConfig(ctx, updated); err != nil {
		return err
	}
	return s.reload(ctx)
}

func (s *Service) Remove(ctx context.Context, environment, app string) (operationErr error) {
	unlock, err := state.AcquireLock(ctx, filepath.Join(s.ingressDir(), "operation.lock"))
	if err != nil {
		return err
	}
	defer unlock()
	routes, err := s.loadRoutes()
	if err != nil {
		return err
	}

	rollback, err := s.rollbackOnFailure(ctx)
	if err != nil {
		return err
	}
	defer rollback(&operationErr)
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
	if err := s.writeConfig(ctx, updated); err != nil {
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

// writeConfig validates a complete candidate before publishing it with one
// atomic rename. The serving container mounts the parent directory so it sees
// the replacement rather than remaining pinned to an old file inode.
func (s *Service) writeConfig(ctx context.Context, routes []Route) error {
	mainPath := filepath.Join(s.ingressDir(), "nginx.conf")
	if _, err := os.Stat(mainPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("run noops install to migrate ingress before updating routes")
		}
		return err
	}
	body, err := RenderConfig(s.routesWithAvailableCertificates(routes))
	if err != nil {
		return err
	}
	if s.config.NginxCloudflare {
		body = append([]byte(cloudflareRealIPConfig+"\n"), body...)
	}
	// Preserve user-owned snippets, while excluding legacy managed files.
	entries, err := os.ReadDir(s.configDir())
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".conf") {
			continue
		}
		switch entry.Name() {
		case "routes.conf", "default.conf", "external.conf", "internal.conf", "cloudflare.conf":
			continue
		}
		body = append(body, []byte(fmt.Sprintf("\ninclude %q;\n", "/etc/nginx/conf.d/"+entry.Name()))...)
	}
	candidate, err := os.CreateTemp(s.ingressDir(), ".candidate-*.conf")
	if err != nil {
		return err
	}
	path := candidate.Name()
	candidate.Close()
	defer os.Remove(path)
	content := nginxconfig.Wrap(body)
	if err := atomicWrite(path, content); err != nil {
		return err
	}
	if err := s.validateCandidate(ctx, "/etc/noops/nginx/"+filepath.Base(path)); err != nil {
		return err
	}
	return atomicWrite(mainPath, content)
}

func (s *Service) validateCandidate(ctx context.Context, path string) error {
	result, err := s.runner.Run(ctx, "docker", []string{"ps", "-q", "--filter", "label=com.docker.swarm.service.name=" + s.config.NginxName + "_nginx"}, command.RunOptions{})
	if err != nil {
		return fmt.Errorf("find nginx validation containers: %w: %s", err, result.Output)
	}
	containers := strings.Fields(string(result.Output))
	if len(containers) == 0 {
		return fmt.Errorf("nginx has no running containers to validate candidate configuration")
	}
	for _, container := range containers {
		result, err := s.runner.Run(ctx, "docker", []string{"exec", container, "nginx", "-c", path, "-t"}, command.RunOptions{LogCommand: true})
		if err != nil {
			return fmt.Errorf("validate nginx candidate in %q: %w: %s", container, err, result.Output)
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
		if strings.HasPrefix(route.Domain, "*.") {
			return false, fmt.Errorf("wildcard ingress domain %q requires tls_certificate; ACME HTTP-01 cannot issue wildcard certificates", route.Domain)
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
	result, err := s.runner.Run(ctx, "docker", []string{
		"ps", "-q", "--filter", "label=com.docker.swarm.service.name=" + serviceName,
	}, command.RunOptions{})
	if err != nil {
		return fmt.Errorf("find nginx containers for service %q: %w: %s", serviceName, err, strings.TrimSpace(string(result.Output)))
	}
	containers := strings.Fields(string(result.Output))
	if len(containers) == 0 {
		return fmt.Errorf("nginx service %q has no running containers", serviceName)
	}
	for _, container := range containers {
		result, err = s.runner.Run(ctx, "docker", []string{"exec", container, "nginx", "-c", nginxconfig.ContainerPath, "-t"}, command.RunOptions{LogCommand: true})
		if err != nil {
			return fmt.Errorf("validate nginx configuration in container %q: %w: %s", container, err, strings.TrimSpace(string(result.Output)))
		}
	}
	for _, container := range containers {
		result, err = s.runner.Run(ctx, "docker", []string{"exec", container, "nginx", "-c", nginxconfig.ContainerPath, "-s", "reload"}, command.RunOptions{LogCommand: true})
		if err != nil {
			return fmt.Errorf("reload nginx container %q for service %q: %w: %s", container, serviceName, err, strings.TrimSpace(string(result.Output)))
		}
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
	return state.WriteFile(path, data, fileMode)
}
