package release

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

func TestBuildEnvironmentValuesIncludesOnlyOrdinaryValues(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "app.yml")
	envPath := filepath.Join(dir, "env.yml")
	if err := os.WriteFile(envPath, []byte("sections:\n  - name: app\n    items:\n      - key: NEXT_PUBLIC_APP_URI\n        values:\n          prod: https://example.com\n      - key: API_TOKEN\n        from_secret: API_TOKEN\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	values, err := buildEnvironmentValues(manifestPath, manifest.Manifest{Env: manifest.Env{File: "env.yml"}}, "prod", dir)
	if err != nil {
		t.Fatalf("buildEnvironmentValues() error = %v", err)
	}
	if got, want := values["NEXT_PUBLIC_APP_URI"], "https://example.com"; got != want {
		t.Fatalf("NEXT_PUBLIC_APP_URI = %q, want %q", got, want)
	}
	if _, ok := values["API_TOKEN"]; ok {
		t.Fatal("managed secret was included in build arguments")
	}
}

func TestBuildEnvironmentValuesSharedConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "db.yml"), []byte("sections:\n- items:\n  - {key: HOST, value: database}\n  - {key: TOKEN, from_secret: token}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "env.yml"), []byte("sections:\n- items:\n  - {key: HOST, from_config: db.yml}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	values, err := buildEnvironmentValues(filepath.Join(dir, "app.yml"), manifest.Manifest{Env: manifest.Env{File: "env.yml"}}, "prod", dir)
	if err != nil {
		t.Fatal(err)
	}
	if values["HOST"] != "database" || len(values) != 1 {
		t.Fatalf("unexpected build values: %#v", values)
	}
}

func TestBuildEnvironmentUsesInstalledAliasOutsideManifestTree(t *testing.T) {
	root, dir := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "configs"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "configs/database.yml"), []byte("sections:\n- items:\n  - {key: HOST, value: installed}\n  - {key: UNUSED, value: extra}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "env.yml"), []byte("sections:\n- items:\n  - {key: HOST, from_config: database}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	values, err := buildEnvironmentValues(filepath.Join(dir, "app.yml"), manifest.Manifest{Env: manifest.Env{File: "env.yml"}}, "prod", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values["HOST"] != "installed" {
		t.Fatalf("build values: %#v", values)
	}
}
