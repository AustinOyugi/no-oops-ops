package deploy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOptionalEnvFileAllowsAnEmptyPath(t *testing.T) {
	envFile, err := LoadOptionalEnvFile("")
	if err != nil {
		t.Fatalf("LoadOptionalEnvFile returned error: %v", err)
	}
	if len(envFile.Sections) != 0 {
		t.Errorf("sections = %#v, want no sections", envFile.Sections)
	}
}

func TestLoadEnvFileSelectsConfigKeys(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "db.yml"), []byte("sections:\n- items:\n  - {key: HOST, value: database}\n  - {key: EXTRA, value: unused}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "env.yml")
	if err := os.WriteFile(path, []byte("sections:\n- items:\n  - {key: HOST, from_config: db.yml}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := LoadEnvFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := ResolveEnvFile(file, "prod", nil)
	if len(got.Values) != 1 || got.Values["HOST"] != "database" {
		t.Fatalf("selected values: %#v", got.Values)
	}
}
