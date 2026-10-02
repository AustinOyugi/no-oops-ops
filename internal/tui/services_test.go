package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
)

func TestServiceOwnershipAndStates(t *testing.T) {
	cfg := config.Config{StateDir: t.TempDir(), RegistryName: "custom-registry", NginxName: "custom-ingress"}
	dir := filepath.Join(cfg.StateDir, "apps", "shop-api", "prod")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"stack.yml", "stack-prod-shop-api-v42.yml"} {
		service := "prod-shop-api"
		if name != "stack.yml" {
			service = "app"
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte("services:\n  "+service+":\n    image: example\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	owners, err := managedServices(cfg)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := parseServices(`{"ID":"1","Name":"prod-shop-api_prod-shop-api","Replicas":"2/2"}
{"ID":"2","Name":"prod-shop-api-v42_app","Replicas":"0/1"}
{"ID":"3","Name":"custom-registry_registry","Replicas":"0/0"}
{"ID":"4","Name":"unrelated_api","Replicas":"1/1"}`, owners)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %+v", rows)
	}
	states := map[string]string{}
	for _, r := range rows {
		states[r.ID] = r.State
	}
	if states["1"] != "running" || states["2"] != "degraded" || states["3"] != "scaled down" {
		t.Fatalf("states = %v", states)
	}
	if _, err := parseServices("not json", owners); err == nil {
		t.Fatal("malformed Docker output accepted")
	}
}
