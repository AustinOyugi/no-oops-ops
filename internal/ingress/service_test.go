package ingress

import (
	"strings"
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

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
