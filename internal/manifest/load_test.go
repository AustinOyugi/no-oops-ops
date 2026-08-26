package manifest

import (
	"os"
	"path/filepath"
	"strings"
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

func TestLoadServiceSelectsOneServiceFromMultiServiceManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.yml")
	data := []byte("services:\n  worker:\n    image: repo/worker:1\n    healthcheck: {test: [CMD, 'true']}\n    x-noops: {service: {internal_port: 8081}}\n  api:\n    image: repo/api:1\n    healthcheck: {test: [CMD, 'true']}\n    x-noops: {service: {internal_port: 8080}}\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	names, err := Services(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(names, ","), "api,worker"; got != want {
		t.Fatalf("Services = %q, want %q", got, want)
	}
	m, err := LoadService(path, "worker")
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "worker" || m.Service.InternalPort != 8081 {
		t.Fatalf("unexpected selected manifest: %#v", m)
	}
}

func TestLoadSupportsCompactIngressMetadataAndWorkerWithoutHealthcheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.yml")
	data := []byte("services:\n  api:\n    image: repo/api:1\n    x-noops:\n      service: {internal_port: 8080}\n      ingress: {domains: [api.example.test]}\n  worker:\n    image: repo/worker:1\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	api, err := LoadService(path, "api")
	if err != nil {
		t.Fatal(err)
	}
	if !api.Expose.Enabled || api.Expose.Domain != "api.example.test" {
		t.Fatalf("compact ingress was not normalized: %#v", api.Expose)
	}
	if _, err := LoadService(path, "worker"); err != nil {
		t.Fatalf("worker without ingress or healthcheck should load: %v", err)
	}
}

func TestLoadBuildGitSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.yml")
	data := []byte(`services:
  api:
    image: example/api
    build: {context: ., dockerfile: Dockerfile}
    x-noops:
      build:
        source:
          git:
            url: https://github.com/example/api.git
            environments:
              prod: {ref: refs/tags/v1.2.3, credential: github-readonly}
        resources: {cpus: "1.5", memory: 2Gi}
        timeout: 20m
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Build.Source.Git == nil || m.Build.Source.Git.Environments["prod"].Ref != "refs/tags/v1.2.3" {
		t.Fatalf("Git build source was not mapped: %#v", m.Build)
	}
}

func TestLoadRejectsHostPrebuildCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.yml")
	data := []byte("services:\n  api:\n    image: example/api\n    build: {context: ., dockerfile: Dockerfile}\n    x-noops:\n      source:\n        build:\n          command: [mvn, package]\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected host pre-build command rejection, got %v", err)
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
	for _, example := range []struct {
		path    string
		service string
	}{
		{"../../examples/paas/apps/postgres/postgres.app.yml", "postgres"},
		{"../../examples/paas/apps/postgres/postgres.app.yml", "pgbouncer"},
		{"../../examples/paas/apps/keycloak/keycloak.app.yml", "keycloak"},
		{"../../examples/paas/apps/service/lango.app.yml", "lango"},
	} {
		if _, err := LoadService(example.path, example.service); err != nil {
			t.Errorf("LoadService(%q, %q): %v", example.path, example.service, err)
		}
	}
}
