package upgrade

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"syscall"
)

var catalogVersionPattern = regexp.MustCompile(`(?m)^version:[^\r\n]*$`)

// UpdateCatalogVersion adopts target for the current workspace without
// reformatting the rest of apps.yml. It returns a rollback function for use
// when installing the executable fails.
func UpdateCatalogVersion(workspace, target string) (func() error, error) {
	if err := ValidateVersion(target); err != nil {
		return nil, err
	}
	path := filepath.Join(workspace, "apps.yml")
	original, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read app catalog %q: %w", path, err)
	}
	if !catalogVersionPattern.Match(original) {
		return nil, fmt.Errorf("app catalog %q has no top-level version", path)
	}
	updated := catalogVersionPattern.ReplaceAll(original, []byte("version: "+target))
	if err := writeAtomic(path, updated); err != nil {
		return nil, err
	}
	return func() error { return writeAtomic(path, original) }, nil
}

func writeAtomic(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect %q: %w", path, err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".apps.yml-*")
	if err != nil {
		return fmt.Errorf("prepare app catalog update: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(info.Mode().Perm()); err != nil {
		temporary.Close()
		return fmt.Errorf("preserve app catalog permissions: %w", err)
	}
	if os.Geteuid() == 0 {
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			if err := temporary.Chown(int(stat.Uid), int(stat.Gid)); err != nil {
				temporary.Close()
				return fmt.Errorf("preserve app catalog ownership: %w", err)
			}
		}
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("write app catalog update: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync app catalog update: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close app catalog update: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace app catalog %q: %w", path, err)
	}
	return nil
}
