package manifest

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func Load(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest %q: %w", path, err)
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest %q: %w", path, err)
	}

	m.applyDefaults()

	if err := m.Validate(); err != nil {
		return Manifest{}, err
	}

	return m, nil
}
