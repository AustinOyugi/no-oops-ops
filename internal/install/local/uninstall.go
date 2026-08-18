package local

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"github.com/AustinOyugi/no-oops-ops/internal/uninstall"
)

func (h *Host) LoadInstallation(ctx context.Context) (uninstall.Metadata, error) {
	m, err := h.readInstallMetadata(ctx)
	if err != nil {
		return uninstall.Metadata{}, err
	}

	stateDir := m.StateDir
	if stateDir == "" {
		stateDir = h.stateDir
	}
	dataDir := m.DataDir
	if dataDir == "" {
		dataDir = h.dataDir
	}

	return uninstall.Metadata{
		StateDir: stateDir,
		DataDir:  dataDir,
		Network:  uninstall.Network{Name: m.Network.Name},
		Registry: uninstall.Registry{Name: m.Registry.Name},
	}, nil
}

func (h *Host) RemoveApps(ctx context.Context, m uninstall.Metadata) error {
	appsDir := filepath.Join(m.StateDir, "apps")
	apps, err := os.ReadDir(appsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read managed apps %q: %w", appsDir, err)
	}

	for _, app := range apps {
		if !app.IsDir() {
			continue
		}
		environments, err := os.ReadDir(filepath.Join(appsDir, app.Name()))
		if err != nil {
			return fmt.Errorf("read environments for app %q: %w", app.Name(), err)
		}
		for _, environment := range environments {
			if !environment.IsDir() {
				continue
			}
			stackName := environment.Name() + "-" + app.Name()
			if err := h.removeStack(ctx, stackName); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *Host) RemoveRegistry(ctx context.Context, m uninstall.Metadata) error {
	if m.Registry.Name == "" {
		return nil
	}
	return h.removeStack(ctx, m.Registry.Name)
}

func (h *Host) RemoveNetwork(ctx context.Context, m uninstall.Metadata) error {
	if m.Network.Name == "" {
		return nil
	}
	result, err := h.runner.Run(ctx, "docker", []string{"network", "rm", m.Network.Name}, command.RunOptions{LogCommand: true})
	if err != nil && !alreadyRemoved(err, string(result.Output)) {
		return fmt.Errorf("remove network %q: %w", m.Network.Name, err)
	}
	return nil
}

func (h *Host) RemoveGeneratedState(ctx context.Context, m uninstall.Metadata) error {
	_ = ctx
	if err := safePath(m.StateDir); err != nil {
		return fmt.Errorf("validate state directory: %w", err)
	}
	entries, err := os.ReadDir(m.StateDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read state directory %q: %w", m.StateDir, err)
	}
	for _, entry := range entries {
		if entry.Name() == "install.json" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(m.StateDir, entry.Name())); err != nil {
			return fmt.Errorf("remove generated state %q: %w", entry.Name(), err)
		}
	}
	return nil
}

func (h *Host) RemoveData(ctx context.Context, m uninstall.Metadata) error {
	_ = ctx
	if err := safeRemoveAll(m.DataDir); err != nil {
		return fmt.Errorf("remove persistent data %q: %w", m.DataDir, err)
	}
	return nil
}

func (h *Host) RemoveInstallMetadata(ctx context.Context, m uninstall.Metadata) error {
	_ = ctx
	path := filepath.Join(m.StateDir, "install.json")
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove install metadata %q: %w", path, err)
	}
	if err := os.Remove(m.StateDir); err != nil && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, syscall.ENOTEMPTY) {
		return fmt.Errorf("remove empty state directory %q: %w", m.StateDir, err)
	}
	return nil
}

func (h *Host) removeStack(ctx context.Context, name string) error {
	result, err := h.runner.Run(ctx, "docker", []string{"stack", "rm", name}, command.RunOptions{LogCommand: true})
	if err != nil && !alreadyRemoved(err, string(result.Output)) {
		return fmt.Errorf("remove stack %q: %w", name, err)
	}
	return nil
}

func alreadyRemoved(err error, output string) bool {
	message := strings.ToLower(err.Error() + " " + output)
	return strings.Contains(message, "not found") || strings.Contains(message, "not exist") || strings.Contains(message, "no such")
}

func safeRemoveAll(path string) error {
	if err := safePath(path); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Clean(path))
}

func safePath(path string) error {
	clean := filepath.Clean(path)
	if path == "" || clean == "." || clean == string(filepath.Separator) {
		return fmt.Errorf("refusing to remove unsafe path %q", path)
	}
	return nil
}
