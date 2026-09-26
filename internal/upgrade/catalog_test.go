package upgrade

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateCatalogVersionPreservesAndCanRollback(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "apps.yml")
	original := "version: v1.0.0\n\n# retain this comment\napps: {}\n"
	if err := os.WriteFile(path, []byte(original), 0o640); err != nil {
		t.Fatal(err)
	}
	rollback, err := UpdateCatalogVersion(root, "1.1.0")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "version: 1.1.0") || !strings.Contains(string(updated), "# retain this comment") {
		t.Errorf("updated catalog = %q", updated)
	}
	if err := rollback(); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != original {
		t.Errorf("restored catalog = %q, want %q", restored, original)
	}
}
