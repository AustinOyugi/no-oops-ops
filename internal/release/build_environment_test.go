package release

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

func TestMaterializeBuildEnvironmentWritesAndRestoresFiles(t *testing.T) {
	dir := t.TempDir()
	ignorePath := filepath.Join(dir, ".dockerignore")
	if err := os.WriteFile(ignorePath, []byte(".env*\nnode_modules\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cleanup, err := materializeBuildEnvironment(dir, &manifest.EnvBuild{File: ".env.production"}, map[string]string{"NAME": "Vybes Africa", "URL": "https://example.com"})
	if err != nil {
		t.Fatalf("materializeBuildEnvironment() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".env.production"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "NAME=\"Vybes Africa\"\nURL=\"https://example.com\"\n"; got != want {
		t.Fatalf("generated dotenv = %q, want %q", got, want)
	}
	ignore, err := os.ReadFile(ignorePath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(ignore), ".env*\nnode_modules\n\n!.env.production\n"; got != want {
		t.Fatalf("generated .dockerignore = %q, want %q", got, want)
	}

	if err := cleanup(); err != nil {
		t.Fatalf("cleanup() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".env.production")); !os.IsNotExist(err) {
		t.Fatalf("generated environment file still exists: %v", err)
	}
	ignore, err = os.ReadFile(ignorePath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(ignore), ".env*\nnode_modules\n"; got != want {
		t.Fatalf("restored .dockerignore = %q, want %q", got, want)
	}
}

func TestMaterializeBuildEnvironmentSkipsEmptyFile(t *testing.T) {
	cleanup, err := materializeBuildEnvironment(t.TempDir(), &manifest.EnvBuild{Secrets: []string{"SENTRY_AUTH_TOKEN"}}, nil)
	if err != nil {
		t.Fatalf("materializeBuildEnvironment() error = %v", err)
	}
	if err := cleanup(); err != nil {
		t.Fatalf("cleanup() error = %v", err)
	}
}
