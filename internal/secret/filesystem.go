package secret

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type filesystemStore struct{}

func newFilesystemStore() filesystemStore {
	return filesystemStore{}
}

func (filesystemStore) Save(stateDir string, metadata Metadata) (string, error) {
	dir := metadataDir(stateDir, metadata.Environment, metadata.Key)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create secret metadata directory %q: %w", dir, err)
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal secret metadata: %w", err)
	}
	data = append(data, '\n')

	path := metadataPath(dir, metadata.Version)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write secret metadata %q: %w", path, err)
	}

	return path, nil
}

func (filesystemStore) List(stateDir string, environment string) ([]Metadata, error) {
	dir := filepath.Join(stateDir, "secrets", environment)
	keys, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []Metadata{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read secret metadata directory %q: %w", dir, err)
	}

	var metadata []Metadata
	for _, key := range keys {
		if !key.IsDir() {
			continue
		}

		versions, err := os.ReadDir(filepath.Join(dir, key.Name()))
		if err != nil {
			return nil, fmt.Errorf("read secret versions for %q: %w", key.Name(), err)
		}

		for _, version := range versions {
			if version.IsDir() || filepath.Ext(version.Name()) != ".json" {
				continue
			}

			path := filepath.Join(dir, key.Name(), version.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("read secret metadata %q: %w", path, err)
			}

			var item Metadata
			if err := json.Unmarshal(data, &item); err != nil {
				return nil, fmt.Errorf("decode secret metadata %q: %w", path, err)
			}
			metadata = append(metadata, item)
		}
	}

	sort.Slice(metadata, func(i, j int) bool {
		if metadata[i].Key == metadata[j].Key {
			return metadata[i].Version < metadata[j].Version
		}
		return metadata[i].Key < metadata[j].Key
	})

	return metadata, nil
}

func metadataDir(stateDir string, environment string, key string) string {
	return filepath.Join(stateDir, "secrets", environment, key)
}

func metadataPath(dir string, version int) string {
	return filepath.Join(dir, fmt.Sprintf("v%d.json", version))
}
