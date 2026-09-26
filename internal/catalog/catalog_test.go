package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolve(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "apps", "api"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "apps.yml"), []byte("apps:\n  api:\n    manifest: ./apps/api/app.yml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "apps", "api", "app.yml")
	if err := os.WriteFile(want, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Resolve(root, "api")
	if err != nil {
		t.Fatalf("resolve app: %v", err)
	}
	if got != want {
		t.Errorf("manifest = %q, want %q", got, want)
	}
}

func TestLoadUpgradeRepository(t *testing.T) {
	root := t.TempDir()
	data := []byte("settings:\n  upgrade:\n    repository: example/noops-fork\napps: {}\n")
	if err := os.WriteFile(filepath.Join(root, "apps.yml"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := file.Settings.Upgrade.Repository; got != "example/noops-fork" {
		t.Errorf("upgrade repository = %q", got)
	}
}
