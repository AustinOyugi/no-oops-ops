package release

import (
	"encoding/json"
	"fmt"
	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"os"
	"path/filepath"
	"sort"

	"github.com/AustinOyugi/no-oops-ops/internal/state"
)

type fileSystemStore struct{}

func NewFilesystemStore() Store {
	return fileSystemStore{}
}

// ListHistory returns all release records for an app environment. Missing
// history is treated as empty so cleanup can safely be retried.
func ListHistory(cfg config.Config, appName, environment string) ([]Metadata, error) {
	dir := releaseHistoryDir(cfg, appName, environment)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []Metadata{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read release history %q: %w", dir, err)
	}

	metadata := make([]Metadata, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read release metadata %q: %w", path, err)
		}
		var item Metadata
		if err := json.Unmarshal(data, &item); err != nil {
			return nil, fmt.Errorf("decode release metadata %q: %w", path, err)
		}
		metadata = append(metadata, item)
	}
	sort.Slice(metadata, func(i, j int) bool {
		if metadata[i].CreateAt.Equal(metadata[j].CreateAt) {
			return metadata[i].Tag > metadata[j].Tag
		}
		return metadata[i].CreateAt.After(metadata[j].CreateAt)
	})

	return metadata, nil
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
	if err := state.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write release metadata %q: %w", path, err)
	}

	return path, nil
}

func (fileSystemStore) SetLatest(cfg config.Config, appName string, metadata ActiveRelease, environment string) error {
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
	if err := state.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write release metadata %q: %w", path, err)
	}

	return nil
}

func (fileSystemStore) Latest(cfg config.Config, name string, environment string) (ActiveRelease, error) {
	history, err := ListHistory(cfg, name, environment)
	if err != nil {
		return ActiveRelease{}, err
	}
	if len(history) == 0 {
		return ActiveRelease{"", false}, nil
	}
	// Choose by the recorded creation timestamp rather than a filename. This
	// remains correct when IDs change format and rejects malformed records.
	return ActiveRelease{Tag: history[0].Tag, IsAvailable: true}, nil
}

func (fileSystemStore) Find(cfg config.Config, name string, environment string, tag string) (Metadata, error) {

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
