package ingress

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

type recordingRunner struct {
	calls [][]string
}

func (r *recordingRunner) Run(_ context.Context, name string, args []string, _ command.RunOptions) (command.Result, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if len(args) > 1 && args[0] == "service" && args[1] == "inspect" {
		return command.Result{Output: []byte("[]")}, nil
	}
	return command.Result{}, nil
}

func TestUpdateRouteAddsExposedApp(t *testing.T) {
	m := manifest.Manifest{
		Name:    "sample",
		Service: manifest.Service{InternalPort: 8080},
		Expose:  manifest.Expose{Enabled: true, Domain: "sample.example.test", PathPrefix: "/"},
	}
	routes, changed, err := updateRoute(nil, "dev", m, "dev-sample_dev-sample")
	if err != nil {
		t.Fatal(err)
	}
	if !changed || len(routes) != 1 {
		t.Fatalf("changed=%v routes=%v", changed, routes)
	}
	if got, want := routes[0].Service, "dev-sample_dev-sample"; got != want {
		t.Errorf("service = %q, want %q", got, want)
	}
}

func TestUpdateRouteCarriesTLSSetting(t *testing.T) {
	m := manifest.Manifest{Name: "sample", Service: manifest.Service{InternalPort: 8080}, Expose: manifest.Expose{Enabled: true, TLS: true, Domain: "sample.example.test", PathPrefix: "/"}}
	routes, _, err := updateRoute(nil, "dev", m, "dev-sample_dev-sample")
	if err != nil {
		t.Fatal(err)
	}
	if !routes[0].TLS {
		t.Fatal("TLS route setting was not preserved")
	}
}

func TestUpdateRouteRejectsDuplicateDomainAndPath(t *testing.T) {
	existing := []Route{{Environment: "dev", App: "one", Domain: "example.test", PathPrefix: "/", Service: "dev-one_dev-one", Port: 8080}}
	m := manifest.Manifest{Name: "two", Service: manifest.Service{InternalPort: 8080}, Expose: manifest.Expose{Enabled: true, Domain: "example.test", PathPrefix: "/"}}
	_, _, err := updateRoute(existing, "dev", m, "dev-two_dev-two")
	if err == nil || !strings.Contains(err.Error(), "already owned") {
		t.Fatalf("error = %v, want duplicate route error", err)
	}
}

func TestUpdateRouteRemovesDisabledExposure(t *testing.T) {
	routes, changed, err := updateRoute([]Route{{Environment: "dev", App: "sample"}}, "dev", manifest.Manifest{Name: "sample"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !changed || len(routes) != 0 {
		t.Fatalf("changed=%v routes=%v", changed, routes)
	}
}

func TestReconcileSkipsReloadForUnexposedAppWithoutRoute(t *testing.T) {
	temp := t.TempDir()
	runner := &recordingRunner{}
	service := &Service{
		logger: slog.Default(),
		config: config.Config{
			StateDir:  filepath.Join(temp, "state"),
			DataDir:   filepath.Join(temp, "data"),
			NginxName: "noops-nginx",
		},
		runner: runner,
	}

	if err := service.Reconcile(context.Background(), "dev", manifest.Manifest{Name: "postgres"}, "dev-postgres_dev-postgres"); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("unexpected nginx command: %v", runner.calls)
	}
}

func TestEnsureNetworkAddsInternalIngressAlias(t *testing.T) {
	temp := t.TempDir()
	runner := &recordingRunner{}
	service := &Service{
		logger: slog.Default(),
		config: config.Config{
			StateDir:  filepath.Join(temp, "state"),
			NginxName: "noops-nginx",
		},
		runner: runner,
	}

	if err := service.EnsureNetwork(context.Background(), "noops-prod"); err != nil {
		t.Fatal(err)
	}

	want := [][]string{{"docker", "service", "inspect", "--format", "{{json .Spec.TaskTemplate.Networks}}", "noops-nginx_nginx"}, {
		"docker", "service", "update", "--network-add",
		"name=noops-prod,alias=ingress.noops.internal", "noops-nginx_nginx",
	}}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("commands = %v, want %v", runner.calls, want)
	}
}

func TestReloadGracefullyReloadsRunningNginxContainers(t *testing.T) {
	runner := &reloadRecordingRunner{output: "nginx-one\nnginx-two\n"}
	service := &Service{
		logger: slog.Default(),
		config: config.Config{NginxName: "noops-nginx"},
		runner: runner,
	}

	if err := service.reload(context.Background()); err != nil {
		t.Fatal(err)
	}

	want := [][]string{
		{"docker", "ps", "-q", "--filter", "label=com.docker.swarm.service.name=noops-nginx_nginx"},
		{"docker", "exec", "nginx-one", "nginx", "-c", "/etc/noops/nginx/nginx.conf", "-t"},
		{"docker", "exec", "nginx-two", "nginx", "-c", "/etc/noops/nginx/nginx.conf", "-t"},
		{"docker", "exec", "nginx-one", "nginx", "-c", "/etc/noops/nginx/nginx.conf", "-s", "reload"},
		{"docker", "exec", "nginx-two", "nginx", "-c", "/etc/noops/nginx/nginx.conf", "-s", "reload"},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("reload commands = %v, want %v", runner.calls, want)
	}
}

type reloadRecordingRunner struct {
	calls       [][]string
	output      string
	invalid     bool
	failReloads int
}

func (r *reloadRecordingRunner) Run(_ context.Context, name string, args []string, _ command.RunOptions) (command.Result, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if len(args) > 0 && args[0] == "ps" {
		return command.Result{Output: []byte(r.output)}, nil
	}
	if args[len(args)-1] == "reload" && r.failReloads > 0 {
		r.failReloads--
		return command.Result{Output: []byte("reload failed")}, errors.New("exit status 1")
	}
	if r.invalid && args[len(args)-1] == "-t" {
		return command.Result{Output: []byte("invalid nginx config")}, errors.New("exit status 1")
	}
	return command.Result{}, nil
}

func TestReloadRejectsInvalidConfig(t *testing.T) {
	r := &reloadRecordingRunner{output: "nginx-one", invalid: true}
	s := &Service{logger: slog.Default(), runner: r, config: config.Config{NginxName: "noops-nginx"}}
	err := s.reload(context.Background())
	if err == nil || !strings.Contains(err.Error(), "invalid nginx config") {
		t.Fatalf("error = %v", err)
	}
	if len(r.calls) != 2 || r.calls[1][len(r.calls[1])-1] != "-t" {
		t.Fatalf("reload attempted: %v", r.calls)
	}
}

func TestValidateCloudflareRoutesRequiresImportedCertificate(t *testing.T) {
	service := &Service{config: config.Config{NginxCloudflare: true}}
	err := service.validateCloudflareRoutes([]Route{{Domain: "app.example.com", TLS: true}})
	if err == nil || !strings.Contains(err.Error(), "tls_certificate") {
		t.Fatalf("error = %v, want missing tls_certificate error", err)
	}
	if err := service.validateCloudflareRoutes([]Route{{Domain: "app.example.com", TLS: true, TLSCertificate: "cloudflare-origin"}}); err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

func TestReconcileRestoresRoutesAfterValidationFailure(t *testing.T) {
	root := t.TempDir()
	r := &reloadRecordingRunner{output: "nginx-one", invalid: true}
	s := &Service{logger: slog.Default(), runner: r, config: config.Config{StateDir: root, DataDir: root, NginxName: "nginx"}}
	old := []Route{{Environment: "prod", App: "api", Domain: "api.example.test", PathPrefix: "/", Service: "old-service", Port: 8080}}
	if err := os.MkdirAll(s.configDir(), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.ingressDir(), "nginx.conf"), []byte("previous complete configuration"), 0600); err != nil {
		t.Fatal(err)
	}
	r.invalid = false
	if err := s.writeConfig(context.Background(), old); err != nil {
		t.Fatal(err)
	}
	if err := s.writeRoutes(old); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(s.routesPath())
	if err != nil {
		t.Fatal(err)
	}
	oldConfig, err := os.ReadFile(filepath.Join(s.ingressDir(), "nginx.conf"))
	if err != nil {
		t.Fatal(err)
	}
	r.invalid = true
	m := manifest.Manifest{Name: "api", Service: manifest.Service{InternalPort: 8080}, Expose: manifest.Expose{Enabled: true, Domain: "api.example.test", PathPrefix: "/"}}
	if err := s.Reconcile(context.Background(), "prod", m, "deleted-candidate"); err == nil {
		t.Fatal("expected validation failure")
	}
	after, _ := os.ReadFile(s.routesPath())
	if string(after) != string(before) {
		t.Fatalf("route state changed: %s", after)
	}
	restored, _ := os.ReadFile(filepath.Join(s.ingressDir(), "nginx.conf"))
	if string(restored) != string(oldConfig) {
		t.Fatalf("config not restored: %s", restored)
	}
}

func TestEnsureNetworkRepairsStaleCache(t *testing.T) {
	r := &recordingRunner{}
	s := &Service{logger: slog.Default(), runner: r, config: config.Config{StateDir: t.TempDir(), NginxName: "noops-nginx"}}
	if err := s.saveNetworks(map[string]bool{"noops-prod": true}); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureNetwork(context.Background(), "noops-prod"); err != nil {
		t.Fatal(err)
	}
	if len(r.calls) != 2 || r.calls[1][2] != "update" {
		t.Fatalf("missing repair: %v", r.calls)
	}
}

func TestCandidateValidationLeavesPublishedConfigUntouched(t *testing.T) {
	root := t.TempDir()
	r := &candidateObserver{t: t, root: root}
	s := &Service{logger: slog.Default(), runner: r, config: config.Config{StateDir: root, DataDir: root, NginxName: "nginx"}}
	if err := os.MkdirAll(s.configDir(), 0700); err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(s.ingressDir(), "nginx.conf")
	if err := os.WriteFile(main, []byte("old complete configuration"), 0600); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(s.configDir(), "routes.conf")
	if err := os.WriteFile(legacy, []byte("legacy routes retained"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.configDir(), "custom.conf"), []byte("server { listen 8080; }"), 0600); err != nil {
		t.Fatal(err)
	}
	routes := []Route{{Environment: "prod", App: "api", Domain: "api.example.test", PathPrefix: "/", Service: "new-service", Port: 8080}}
	if err := s.writeConfig(context.Background(), routes); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(main)
	if err != nil {
		t.Fatal(err)
	}
	if !r.checked || !strings.Contains(string(content), "new-service") || !strings.Contains(string(content), `include "/etc/nginx/conf.d/custom.conf";`) {
		t.Fatalf("incomplete publication: %s", content)
	}
	unchanged, _ := os.ReadFile(legacy)
	if string(unchanged) != "legacy routes retained" {
		t.Fatal("legacy snippets modified")
	}
	candidates, _ := filepath.Glob(filepath.Join(s.ingressDir(), ".candidate-*"))
	if len(candidates) != 0 {
		t.Fatalf("candidate files leaked: %v", candidates)
	}
}

type candidateObserver struct {
	t       *testing.T
	root    string
	checked bool
}

func (r *candidateObserver) Run(_ context.Context, _ string, args []string, _ command.RunOptions) (command.Result, error) {
	if args[0] == "ps" {
		return command.Result{Output: []byte("nginx-one")}, nil
	}
	active, err := os.ReadFile(filepath.Join(r.root, "nginx", "nginx.conf"))
	if err != nil {
		r.t.Fatal(err)
	}
	if string(active) != "old complete configuration" {
		r.t.Fatal("configuration published before validation")
	}
	candidate, err := os.ReadFile(filepath.Join(r.root, "nginx", filepath.Base(args[len(args)-2])))
	if err != nil {
		r.t.Fatal(err)
	}
	if !strings.HasPrefix(string(candidate), "user nginx;") || !strings.HasSuffix(string(candidate), "\n}\n") || !strings.Contains(string(candidate), "new-service") {
		r.t.Fatalf("incomplete candidate: %s", candidate)
	}
	r.checked = true
	return command.Result{}, nil
}

func TestReconcileRestoresRouteAfterCertificateFailure(t *testing.T) {
	root := t.TempDir()
	r := &reloadRecordingRunner{output: "nginx-one"}
	s := &Service{logger: slog.Default(), runner: r, config: config.Config{StateDir: root, DataDir: root, NginxName: "nginx"}}
	if err := os.MkdirAll(s.configDir(), 0700); err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(s.ingressDir(), "nginx.conf")
	if err := os.WriteFile(main, []byte("original configuration"), 0600); err != nil {
		t.Fatal(err)
	}
	old := []Route{{Environment: "prod", App: "api", Domain: "api.example.test", PathPrefix: "/", Service: "old-service", Port: 8080}}
	if err := s.writeRoutes(old); err != nil {
		t.Fatal(err)
	}
	m := manifest.Manifest{Name: "api", Service: manifest.Service{InternalPort: 8080}, Expose: manifest.Expose{Enabled: true, TLS: true, Domain: "api.example.test", PathPrefix: "/"}}
	err := s.Reconcile(context.Background(), "prod", m, "candidate")
	if err == nil || !strings.Contains(err.Error(), "ACME email") {
		t.Fatalf("error=%v", err)
	}
	routes, err := s.loadRoutes()
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 || routes[0].Service != "old-service" {
		t.Fatalf("stale candidate persisted: %v", routes)
	}
	data, _ := os.ReadFile(main)
	if string(data) != "original configuration" {
		t.Fatalf("configuration not restored: %s", data)
	}
	reloads := 0
	for _, call := range r.calls {
		if call[len(call)-1] == "reload" {
			reloads++
		}
	}
	if reloads != 2 {
		t.Fatalf("expected candidate and restored reloads, got %d", reloads)
	}
}

func TestReconcileRestoresAfterReloadFailure(t *testing.T) {
	for _, failures := range []int{1, 2} {
		t.Run(fmt.Sprint(failures), func(t *testing.T) {
			root := t.TempDir()
			r := &reloadRecordingRunner{output: "nginx-one", failReloads: failures}
			s := &Service{logger: slog.Default(), runner: r, config: config.Config{StateDir: root, DataDir: root, NginxName: "nginx"}}
			if err := os.MkdirAll(s.configDir(), 0700); err != nil {
				t.Fatal(err)
			}
			main := filepath.Join(s.ingressDir(), "nginx.conf")
			if err := os.WriteFile(main, []byte("old complete configuration"), 0600); err != nil {
				t.Fatal(err)
			}
			old := []Route{{Environment: "prod", App: "api", Domain: "api.example.test", PathPrefix: "/", Service: "old-service", Port: 8080}}
			if err := s.writeRoutes(old); err != nil {
				t.Fatal(err)
			}
			m := manifest.Manifest{Name: "api", Service: manifest.Service{InternalPort: 8080}, Expose: manifest.Expose{Enabled: true, Domain: "api.example.test", PathPrefix: "/"}}
			err := s.Reconcile(context.Background(), "prod", m, "candidate-service")
			if err == nil {
				t.Fatal("expected reload error")
			}
			routes, loadErr := s.loadRoutes()
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			if len(routes) != 1 || routes[0].Service != "old-service" {
				t.Fatalf("stale routes: %v", routes)
			}
			restored, _ := os.ReadFile(main)
			if string(restored) != "old complete configuration" {
				t.Fatalf("config not restored: %s", restored)
			}
			var recovery *RecoveryError
			if errors.As(err, &recovery) != (failures == 2) {
				t.Fatalf("unexpected recovery status: %v", err)
			}
		})
	}
}
