package manifest

import "testing"

func boolPtr(value bool) *bool {
	return &value
}

func TestValidateAcceptsValidResolutionMode(t *testing.T) {
	for _, mode := range []string{"env", "file"} {
		secrets := &EnvSecrets{Resolution: mode, Resolvable: []string{"KEY"}}
		m := Manifest{
			Name:        "test",
			Image:       Image{Repository: "repo"},
			Service:     Service{InternalPort: 8080},
			Healthcheck: Healthcheck{Test: []string{"CMD", "true"}},
			Source:      Source{Context: ".", Dockerfile: "Dockerfile"},
			Env:         Env{Secrets: secrets},
		}

		if err := m.Validate(); err != nil {
			t.Errorf("resolution %q should be valid: %v", mode, err)
		}
	}
}

func TestValidateRejectsInvalidResolutionMode(t *testing.T) {
	secrets := &EnvSecrets{Resolution: "something-else"}
	m := Manifest{
		Name:        "test",
		Image:       Image{Repository: "repo"},
		Service:     Service{InternalPort: 8080},
		Healthcheck: Healthcheck{Test: []string{"CMD", "true"}},
		Source:      Source{Context: ".", Dockerfile: "Dockerfile"},
		Env:         Env{Secrets: secrets},
	}

	if err := m.Validate(); err == nil {
		t.Error("expected validation error for invalid resolution mode")
	}
}

func TestValidateAcceptsNilSecrets(t *testing.T) {
	m := Manifest{
		Name:        "test",
		Image:       Image{Repository: "repo"},
		Service:     Service{InternalPort: 8080},
		Healthcheck: Healthcheck{Test: []string{"CMD", "true"}},
		Source:      Source{Context: ".", Dockerfile: "Dockerfile"},
	}

	if err := m.Validate(); err != nil {
		t.Errorf("nil secrets should be valid: %v", err)
	}
}

func TestValidateAllowsExternalImageWithoutSource(t *testing.T) {
	m := Manifest{
		Name:        "test",
		Image:       Image{Repository: "quay.io/example/app", Tag: "1.0", Build: boolPtr(false)},
		Service:     Service{InternalPort: 8080},
		Healthcheck: Healthcheck{Test: []string{"CMD", "true"}},
	}

	if err := m.Validate(); err != nil {
		t.Fatalf("external image manifest should be valid: %v", err)
	}
}

func TestValidateRequiresSourceForBuiltImage(t *testing.T) {
	m := Manifest{
		Name:        "test",
		Image:       Image{Repository: "repo", Build: boolPtr(true)},
		Service:     Service{InternalPort: 8080},
		Healthcheck: Healthcheck{Test: []string{"CMD", "true"}},
	}

	if err := m.Validate(); err == nil {
		t.Fatal("expected source validation error for built image")
	}
}

func TestEnvSecretsValidateRejectsEmptyResolution(t *testing.T) {
	s := &EnvSecrets{Resolution: ""}
	if err := s.Validate(); err == nil {
		t.Error("expected error for empty resolution")
	}
}
