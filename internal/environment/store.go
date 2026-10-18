package environment

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/AustinOyugi/no-oops-ops/internal/state"
	"gopkg.in/yaml.v3"
)

var configNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// Install copies a reusable config into the workspace without replacing one.
func Install(ctx context.Context, root, name, source string) (string, error) {
	return storeConfig(ctx, root, name, source, false)
}

// Upgrade atomically replaces an installed reusable config.
func Upgrade(ctx context.Context, root, name, source string) (string, error) {
	return storeConfig(ctx, root, name, source, true)
}

func storeConfig(ctx context.Context, root, name, source string, upgrade bool) (string, error) {
	target, err := installedConfigPath(root, name)
	if err != nil {
		return "", err
	}
	unlock, err := state.AcquireLock(ctx, target+".lock")
	if err != nil {
		return "", err
	}
	defer unlock()
	_, err = os.Stat(target)
	if !upgrade && err == nil {
		return "", fmt.Errorf("config %q already installed; use config upgrade", name)
	}
	if upgrade && os.IsNotExist(err) {
		return "", fmt.Errorf("config %q is not installed; use config install", name)
	}
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return "", fmt.Errorf("read config %q: %w", source, err)
	}
	var file File
	if err := yaml.Unmarshal(data, &file); err != nil {
		return "", fmt.Errorf("decode config %q: %w", source, err)
	}
	if err := state.WriteFile(target, data, 0600); err != nil {
		return "", err
	}
	return target, nil
}

func installedConfigPath(root, alias string) (string, error) {
	if !configNamePattern.MatchString(alias) {
		return "", fmt.Errorf("config alias must contain only letters, digits, underscores and hyphens")
	}
	if root == "" {
		return "", fmt.Errorf("workspace is required to resolve config alias %q", alias)
	}
	path, err := filepath.Abs(filepath.Join(root, "configs", alias+".yml"))
	if err != nil {
		return "", err
	}
	return path, nil
}
