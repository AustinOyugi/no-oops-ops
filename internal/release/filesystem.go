package release

import (
	"encoding/json"
	"fmt"
	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Store interface {
	Find(app, environment, tag string) (Metadata, error)
	Latest(cfg config.Config, name string, environment string) (ActiveRelease, error)
	SetLatest(app, environment, tag string) error
}

func saveMetadataHistory(cfg config.Config, appName string, metadata Metadata) (string, error) {
	dir := releaseHistoryMetadataDir(cfg, appName, metadata.Environment)

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create app dir %q: %w", dir, err)
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal release history metadata: %w", err)
	}

	data = append(data, '\n')

	path := releaseHistoryMetadataPath(dir, metadata.Tag)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write release metadata %q: %w", path, err)
	}

	return path, nil
}

func SetLatest(cfg config.Config, appName string, metadata ActiveRelease, environment string) error {
	dir := appDir(cfg, appName, environment)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create app dir %q: %w", dir, err)
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal release metadata: %w", err)
	}

	data = append(data, '\n')

	path := releaseMetadataPath(cfg, appName, environment)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write release metadata %q: %w", path, err)
	}

	return nil
}

func Latest(cfg config.Config, name string, environment string) (ActiveRelease, error) {

	releasesPath := releaseHistoryDir(cfg, name, environment)

	files, err := os.ReadDir(releasesPath)

	if err != nil {
		return ActiveRelease{}, fmt.Errorf("read release history dir %q: %w", releasesPath, err)
	}

	cleanedNames := make(map[string]string)

	for _, file := range files {
		if !file.IsDir() {
			name := file.Name()
			name = strings.ReplaceAll(name, ".json", "")
			name = strings.ReplaceAll(name, "-", "")
			cleanedNames[name] = file.Name()
		}
	}

	if len(cleanedNames) == 0 {
		return ActiveRelease{}, fmt.Errorf("no releases found %q: %w", releasesPath, err)
	}

	keys := make([]string, 0, len(cleanedNames))

	for key := range cleanedNames {
		keys = append(keys, key)
	}

	sort.Slice(keys, func(i, j int) bool {
		return keys[i] > keys[j]
	})

	latestTagName := keys[0]

	return ActiveRelease{strings.Replace(cleanedNames[latestTagName], ".json", "", -1)}, nil
}

func Find(cfg config.Config, name string, environment string, tag string) (Metadata, error) {

	path := releaseMetadataHistoryPath(cfg, name, environment, tag)
	data, err := os.ReadFile(path)

	if err != nil {
		return Metadata{}, fmt.Errorf("read release metadata %q: %w", path, err)
	}

	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return Metadata{}, fmt.Errorf("decode release metadata %q: %w", path, err)
	}

	return metadata, nil
}

func releaseMetadataPath(cfg config.Config, name string, environment string) string {
	return filepath.Join(appDir(cfg, name, environment), "release.json")
}

func releaseMetadataHistoryPath(cfg config.Config, name string, environment string, tag string) string {
	return filepath.Join(appDir(cfg, name, environment), "releases", tag+".json")
}

func releaseHistoryDir(cfg config.Config, name string, environment string) string {
	return filepath.Join(appDir(cfg, name, environment), "releases")
}
