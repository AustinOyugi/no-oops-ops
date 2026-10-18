package environment

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConfigRef selects one item from a reusable config, optionally renaming its key.
// A scalar is shorthand for file; the environment item's key is the default.
type ConfigRef struct {
	File string `yaml:"file"`
	Key  string `yaml:"key,omitempty"`
}

func (r *ConfigRef) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		if node.Tag != "!!str" {
			return fmt.Errorf("from_config must be a file path or a mapping with file and optional key")
		}
		r.File = node.Value
	case yaml.MappingNode:
		for n := 0; n+1 < len(node.Content); n += 2 {
			if key := node.Content[n].Value; key != "file" && key != "key" {
				return fmt.Errorf("unknown from_config field %q", key)
			}
		}
		type plain ConfigRef
		if err := node.Decode((*plain)(r)); err != nil {
			return err
		}
	default:
		return fmt.Errorf("from_config must be a file path or a mapping with file and optional key")
	}
	if strings.TrimSpace(r.File) == "" {
		return fmt.Errorf("from_config.file must not be empty")
	}
	return nil
}

// LoadForManifest loads the selected service env file relative to its manifest.
func LoadForManifest(manifestPath, serviceFile string) (File, error) {
	if serviceFile == "" {
		return File{}, nil
	}
	base, _, _ := strings.Cut(manifestPath, "#")
	if !filepath.IsAbs(serviceFile) {
		serviceFile = filepath.Join(filepath.Dir(base), serviceFile)
	}
	return Load(serviceFile)
}

type configLoader struct {
	files    map[string]File
	visiting map[string]bool
}

func newConfigLoader() *configLoader {
	return &configLoader{files: map[string]File{}, visiting: map[string]bool{}}
}

func (l *configLoader) file(path string) (File, error) {
	if file, ok := l.files[path]; ok {
		return file, nil
	}
	file, err := readFile(path)
	if err != nil {
		return File{}, err
	}
	l.files[path] = file
	return file, nil
}

func (l *configLoader) loadEnvironment(path string) (File, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return File{}, err
	}
	file, err := l.file(path)
	if err != nil {
		return File{}, err
	}
	// Copy slices: resolving a service must not mutate cached config documents.
	result := File{Sections: make([]Section, len(file.Sections))}
	for s, section := range file.Sections {
		result.Sections[s] = Section{Name: section.Name, Items: make([]Item, len(section.Items))}
		for n, item := range section.Items {
			resolved, err := l.resolve(path, item)
			if err != nil {
				return File{}, fmt.Errorf("env file %q key %q: %w", path, item.Key, err)
			}
			result.Sections[s].Items[n] = resolved
		}
	}
	return result, nil
}

func (l *configLoader) resolve(path string, item Item) (Item, error) {
	if item.FromConfig == nil {
		return item, nil
	}
	if item.FromSecret != "" {
		return Item{}, fmt.Errorf("from_config and from_secret cannot be combined")
	}
	ref := *item.FromConfig
	if strings.TrimSpace(ref.File) == "" {
		return Item{}, fmt.Errorf("from_config.file must not be empty")
	}
	if ref.Key == "" {
		ref.Key = item.Key
	}
	if ref.Key == "" {
		return Item{}, fmt.Errorf("from_config requires a key")
	}
	target := ref.File
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	target, err := filepath.Abs(target)
	if err != nil {
		return Item{}, err
	}
	// Canonicalize existing paths so symlink aliases cannot hide a reference cycle.
	if real, err := filepath.EvalSymlinks(target); err == nil {
		target = real
	}
	id := target + "#" + ref.Key
	if l.visiting[id] {
		return Item{}, fmt.Errorf("from_config reference cycle at %s", id)
	}
	l.visiting[id] = true
	defer delete(l.visiting, id)
	file, err := l.file(target)
	if err != nil {
		return Item{}, fmt.Errorf("from_config %q: %w", ref.File, err)
	}
	var selected Item
	matches := 0
	for _, section := range file.Sections {
		for _, candidate := range section.Items {
			if candidate.Key == ref.Key {
				selected = candidate
				matches++
			}
		}
	}
	if matches == 0 {
		return Item{}, fmt.Errorf("from_config key %q not found in %q", ref.Key, target)
	}
	if matches > 1 {
		return Item{}, fmt.Errorf("from_config key %q is duplicated in %q", ref.Key, target)
	}
	base, err := l.resolve(target, selected)
	if err != nil {
		return Item{}, err
	}
	base.Key = item.Key
	base.FromConfig = nil
	if item.valueSet || item.Value != "" {
		base.Value, base.valueSet = item.Value, true
		base.Values = nil
		base.FromSecret = ""
	}
	if len(item.Values) > 0 {
		values := make(map[string]string, len(base.Values)+len(item.Values))
		for key, value := range base.Values {
			values[key] = value
		}
		for key, value := range item.Values {
			values[key] = value
		}
		base.Values = values
		if base.FromSecret != "" {
			return Item{}, fmt.Errorf("cannot override secret-backed from_config with values; use value or a local from_secret instead")
		}
	}
	base.emitEmpty = base.valueSet && base.Value == ""
	return base, nil
}
