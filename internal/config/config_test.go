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
      name: cranium-platform
    registry:
      name: cranium-registry
      port: 5100
    ingress:
      name: cranium-ingress
      http_port: 8080
      https_port: 8443
      cloudflare: true
    networks:
      default: "cranium-{environment}"
      environments:
        prod: cranium-production
apps: {}
`
	if err := os.WriteFile(filepath.Join(root, "apps.yml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cfg.NetworkName, "cranium-platform"; got != want {
		t.Errorf("platform network = %q, want %q", got, want)
	}
	if got, want := cfg.RegistryPort, "5100"; got != want {
		t.Errorf("registry port = %q, want %q", got, want)
	}
	if !cfg.NginxCloudflare {
		t.Error("expected Cloudflare ingress support to be enabled")
	}
	if got, want := cfg.EnvironmentNetwork("prod"), "cranium-production"; got != want {
		t.Errorf("prod network = %q, want %q", got, want)
	}
	if got, want := cfg.EnvironmentNetwork("dev"), "cranium-dev"; got != want {
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
