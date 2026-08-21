package deploy

import (
	"slices"
	"strings"
	"testing"
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
	cfg := BuildWrapperConfig("env", "myimage:v1", ImageMetadata{}, nil, nil)

	if cfg.UseWrapper {
		t.Error("expected UseWrapper=false when no secret bindings")
	}
}

func TestBuildWrapperConfigFileModeReturnsNoWrapper(t *testing.T) {
	bindings := []SecretBinding{{EnvKey: "REDIS_PASSWORD", SecretName: "REDIS_PASSWORD_SECRET"}}
	cfg := BuildWrapperConfig("file", "myimage:v1", ImageMetadata{}, nil, bindings)

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

	cfg := BuildWrapperConfig("env", "myimage:v1", img, nil, bindings)

	if !cfg.UseWrapper {
		t.Fatal("expected UseWrapper=true")
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
	got := secretMappingsValue(mappings)
	want := "REDIS_PASSWORD=/run/secrets/REDIS_PASSWORD,AUTH_SECRET=/run/secrets/AUTH_SECRET"
	if got != want {
		t.Errorf("secretMappingsValue = %q, want %q", got, want)
	}
}

func TestBuildWrapperConfigRejectsMissingExecutionContract(t *testing.T) {
	cfg := BuildWrapperConfig("env", "myimage:v1", ImageMetadata{}, nil, []SecretBinding{{EnvKey: "KEY", SecretName: "SECRET"}})
	if cfg.UseWrapper {
		t.Error("expected no wrapper when the image has no entrypoint or command")
	}
}

func TestWrappedImageDockerfileUsesApplicationImageAsBase(t *testing.T) {
	got := wrappedImageDockerfile("registry/app:v1")
	if !strings.Contains(got, "FROM registry/app:v1") || !strings.Contains(got, "COPY bootstrap.sh /bootstrap.sh") {
		t.Fatalf("unexpected Dockerfile:\n%s", got)
	}
}

func TestBuildWrapperConfigUsesManifestCommand(t *testing.T) {
	cfg := BuildWrapperConfig("env", "myimage:v1", ImageMetadata{Entrypoint: []string{"run"}, Cmd: []string{"default"}}, []string{"override"}, []SecretBinding{{EnvKey: "KEY", SecretName: "SECRET"}})
	if got, want := cfg.EffectiveExec.Cmd, []string{"override"}; !slices.Equal(got, want) {
		t.Errorf("command = %v, want %v", got, want)
	}
}
