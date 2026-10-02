package release

import (
	"context"
	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

func TestIsolatedBuildArgsHasOneBuildContext(t *testing.T) {
	got := isolatedBuildArgs("registry.example/app:tag", "Dockerfile", manifest.BuildResources{}, false, []BuildSecretBinding{{ID: "SENTRY_AUTH_TOKEN"}})
	want := []string{
		"--no-cache",
		"-t", "registry.example/app:tag",
		"-f", "/work/Dockerfile",
		"--secret", "id=SENTRY_AUTH_TOKEN,src=/run/secrets/SENTRY_AUTH_TOKEN",
		"/work",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("isolatedBuildArgs() = %q, want %q", got, want)
	}
}

func TestIsolatedBuildArgsHasNoResourceLimitsByDefault(t *testing.T) {
	got := isolatedBuildArgs("registry.example/app:tag", "Dockerfile", manifest.BuildResources{}, false, nil)
	want := []string{
		"-t", "registry.example/app:tag",
		"-f", "/work/Dockerfile",
		"/work",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("isolatedBuildArgs() = %q, want %q", got, want)
	}
}

func TestIsolatedBuildArgsAddsOnlyConfiguredResourceLimits(t *testing.T) {
	got := isolatedBuildArgs("registry.example/app:tag", "Dockerfile", manifest.BuildResources{Memory: "2Gi"}, false, nil)
	want := []string{
		"-t", "registry.example/app:tag",
		"-f", "/work/Dockerfile",
		"--memory", "2Gi",
		"/work",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("isolatedBuildArgs() = %q, want %q", got, want)
	}
}

func TestIsolatedBuildArgsBypassesCacheWhenRequested(t *testing.T) {
	got := isolatedBuildArgs("registry.example/app:tag", "Dockerfile", manifest.BuildResources{}, true, nil)
	want := []string{
		"--no-cache",
		"-t", "registry.example/app:tag",
		"-f", "/work/Dockerfile",
		"/work",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("isolatedBuildArgs() = %q, want %q", got, want)
	}
}

func TestCancelledBuildRemovesSwarmService(t *testing.T) {
	root := t.TempDir()
	script := `#!/bin/sh
case "$1 $2" in
 "service create") echo "$@" > "$BUILD_TEST_CREATED" ;;
 "service ps") while :; do sleep 0.05; done ;;
 "service logs") while :; do sleep 0.05; done ;;
 "service rm") echo "$3" > "$BUILD_TEST_REMOVED" ;;
esac
`
	if err := os.WriteFile(filepath.Join(root, "docker"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	created, removed := filepath.Join(root, "created"), filepath.Join(root, "removed")
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BUILD_TEST_CREATED", created)
	t.Setenv("BUILD_TEST_REMOVED", removed)
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), config.Config{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- service.buildImageIsolated(ctx, "example:test", filepath.Join(root, "Dockerfile"), root, manifest.BuildResources{}, false, nil)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(created); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("build service did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	var err error
	select {
	case err = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup did not finish")
	}
	if err == nil {
		t.Fatal("cancelled build succeeded")
	}
	args, err := os.ReadFile(created)
	if err != nil {
		t.Fatal(err)
	}
	name, err := os.ReadFile(removed)
	if err != nil {
		t.Fatalf("temporary workload not removed: %v", err)
	}
	if !strings.Contains(string(args), "--name "+strings.TrimSpace(string(name))+" ") {
		t.Fatalf("removed wrong service: %q / %q", args, name)
	}
}
