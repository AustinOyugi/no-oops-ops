package deploy

import "testing"

func TestResolveEnvFileSeparatesValuesFromSecretReferences(t *testing.T) {
	resolved := ResolveEnvFile(EnvFile{Sections: []EnvSection{{Items: []EnvItem{
		{Key: "APP_ENV", Value: "production"},
		{Key: "DATABASE_URL", FromSecret: "PROD_DATABASE_URL"},
		{Key: "LOG_LEVEL", Values: map[string]string{"prod": "info", "dev": "debug"}},
	}}}}, "prod")

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
