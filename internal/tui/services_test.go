package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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
	if len(rows) != 4 {
		t.Fatalf("rows = %+v", rows)
	}
	states := map[string]string{}
	for _, r := range rows {
		states[r.ID] = r.State
		if r.ID == "4" && (!r.Untracked || r.App != "untracked") {
			t.Fatal("unmatched service not labeled")
		}
	}
	if states["1"] != "running" || states["2"] != "degraded" || states["3"] != "scaled down" {
		t.Fatalf("states = %v", states)
	}
	if _, err := parseServices("not json", owners); err == nil {
		t.Fatal("malformed Docker output accepted")
	}
}

func TestServiceAgeUsesCreationRatherThanTaskStart(t *testing.T) {
	rows := []Row{{Service: "api"}, {Service: "unknown"}}
	if err := applyServiceAges(rows, `{"Name":"api","CreatedAt":"2026-10-01T00:00:00Z"}`); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 2, 4, 0, 0, 0, time.UTC)
	if got := serviceAge(rows[0], now); got != "1d 4h" {
		t.Fatalf("age %s", got)
	}
	if got := serviceAge(rows[1], now); got != "—" {
		t.Fatalf("unknown age %s", got)
	}
	if err := applyServiceAges(rows, `{"Name":"api","CreatedAt":"invalid"}`); err == nil {
		t.Fatal("invalid timestamp accepted")
	}
}

func TestHistoryFindsServiceWithoutStackManifest(t *testing.T) {
	cfg := config.Config{StateDir: t.TempDir()}
	dir := filepath.Join(cfg.StateDir, "apps", "nyota-web", "prod", "deployments")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "old.json"), []byte(`{"stack_name":"prod-nyota-web-old","service_name":"prod-nyota-web-old_app"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "corrupt.json"), []byte(`broken`), 0600); err != nil {
		t.Fatal(err)
	}
	owners, err := managedServices(cfg)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := parseServices(`{"ID":"web","Name":"prod-nyota-web-old_app","Replicas":"1/1"}`, owners)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Untracked || rows[0].App != "nyota-web" || rows[0].Environment != "prod" {
		t.Fatalf("missing history ownership: %+v", rows)
	}
}
