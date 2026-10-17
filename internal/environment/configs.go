package environment

import (
	"fmt"
	"path/filepath"
	"strings"
)

// LoadLayers merges reusable config files followed by the service env file.
// Whole items are replaced by key, including transitions to/from secret refs.
func LoadLayers(manifestPath string, configs []string, serviceFile string) (File, error) {
	base, _, _ := strings.Cut(manifestPath, "#")
	paths := append([]string{}, configs...)
	if serviceFile != "" {
		paths = append(paths, serviceFile)
	}
	result := File{Sections: []Section{{Name: "merged"}}}
	indexes := map[string]int{}
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			return File{}, fmt.Errorf("config file path must not be empty")
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(filepath.Dir(base), path)
		}
		file, err := Load(path)
		if err != nil {
			return File{}, err
		}
		for _, section := range file.Sections {
			for _, item := range section.Items {
				if i, ok := indexes[item.Key]; ok {
					result.Sections[0].Items[i] = item
				} else {
					indexes[item.Key] = len(result.Sections[0].Items)
					result.Sections[0].Items = append(result.Sections[0].Items, item)
				}
			}
		}
	}
	return result, nil
}
