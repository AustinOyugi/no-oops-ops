package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/workspace"
)

func TestLoadUsesPlatformSettingsFromAppsCatalog(t *testing.T) {
	root := t.TempDir()
	if _, err := workspace.Initialize(root, Version); err != nil {
		t.Fatal(err)
	}
	content := `version: dev

settings:
  platform:
    network:
      name: noops-platform
    registry:
      name: noops-registry
      port: 5100
    ingress:
      name: noops-ingress
      http_port: 8080
      https_port: 8443
      cloudflare: true
    networks:
      default: "noops-{environment}"
      environments:
        prod: noops-production
apps: {}
`
	if err := os.WriteFile(filepath.Join(root, "apps.yml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cfg.NetworkName, "noops-platform"; got != want {
		t.Errorf("platform network = %q, want %q", got, want)
	}
	if got, want := cfg.RegistryPort, "5100"; got != want {
		t.Errorf("registry port = %q, want %q", got, want)
	}
	if !cfg.NginxCloudflare {
		t.Error("expected Cloudflare ingress support to be enabled")
	}
	if got, want := cfg.EnvironmentNetwork("prod"), "noops-production"; got != want {
		t.Errorf("prod network = %q, want %q", got, want)
	}
	if got, want := cfg.EnvironmentNetwork("dev"), "noops-dev"; got != want {
		t.Errorf("dev network = %q, want %q", got, want)
	}
}

func TestLoadRejectsDifferentAppsCatalogVersion(t *testing.T) {
	root := t.TempDir()
	if _, err := workspace.Initialize(root, Version); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "apps.yml"), []byte("version: another-version\napps: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("expected catalog version mismatch")
	}
}

func TestEnvironmentStateSelectionAndOverride(t *testing.T) {
	root := t.TempDir()
	content := "version: " + Version + "\nsettings:\n  state:\n    directory: .noops-shared\n    environments:\n      dev: .noops-dev\n      prod: .noops-prod\napps: {}\n"
	if err := os.WriteFile(filepath.Join(root, "apps.yml"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{".noops-shared", ".noops-dev", ".noops-prod", ".manual"} {
		if _, err := workspace.InitializeAt(root, directory, Version); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		options   Options
		directory string
	}{
		{Options{}, ".noops-shared"},
		{Options{Environment: "dev"}, ".noops-dev"},
		{Options{Environment: "prod"}, ".noops-prod"},
		{Options{Environment: "dev", StateDir: ".manual"}, ".manual"},
	} {
		cfg, err := LoadWithOptions(root, test.options)
		if err != nil {
			t.Fatal(err)
		}
		store := filepath.Join(root, test.directory)
		if cfg.RuntimeDir != store || cfg.StateDir != filepath.Join(store, "state") || cfg.DataDir != filepath.Join(store, "data") || cfg.ConfigPath != filepath.Join(store, "config.yml") {
			t.Fatalf("incorrect runtime boundary: %+v", cfg)
		}
	}
}

func TestMissingSelectedStoreDoesNotFallBack(t *testing.T) {
	root := t.TempDir()
	if _, err := workspace.Initialize(root, Version); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "apps.yml"), []byte("version: "+Version+"\nsettings:\n  state:\n    environments:\n      prod: .noops-prod\napps: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadWithOptions(root, Options{Environment: "prod"}); err == nil {
		t.Fatal("selected missing store fell back to initialized default")
	}
}

func TestStateDirectoryTemplateRequiresSafeEnvironment(t *testing.T) {
	root := t.TempDir()
	if _, err := StateDirectory(root, Options{StateDir: ".noops-{environment}"}); err == nil {
		t.Fatal("template accepted without environment")
	}
	if _, err := StateDirectory(root, Options{StateDir: ".noops-{environment}", Environment: "../prod"}); err == nil {
		t.Fatal("environment escaped directory template")
	}
	got, err := StateDirectory(root, Options{StateDir: ".noops-{environment}", Environment: "dev"})
	if err != nil || got != ".noops-dev" {
		t.Fatalf("template: %q %v", got, err)
	}
}
