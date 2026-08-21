package deploy

import (
	"fmt"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

func ValidateResolvableKeys(m manifest.Manifest, envFile EnvFile) error {
	if m.Env.Secrets == nil {
		return nil
	}

	secretKeys := make(map[string]bool)
	for _, section := range envFile.Sections {
		for _, item := range section.Items {
			if item.FromSecret != "" {
				secretKeys[item.Key] = true
			}
		}
	}

	allKeys := make(map[string]bool)
	for _, section := range envFile.Sections {
		for _, item := range section.Items {
			allKeys[item.Key] = true
		}
	}

	for _, key := range m.Env.Secrets.Resolvable {
		if !allKeys[key] {
			return fmt.Errorf("app %q: resolvable key %q is not defined in %s", m.Name, key, m.Env.File)
		}
		if !secretKeys[key] {
			return fmt.Errorf("app %q: resolvable key %q is not a secret-backed key in %s", m.Name, key, m.Env.File)
		}
	}

	return nil
}
