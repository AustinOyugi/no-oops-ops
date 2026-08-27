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

	values, err := buildEnvironmentValues(manifestPath, manifest.Manifest{Env: manifest.Env{File: "env.yml"}}, "prod")
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
