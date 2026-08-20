package deploy

import (
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

func TestResolveEffectiveExecutionUsesImageDefaults(t *testing.T) {
	img := ImageMetadata{
		Entrypoint: []string{"java", "-jar"},
		Cmd:        []string{"app.jar"},
	}

	exec := ResolveEffectiveExecution(img, nil, nil)

	if len(exec.Entrypoint) != 2 || exec.Entrypoint[0] != "java" {
		t.Errorf("entrypoint = %v, want [java -jar]", exec.Entrypoint)
	}
	if len(exec.Cmd) != 1 || exec.Cmd[0] != "app.jar" {
		t.Errorf("cmd = %v, want [app.jar]", exec.Cmd)
	}
}

func TestResolveEffectiveExecutionManifestOverridesEntrypoint(t *testing.T) {
	img := ImageMetadata{
		Entrypoint: []string{"java", "-jar"},
		Cmd:        []string{"app.jar"},
	}

	exec := ResolveEffectiveExecution(img, []string{"python"}, nil)

	if len(exec.Entrypoint) != 1 || exec.Entrypoint[0] != "python" {
		t.Errorf("entrypoint = %v, want [python]", exec.Entrypoint)
	}
	if len(exec.Cmd) != 1 || exec.Cmd[0] != "app.jar" {
		t.Errorf("cmd = %v, want [app.jar]", exec.Cmd)
	}
}

func TestResolveEffectiveExecutionManifestOverridesCmd(t *testing.T) {
	img := ImageMetadata{
		Entrypoint: []string{"java", "-jar"},
		Cmd:        []string{"app.jar"},
	}

	exec := ResolveEffectiveExecution(img, nil, []string{"other.jar"})

	if len(exec.Entrypoint) != 2 || exec.Entrypoint[0] != "java" {
		t.Errorf("entrypoint = %v, want [java -jar]", exec.Entrypoint)
	}
	if len(exec.Cmd) != 1 || exec.Cmd[0] != "other.jar" {
		t.Errorf("cmd = %v, want [other.jar]", exec.Cmd)
	}
}

func TestResolveEffectiveExecutionEmptyImage(t *testing.T) {
	img := ImageMetadata{}

	exec := ResolveEffectiveExecution(img, []string{"/bin/sh", "-c"}, []string{"echo", "hello"})

	if len(exec.Entrypoint) != 2 || exec.Entrypoint[0] != "/bin/sh" {
		t.Errorf("entrypoint = %v, want [/bin/sh -c]", exec.Entrypoint)
	}
	if len(exec.Cmd) != 2 || exec.Cmd[0] != "echo" || exec.Cmd[1] != "hello" {
		t.Errorf("cmd = %v, want [echo hello]", exec.Cmd)
	}
}

func TestBuildWrapperConfigNoSecretsReturnsNoWrapper(t *testing.T) {
	cfg := BuildWrapperConfig("env", "myimage:v1", ImageMetadata{}, manifest.Manifest{}, nil, "wrapper:v1")

	if cfg.UseWrapper {
		t.Error("expected UseWrapper=false when no secret bindings")
	}
}

func TestBuildWrapperConfigFileModeReturnsNoWrapper(t *testing.T) {
	bindings := []SecretBinding{{EnvKey: "REDIS_PASSWORD", SecretName: "REDIS_PASSWORD_SECRET"}}
	cfg := BuildWrapperConfig("file", "myimage:v1", ImageMetadata{}, manifest.Manifest{}, bindings, "wrapper:v1")

	if cfg.UseWrapper {
		t.Error("expected UseWrapper=false when resolution is file")
	}
}

func TestBuildWrapperConfigEnvModeReturnsWrapper(t *testing.T) {
	img := ImageMetadata{
		Entrypoint: []string{"java", "-jar"},
		Cmd:        []string{"app.jar"},
	}
	bindings := []SecretBinding{
		{EnvKey: "REDIS_PASSWORD", SecretName: "REDIS_PASSWORD_SECRET"},
		{EnvKey: "AUTH_SECRET", SecretName: "AUTH_SECRET_SECRET"},
	}

	cfg := BuildWrapperConfig("env", "myimage:v1", img, manifest.Manifest{}, bindings, "wrapper:v1")

	if !cfg.UseWrapper {
		t.Fatal("expected UseWrapper=true")
	}
	if cfg.WrapperImage != "wrapper:v1" {
		t.Errorf("WrapperImage = %q, want %q", cfg.WrapperImage, "wrapper:v1")
	}
	if cfg.OriginalImage != "myimage:v1" {
		t.Errorf("OriginalImage = %q, want %q", cfg.OriginalImage, "myimage:v1")
	}
	if len(cfg.EffectiveExec.Entrypoint) != 2 {
		t.Errorf("EffectiveExec.Entrypoint = %v, want 2 elements", cfg.EffectiveExec.Entrypoint)
	}
	if len(cfg.SecretMappings) != 2 {
		t.Fatalf("SecretMappings = %d, want 2", len(cfg.SecretMappings))
	}
	if cfg.SecretMappings[0].EnvKey != "REDIS_PASSWORD" || cfg.SecretMappings[0].SecretName != "REDIS_PASSWORD_SECRET" {
		t.Errorf("SecretMappings[0] = %+v", cfg.SecretMappings[0])
	}
	if cfg.SecretMappings[1].EnvKey != "AUTH_SECRET" || cfg.SecretMappings[1].SecretName != "AUTH_SECRET_SECRET" {
		t.Errorf("SecretMappings[1] = %+v", cfg.SecretMappings[1])
	}
}

func TestJsonStringSliceNilReturnsEmptyArray(t *testing.T) {
	if got := jsonStringSlice(nil); got != "[]" {
		t.Errorf("jsonStringSlice(nil) = %q, want []", got)
	}
}

func TestJsonStringSlice(t *testing.T) {
	got := jsonStringSlice([]string{"a", "b"})
	want := `["a","b"]`
	if got != want {
		t.Errorf("jsonStringSlice = %q, want %q", got, want)
	}
}

func TestSecretMappingsString(t *testing.T) {
	mappings := []SecretMapping{
		{EnvKey: "REDIS_PASSWORD", SecretName: "REDIS_PW_SECRET"},
		{EnvKey: "AUTH_SECRET", SecretName: "AUTH_SECRET_SECRET"},
	}
	got := secretMappingsString(mappings)
	want := "REDIS_PASSWORD=REDIS_PW_SECRET,AUTH_SECRET=AUTH_SECRET_SECRET"
	if got != want {
		t.Errorf("secretMappingsString = %q, want %q", got, want)
	}
}
