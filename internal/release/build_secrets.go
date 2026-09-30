package release

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/AustinOyugi/no-oops-ops/internal/environment"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

// BuildSecretBinding identifies a Swarm secret that BuildKit receives for the
// duration of an isolated build instruction. ID is the Dockerfile secret ID.
type BuildSecretBinding struct {
	ID        string
	SwarmName string
}

func (s *Service) buildSecretBindings(ctx context.Context, manifestPath string, m manifest.Manifest, target string) ([]BuildSecretBinding, error) {
	if m.Env.Build == nil || len(m.Env.Build.Secrets) == 0 {
		return nil, nil
	}
	if m.Env.File == "" {
		return nil, fmt.Errorf("env.build.secrets requires env.file entries backed by from_secret")
	}

	file, err := environment.Load(filepath.Join(filepath.Dir(manifestPath), m.Env.File))
	if err != nil {
		return nil, err
	}
	resolved := environment.Resolve(file, target, m.Env.Build.Secrets)
	refs := make(map[string]string, len(resolved.SecretRefs))
	for _, ref := range resolved.SecretRefs {
		refs[ref.Key] = ref.SecretName
	}

	bindings := make([]BuildSecretBinding, 0, len(m.Env.Build.Secrets))
	for _, key := range m.Env.Build.Secrets {
		secretName := refs[key]
		if secretName == "" {
			return nil, fmt.Errorf("env.build.secrets key %q must be declared with from_secret in %q", key, m.Env.File)
		}
		metadata, err := s.secrets.Latest(ctx, target, secretName)
		if err != nil {
			return nil, fmt.Errorf("resolve build secret %q: %w", key, err)
		}
		bindings = append(bindings, BuildSecretBinding{ID: key, SwarmName: metadata.SwarmName})
	}
	return bindings, nil
}
