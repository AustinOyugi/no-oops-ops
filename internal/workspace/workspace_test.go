package workspace

import (
	"os"
	"path/filepath"
	"strings"
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
	catalog, err := os.ReadFile(filepath.Join(root, "apps.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(catalog), "repository: AustinOyugi/no-oops-ops") {
		t.Errorf("initial apps.yml is missing the default upgrade repository:\n%s", catalog)
	}
}

func TestIndependentStoresShareOnlyCatalog(t *testing.T) {
	root := t.TempDir()
	dev, err := InitializeAt(root, ".noops-dev", "test")
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, "apps.yml"))
	if err != nil {
		t.Fatal(err)
	}
	prod, err := InitializeAt(root, filepath.Join(t.TempDir(), ".noops"), "another-version")
	if err != nil {
		t.Fatal(err)
	}
	if dev.StateDir == prod.StateDir || dev.DataDir == prod.DataDir {
		t.Fatal("runtime stores overlap")
	}
	if err := os.WriteFile(filepath.Join(dev.StateDir, "install.json"), []byte("dev only"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(prod.StateDir, "install.json")); !os.IsNotExist(err) {
		t.Fatalf("prod sees dev state: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(root, "apps.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("second initialization replaced shared catalog")
	}
	if _, err := os.Stat(filepath.Join(root, DirName)); !os.IsNotExist(err) {
		t.Fatalf("default store unexpectedly created: %v", err)
	}
	for _, paths := range []Paths{dev, prod} {
		opened, err := OpenAt(root, paths.Store)
		if err != nil || opened != paths {
			t.Fatalf("reopen: %+v %v", opened, err)
		}
	}
}
