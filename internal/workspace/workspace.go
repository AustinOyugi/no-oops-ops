// Package workspace manages the on-disk boundary owned by No Oops.
package workspace

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AustinOyugi/no-oops-ops/internal/state"
	"github.com/AustinOyugi/no-oops-ops/internal/templateutil"
)

const (
	DirName    = ".noops"
	ConfigName = "config.yml"
)

//go:embed templates/apps.yml.tmpl
var initialAppsCatalog string

// Paths identifies the only runtime locations No Oops may write to.
type Paths struct {
	Root     string
	Store    string
	StateDir string
	DataDir  string
}

// Initialize creates the No Oops-owned store below root. It seeds apps.yml
// when absent but never replaces an existing application catalog.
func Initialize(root, noopsVersion string) (Paths, error) {
	return InitializeAt(root, "", noopsVersion)
}

// InitializeAt keeps source manifests in root and runtime files in store.
// Relative store paths are resolved against root; an empty store uses .noops.
func InitializeAt(root, store, noopsVersion string) (Paths, error) {
	paths, err := resolveAt(root, store)
	if err != nil {
		return Paths{}, err
	}
	for _, path := range []string{paths.StateDir, paths.DataDir} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return Paths{}, fmt.Errorf("create workspace directory %q: %w", path, err)
		}
	}
	configPath := filepath.Join(paths.Store, ConfigName)
	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		content, err := RenderConfig("")
		if err != nil {
			return Paths{}, err
		}
		if err := state.WriteFile(configPath, content, 0o600); err != nil {
			return Paths{}, fmt.Errorf("write workspace config %q: %w", configPath, err)
		}
	} else if err != nil {
		return Paths{}, fmt.Errorf("inspect workspace config %q: %w", configPath, err)
	}
	appsPath := filepath.Join(paths.Root, "apps.yml")
	if _, err := os.Stat(appsPath); errors.Is(err, os.ErrNotExist) {
		content, err := templateutil.Render("apps.yml.tmpl", initialAppsCatalog, struct{ Version string }{noopsVersion})
		if err != nil {
			return Paths{}, err
		}
		if err := state.WriteFile(appsPath, content, 0o600); err != nil {
			return Paths{}, fmt.Errorf("write app catalog %q: %w", appsPath, err)
		}
	} else if err != nil {
		return Paths{}, fmt.Errorf("inspect app catalog %q: %w", appsPath, err)
	}
	return paths, nil
}

// Open validates a previously initialized workspace.
func Open(root string) (Paths, error) {
	return OpenAt(root, "")
}

func OpenAt(root, store string) (Paths, error) {
	paths, err := resolveAt(root, store)
	if err != nil {
		return Paths{}, err
	}
	configPath := filepath.Join(paths.Store, ConfigName)
	if _, err := os.Stat(configPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Paths{}, fmt.Errorf("%q is not initialized; run noops --state-dir %q init %q", paths.Store, paths.Store, paths.Root)
		}
		return Paths{}, fmt.Errorf("inspect workspace config %q: %w", configPath, err)
	}
	return paths, nil
}

func resolve(root string) (Paths, error) {
	return resolveAt(root, "")
}

func resolveAt(root, store string) (Paths, error) {
	if root == "" {
		return Paths{}, errors.New("workspace is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return Paths{}, fmt.Errorf("resolve workspace %q: %w", root, err)
	}
	abs = filepath.Clean(abs)
	info, err := os.Stat(abs)
	if err != nil {
		return Paths{}, fmt.Errorf("inspect workspace %q: %w", abs, err)
	}
	if !info.IsDir() {
		return Paths{}, fmt.Errorf("workspace %q is not a directory", abs)
	}
	if store == "" {
		store = DirName
	}
	if !filepath.IsAbs(store) {
		store = filepath.Join(abs, store)
	}
	store = filepath.Clean(store)
	return Paths{Root: abs, Store: store, StateDir: filepath.Join(store, "state"), DataDir: filepath.Join(store, "data")}, nil
}

//go:embed templates/config.yml.tmpl
var workspaceConfigTemplate string

// RenderConfig generates the workspace settings file without changing its schema.
func RenderConfig(email string) ([]byte, error) {
	return templateutil.Render("config.yml.tmpl", workspaceConfigTemplate, struct{ ACMEEmail string }{email})
}
