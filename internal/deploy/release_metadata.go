package deploy

import (
	"encoding/json"
	"fmt"
	"os"
)

type releaseMetadata struct {
	Environment   string `json:"environment"`
	Image         string `json:"image"`
	RegistryImage string `json:"registry_image"`
	Tag           string `json:"tag"`
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
