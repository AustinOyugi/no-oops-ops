package release

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

// materializeBuildEnvironment writes ordinary resolved values to the requested
// build-context file. The generated file and the .dockerignore override are
// restored after the build, so neither reaches the application's repository.
func materializeBuildEnvironment(contextDir string, settings *manifest.EnvBuild, values map[string]string) (func() error, error) {
	if settings == nil {
		return func() error { return nil }, nil
	}

	target, relative, err := buildEnvironmentPath(contextDir, settings.File)
	if err != nil {
		return nil, err
	}
	targetState, err := captureFile(target)
	if err != nil {
		return nil, err
	}
	ignorePath := filepath.Join(contextDir, ".dockerignore")
	ignoreState, err := captureFile(ignorePath)
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(target, renderDotenv(values), 0o600); err != nil {
		return nil, fmt.Errorf("write build environment file %q: %w", target, err)
	}
	ignore := append(ignoreState.data, []byte("\n!"+filepath.ToSlash(relative)+"\n")...)
	if err := os.WriteFile(ignorePath, ignore, 0o600); err != nil {
		_ = restoreFile(target, targetState)
		return nil, fmt.Errorf("update build ignore file %q: %w", ignorePath, err)
	}

	return func() error {
		if err := restoreFile(target, targetState); err != nil {
			return err
		}
		return restoreFile(ignorePath, ignoreState)
	}, nil
}

type savedFile struct {
	exists bool
	data   []byte
	mode   os.FileMode
}

func captureFile(path string) (savedFile, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return savedFile{}, nil
	}
	if err != nil {
		return savedFile{}, fmt.Errorf("inspect %q: %w", path, err)
	}
	if info.IsDir() {
		return savedFile{}, fmt.Errorf("build environment path %q is a directory", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return savedFile{}, fmt.Errorf("read %q: %w", path, err)
	}
	return savedFile{exists: true, data: data, mode: info.Mode().Perm()}, nil
}

func restoreFile(path string, saved savedFile) error {
	if !saved.exists {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove generated build file %q: %w", path, err)
		}
		return nil
	}
	if err := os.WriteFile(path, saved.data, saved.mode); err != nil {
		return fmt.Errorf("restore build file %q: %w", path, err)
	}
	return nil
}

func buildEnvironmentPath(contextDir, name string) (string, string, error) {
	if filepath.IsAbs(name) {
		return "", "", fmt.Errorf("env.build.file must be relative to the build context")
	}
	relative := filepath.Clean(name)
	if relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("env.build.file %q escapes the build context", name)
	}
	return filepath.Join(contextDir, relative), relative, nil
}

func renderDotenv(values map[string]string) []byte {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var out strings.Builder
	for _, key := range keys {
		out.WriteString(key)
		out.WriteByte('=')
		out.WriteString(strconv.Quote(values[key]))
		out.WriteByte('\n')
	}
	return []byte(out.String())
}
