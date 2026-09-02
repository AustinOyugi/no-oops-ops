package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseServiceArgsSelectsTheOnlyServiceImplicitly(t *testing.T) {
	manifestPath := filepath.Join(t.TempDir(), "app.yml")
	if err := os.WriteFile(manifestPath, []byte("services:\n  api:\n    image: api\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolve := func(string) (string, error) { return manifestPath, nil }

	environment, path, services, err := parseServiceArgs([]string{"prod", "api"}, "release", resolve, true)
	if err != nil {
		t.Fatalf("parseServiceArgs returned error: %v", err)
	}
	if environment != "prod" || path != manifestPath || len(services) != 1 || services[0] != "api" {
		t.Errorf("parseServiceArgs = (%q, %q, %v), want implicit api release", environment, path, services)
	}
}

func TestParseServiceArgsRequiresSelectionForMultiServiceRelease(t *testing.T) {
	manifestPath := filepath.Join(t.TempDir(), "app.yml")
	if err := os.WriteFile(manifestPath, []byte("services:\n  api:\n    image: api\n  worker:\n    image: worker\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolve := func(string) (string, error) { return manifestPath, nil }

	_, _, _, err := parseServiceArgs([]string{"prod", "api"}, "release", resolve, true)
	if err == nil || !strings.Contains(err.Error(), "multiple services") {
		t.Fatalf("parseServiceArgs error = %v, want multi-service selection error", err)
	}
}

func TestParseReleaseArgsAcceptsDeployBeforeTheTarget(t *testing.T) {
	manifestPath := filepath.Join(t.TempDir(), "app.yml")
	if err := os.WriteFile(manifestPath, []byte("services:\n  api:\n    image: api\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolve := func(string) (string, error) { return manifestPath, nil }

	environment, path, services, deployAfterRelease, err := parseReleaseArgs([]string{"--deploy", "prod", "api"}, resolve)
	if err != nil {
		t.Fatalf("parseReleaseArgs returned error: %v", err)
	}
	if environment != "prod" || path != manifestPath || len(services) != 1 || services[0] != "api" || !deployAfterRelease {
		t.Errorf("parseReleaseArgs = (%q, %q, %v, %t), want release with deploy", environment, path, services, deployAfterRelease)
	}
}

func TestParseReleaseArgsAcceptsDeployAfterTheSelector(t *testing.T) {
	manifestPath := filepath.Join(t.TempDir(), "app.yml")
	if err := os.WriteFile(manifestPath, []byte("services:\n  api:\n    image: api\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolve := func(string) (string, error) { return manifestPath, nil }

	environment, path, services, deployAfterRelease, err := parseReleaseArgs([]string{"prod", "api", "--service", "api", "--deploy"}, resolve)
	if err != nil {
		t.Fatalf("parseReleaseArgs returned error: %v", err)
	}
	if environment != "prod" || path != manifestPath || len(services) != 1 || services[0] != "api" || !deployAfterRelease {
		t.Errorf("parseReleaseArgs = (%q, %q, %v, %t), want release with deploy", environment, path, services, deployAfterRelease)
	}
}
