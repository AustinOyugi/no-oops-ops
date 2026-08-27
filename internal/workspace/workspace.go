// Package workspace manages the on-disk boundary owned by No Oops.
package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	DirName    = ".noops"
	ConfigName = "config.yml"
)

const initialAppsCatalog = `version: %s

settings:
  platform:
    network:
      name: noops-platform
    registry:
      name: noops-registry
      port: 5000
    ingress:
      name: noops-nginx
      http_port: 80
      https_port: 443
      # Enable only when every public ingress hostname is Cloudflare-proxied.
      cloudflare: false
    networks:
      default: "noops-{environment}"
      environments: {}

apps: {}
`

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
	paths, err := resolve(root)
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
		if err := os.WriteFile(configPath, []byte("version: 1\n"), 0o600); err != nil {
			return Paths{}, fmt.Errorf("write workspace config %q: %w", configPath, err)
		}
	} else if err != nil {
		return Paths{}, fmt.Errorf("inspect workspace config %q: %w", configPath, err)
	}
	appsPath := filepath.Join(paths.Root, "apps.yml")
	if _, err := os.Stat(appsPath); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(appsPath, []byte(fmt.Sprintf(initialAppsCatalog, noopsVersion)), 0o600); err != nil {
			return Paths{}, fmt.Errorf("write app catalog %q: %w", appsPath, err)
		}
	} else if err != nil {
		return Paths{}, fmt.Errorf("inspect app catalog %q: %w", appsPath, err)
	}
	return paths, nil
}

// Open validates a previously initialized workspace.
func Open(root string) (Paths, error) {
	paths, err := resolve(root)
	if err != nil {
		return Paths{}, err
	}
	configPath := filepath.Join(paths.Store, ConfigName)
	if _, err := os.Stat(configPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Paths{}, fmt.Errorf("%q is not initialized; run noops init %q", paths.Root, paths.Root)
		}
		return Paths{}, fmt.Errorf("inspect workspace config %q: %w", configPath, err)
	}
	return paths, nil
}

func resolve(root string) (Paths, error) {
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
	store := filepath.Join(abs, DirName)
	return Paths{Root: abs, Store: store, StateDir: filepath.Join(store, "state"), DataDir: filepath.Join(store, "data")}, nil
}
