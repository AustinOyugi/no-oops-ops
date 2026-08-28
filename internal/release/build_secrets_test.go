package release

import (
	"reflect"
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

func TestIsolatedBuildArgsHasOneBuildContext(t *testing.T) {
	got := isolatedBuildArgs("registry.example/app:tag", "Dockerfile", manifest.BuildResources{}, []BuildSecretBinding{{ID: "SENTRY_AUTH_TOKEN"}})
	want := []string{
		"-t", "registry.example/app:tag",
		"-f", "/work/Dockerfile",
		"--secret", "id=SENTRY_AUTH_TOKEN,src=/run/secrets/SENTRY_AUTH_TOKEN",
		"/work",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("isolatedBuildArgs() = %q, want %q", got, want)
	}
}
