package deploy

import "testing"

func TestResolveEnvFileSeparatesValuesFromSecretReferences(t *testing.T) {
	resolved := ResolveEnvFile(EnvFile{Sections: []EnvSection{{Items: []EnvItem{
		{Key: "APP_ENV", Value: "production"},
		{Key: "DATABASE_URL", FromSecret: "PROD_DATABASE_URL"},
		{Key: "LOG_LEVEL", Values: map[string]string{"prod": "info", "dev": "debug"}},
	}}}}, "prod", nil)

	if got, want := resolved.Values["APP_ENV"], "production"; got != want {
		t.Errorf("APP_ENV = %q, want %q", got, want)
	}
	if got, want := resolved.Values["LOG_LEVEL"], "info"; got != want {
		t.Errorf("LOG_LEVEL = %q, want %q", got, want)
	}
	if _, exists := resolved.Values["DATABASE_URL"]; exists {
		t.Error("DATABASE_URL should be a secret reference, not a plain env value")
	}
	if len(resolved.SecretRefs) != 1 {
		t.Fatalf("secret refs = %v, want one", resolved.SecretRefs)
	}
	if got, want := resolved.SecretRefs[0], (EnvSecretRef{Key: "DATABASE_URL", SecretName: "PROD_DATABASE_URL"}); got != want {
		t.Errorf("secret ref = %#v, want %#v", got, want)
	}
}

func TestResolveEnvFileAllowlistFiltersSecretRefs(t *testing.T) {
	envFile := EnvFile{Sections: []EnvSection{{Items: []EnvItem{
		{Key: "REDIS_PASSWORD", FromSecret: "REDIS_PASSWORD_SECRET"},
		{Key: "AUTH_SECRET", FromSecret: "AUTH_SECRET_SECRET"},
		{Key: "INTERNAL_KEY", FromSecret: "INTERNAL_KEY_SECRET"},
	}}}}

	resolved := ResolveEnvFile(envFile, "dev", []string{"REDIS_PASSWORD", "AUTH_SECRET"})

	if len(resolved.SecretRefs) != 2 {
		t.Fatalf("secret refs = %d, want 2", len(resolved.SecretRefs))
	}

	keys := make(map[string]bool)
	for _, ref := range resolved.SecretRefs {
		keys[ref.Key] = true
	}

	if !keys["REDIS_PASSWORD"] {
		t.Error("REDIS_PASSWORD should be in allowlist")
	}
	if !keys["AUTH_SECRET"] {
		t.Error("AUTH_SECRET should be in allowlist")
	}
	if keys["INTERNAL_KEY"] {
		t.Error("INTERNAL_KEY should NOT be in allowlist")
	}
}

func TestResolveEnvFileNilAllowlistIncludesAllSecretRefs(t *testing.T) {
	envFile := EnvFile{Sections: []EnvSection{{Items: []EnvItem{
		{Key: "A", FromSecret: "A_SECRET"},
		{Key: "B", FromSecret: "B_SECRET"},
	}}}}

	resolved := ResolveEnvFile(envFile, "dev", nil)

	if len(resolved.SecretRefs) != 2 {
		t.Fatalf("secret refs = %d, want 2", len(resolved.SecretRefs))
	}
}

func TestResolveEnvFileEmptyAllowlistExcludesAllSecretRefs(t *testing.T) {
	envFile := EnvFile{Sections: []EnvSection{{Items: []EnvItem{
		{Key: "A", FromSecret: "A_SECRET"},
		{Key: "B", FromSecret: "B_SECRET"},
	}}}}

	resolved := ResolveEnvFile(envFile, "dev", []string{})

	if len(resolved.SecretRefs) != 0 {
		t.Fatalf("secret refs = %d, want 0", len(resolved.SecretRefs))
	}
}

func TestResolveEnvFileAllowlistIgnoresNonSecretKeys(t *testing.T) {
	envFile := EnvFile{Sections: []EnvSection{{Items: []EnvItem{
		{Key: "SERVER_PORT", Value: "8080"},
		{Key: "REDIS_PASSWORD", FromSecret: "REDIS_PASSWORD_SECRET"},
	}}}}

	resolved := ResolveEnvFile(envFile, "dev", []string{"SERVER_PORT", "REDIS_PASSWORD"})

	if got, want := resolved.Values["SERVER_PORT"], "8080"; got != want {
		t.Errorf("SERVER_PORT = %q, want %q", got, want)
	}
	if len(resolved.SecretRefs) != 1 {
		t.Fatalf("secret refs = %d, want 1", len(resolved.SecretRefs))
	}
	if got, want := resolved.SecretRefs[0].Key, "REDIS_PASSWORD"; got != want {
		t.Errorf("secret ref key = %q, want %q", got, want)
	}
}
