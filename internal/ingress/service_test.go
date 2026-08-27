package ingress

import (
	"context"
	"log/slog"
	"path/filepath"
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
	return command.Result{}, nil
}

func TestUpdateRouteAddsExposedApp(t *testing.T) {
	m := manifest.Manifest{
		Name:    "lango",
		Service: manifest.Service{InternalPort: 8080},
		Expose:  manifest.Expose{Enabled: true, Domain: "lango.example.test", PathPrefix: "/"},
	}
	routes, changed, err := updateRoute(nil, "dev", m, "dev-lango_dev-lango")
	if err != nil {
		t.Fatal(err)
	}
	if !changed || len(routes) != 1 {
		t.Fatalf("changed=%v routes=%v", changed, routes)
	}
	if got, want := routes[0].Service, "dev-lango_dev-lango"; got != want {
		t.Errorf("service = %q, want %q", got, want)
	}
}

func TestUpdateRouteCarriesTLSSetting(t *testing.T) {
	m := manifest.Manifest{Name: "lango", Service: manifest.Service{InternalPort: 8080}, Expose: manifest.Expose{Enabled: true, TLS: true, Domain: "lango.example.test", PathPrefix: "/"}}
	routes, _, err := updateRoute(nil, "dev", m, "dev-lango_dev-lango")
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
	routes, changed, err := updateRoute([]Route{{Environment: "dev", App: "lango"}}, "dev", manifest.Manifest{Name: "lango"}, "")
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
