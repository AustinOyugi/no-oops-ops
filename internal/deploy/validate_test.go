package deploy

import (
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

func TestValidateResolvableKeysAcceptsValidKeys(t *testing.T) {
	m := manifest.Manifest{
		Name: "test",
		Env: manifest.Env{
			File: "test.env.yml",
			Secrets: &manifest.EnvSecrets{
				Resolution: "env",
				Resolvable: []string{"REDIS_PASSWORD"},
			},
		},
	}
	envFile := EnvFile{Sections: []EnvSection{{Items: []EnvItem{
		{Key: "REDIS_PASSWORD", FromSecret: "REDIS_PASSWORD_SECRET"},
	}}}}

	if err := ValidateResolvableKeys(m, envFile); err != nil {
		t.Errorf("expected no error: %v", err)
	}
}

func TestValidateResolvableKeysRejectsUndefinedKey(t *testing.T) {
	m := manifest.Manifest{
		Name: "test",
		Env: manifest.Env{
			File: "test.env.yml",
			Secrets: &manifest.EnvSecrets{
				Resolution: "env",
				Resolvable: []string{"MISSING_KEY"},
			},
		},
	}
	envFile := EnvFile{Sections: []EnvSection{{Items: []EnvItem{
		{Key: "REDIS_PASSWORD", FromSecret: "REDIS_PASSWORD_SECRET"},
	}}}}

	if err := ValidateResolvableKeys(m, envFile); err == nil {
		t.Error("expected error for undefined resolvable key")
	}
}

func TestValidateResolvableKeysRejectsNonSecretKey(t *testing.T) {
	m := manifest.Manifest{
		Name: "test",
		Env: manifest.Env{
			File: "test.env.yml",
			Secrets: &manifest.EnvSecrets{
				Resolution: "env",
				Resolvable: []string{"SERVER_PORT"},
			},
		},
	}
	envFile := EnvFile{Sections: []EnvSection{{Items: []EnvItem{
		{Key: "SERVER_PORT", Value: "8080"},
	}}}}

	if err := ValidateResolvableKeys(m, envFile); err == nil {
		t.Error("expected error for non-secret resolvable key")
	}
}

func TestValidateResolvableKeysSkipsWhenSecretsNil(t *testing.T) {
	m := manifest.Manifest{
		Name: "test",
		Env:  manifest.Env{File: "test.env.yml"},
	}
	envFile := EnvFile{}

	if err := ValidateResolvableKeys(m, envFile); err != nil {
		t.Errorf("expected no error when secrets is nil: %v", err)
	}
}
