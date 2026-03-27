package local

import (
	"encoding/json"
	"fmt"
	"os"
)

func readMetadata(path string) (metadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return metadata{}, fmt.Errorf("read metadata %q: %w", path, err)
	}

	var m metadata
	if err := json.Unmarshal(data, &m); err != nil {
		return metadata{}, fmt.Errorf("decode metadata %q: %w", path, err)
	}

	return m, nil
}
