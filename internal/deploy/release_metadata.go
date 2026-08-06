package deploy

import (
	"encoding/json"
	"fmt"
	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"os"
	"path/filepath"
)

type releaseMetadata struct {
	Environment   string `json:"environment"`
	Image         string `json:"image"`
	RegistryImage string `json:"registry_image"`
	Tag           string `json:"tag"`
}

type currentRelease struct {
	Tag string `json:"tag"`
}

func readReleaseMetadata(path string) (releaseMetadata, error) {

	data, err := os.ReadFile(path)

	if err != nil {
		return releaseMetadata{}, fmt.Errorf("read release metadata %q: %w", path, err)
	}

	var metadata releaseMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return releaseMetadata{}, fmt.Errorf("decode release metadata %q: %w", path, err)
	}

	return metadata, nil
}

func readCurrentRelease(path string) (currentRelease, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return currentRelease{}, fmt.Errorf("read current release metadata %q: %w", path, err)
	}

	var metadata currentRelease
	if err := json.Unmarshal(data, &metadata); err != nil {
		return currentRelease{}, fmt.Errorf("decode current release metadata %q: %w", path, err)
	}

	return metadata, nil
}

func releaseMetadataPath(cfg config.Config, name string, environment string) string {
	return filepath.Join(appDir(cfg, name, environment), "release.json")
}

func releaseMetadataHistoryPath(cfg config.Config, name string, environment string, tag string) string {
	return filepath.Join(appDir(cfg, name, environment), "releases", tag+".json")
}
