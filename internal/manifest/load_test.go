package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadComposeShapedSingleService(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.yml")
	data := []byte(`services:
  api:
    image: registry.example.test/team/api:1.2.3
    command: ["serve"]
    healthcheck:
      test: ["CMD", "true"]
    deploy:
      replicas: 2
    networks: [platform]
    volumes: ["data:/srv/data"]
    x-noops:
      service:
        internal_port: 8080
      env:
        file: api.env.yml
      expose:
        enabled: true
        domain: api.example.test
        path_prefix: /
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "api" || m.Image.Repository != "registry.example.test/team/api" || m.Image.Tag != "1.2.3" {
		t.Fatalf("unexpected manifest: %#v", m)
	}
	if m.Image.ShouldBuild() || m.Service.InternalPort != 8080 || m.Service.Replicas != 2 || m.Service.Network != "platform" {
		t.Fatalf("Compose fields were not mapped: %#v", m)
	}
	if len(m.Service.Command) != 1 || m.Service.Command[0] != "serve" || len(m.Volumes) != 1 {
		t.Fatalf("service passthrough was not mapped: %#v", m)
	}
}

func TestLoadComposeShapedManifestRejectsMultipleServices(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.yml")
	if err := os.WriteFile(path, []byte("services:\n  one: {image: repo:one}\n  two: {image: repo:two}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected multi-service Compose manifest to be rejected")
	}
}

func TestLoadRejectsLegacyManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.yml")
	if err := os.WriteFile(path, []byte("name: legacy\nimage: {repository: example/app}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected legacy manifest to be rejected")
	}
}

func TestLoadComposeShapedExamples(t *testing.T) {
	for _, path := range []string{
		"../../examples/apps/postgres/postgres.app.yml",
		"../../examples/apps/keycloak/keycloak.app.yml",
		"../../examples/apps/service/lango.app.yml",
	} {
		if _, err := Load(path); err != nil {
			t.Errorf("Load(%q): %v", path, err)
		}
	}
}
