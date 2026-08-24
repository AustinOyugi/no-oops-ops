package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitializeCreatesOnlyOwnedStore(t *testing.T) {
	root := t.TempDir()
	paths, err := Initialize(root, "test")
	if err != nil {
		t.Fatalf("initialize workspace: %v", err)
	}
	for _, path := range []string{paths.StateDir, paths.DataDir, filepath.Join(paths.Store, ConfigName), filepath.Join(root, "apps.yml")} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %q: %v", path, err)
		}
	}
	if _, err := Open(root); err != nil {
		t.Fatalf("open initialized workspace: %v", err)
	}
}
