package deploy

import (
	"context"
	"encoding/json"
	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecoveryStackUsesSavedReplicasAndImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stack.yml")
	if err := os.WriteFile(path, []byte("services:\n  app:\n    image: registry/demo@sha256:abc\n    deploy:\n      replicas: 2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	image, replicas, err := recoveryStack(path, "prod-demo_app")
	if err != nil || image != "registry/demo@sha256:abc" || replicas != 2 {
		t.Fatalf("%s %d %v", image, replicas, err)
	}
	if _, _, err := recoveryStack(path, "prod-demo_missing"); err == nil {
		t.Fatal("accepted mismatched service")
	}
	if _, _, err := recoveryStack(path+"missing", "prod-demo_app"); err == nil {
		t.Fatal("accepted missing stack")
	}
}

func TestRecoverySnapshotSurvivesFailedDeploymentOverwrite(t *testing.T) {
	cfg := config.Config{StateDir: t.TempDir()}
	svc := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)
	root := appDir(cfg, "api", "prod")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	env := envPath(cfg, "api", "prod")
	if err := os.WriteFile(env, []byte("VERSION=old\n"), 0600); err != nil {
		t.Fatal(err)
	}
	source := stackPath(cfg, "api", "prod")
	original := "services:\n  prod-api:\n    image: registry/api@sha256:old\n    env_file:\n      - " + env + "\n    volumes:\n      - data:/data\nvolumes:\n  data: {}\n"
	if err := os.WriteFile(source, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	relative, err := svc.snapshotRecovery(Deployment{App: "api", Environment: "prod", CreatedAt: time.Now()}, source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("invalid later attempt"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(env, []byte("VERSION=new\n"), 0600); err != nil {
		t.Fatal(err)
	}
	saved, err := recoverySnapshotPath(root, relative)
	if err != nil {
		t.Fatal(err)
	}
	image, _, err := recoveryStack(saved, "prod-api_prod-api")
	if err != nil || image != "registry/api@sha256:old" {
		t.Fatalf("%s %v", image, err)
	}
	values, err := os.ReadFile(filepath.Join(filepath.Dir(saved), ".env"))
	if err != nil || string(values) != "VERSION=old\n" {
		t.Fatalf("%s %v", values, err)
	}
	data, err := os.ReadFile(saved)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), filepath.Join(filepath.Dir(saved), ".env")) || !strings.Contains(string(data), "data:/data") {
		t.Fatal(string(data))
	}
	for _, bad := range []string{"../escape", "/absolute"} {
		if _, err := recoverySnapshotPath(root, bad); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
}

func TestRecoverBlocksMissingDependencyAndPersistsReason(t *testing.T) {
	cfg := config.Config{StateDir: t.TempDir()}
	svc := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)
	path := filepath.Join(t.TempDir(), "app.yml")
	content := "services:\n  db:\n    image: redis:7\n  api:\n    image: demo:1\n    x-noops:\n      depends_on: [db]\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	err := svc.Recover(context.Background(), "prod", manifest.WithService(path, "api"))
	if err == nil || !strings.Contains(err.Error(), "dependency db has no successful deployment") {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(appDir(cfg, "api", "prod"), "recovery.json"))
	if err != nil {
		t.Fatal(err)
	}
	var progress RecoveryProgress
	if err := json.Unmarshal(data, &progress); err != nil {
		t.Fatal(err)
	}
	if progress.Stage != "failed" || progress.Dependency != "db" || progress.Error == "" {
		t.Fatalf("%+v", progress)
	}
}
func TestRecoverReportsMissingImageBeforeStartingService(t *testing.T) {
	cfg := config.Config{StateDir: t.TempDir()}
	svc := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)
	path := filepath.Join(t.TempDir(), "app.yml")
	if err := os.WriteFile(path, []byte("services:\n  api:\n    image: demo:1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	d := Deployment{App: "api", Environment: "prod", CreatedAt: time.Now(), ReleaseImage: "registry/api@sha256:missing", StackName: "prod-api", ServiceName: "prod-api_prod-api"}
	if _, err := svc.deployments.Save(cfg, d); err != nil {
		t.Fatal(err)
	}
	saved := stackPath(cfg, "api", "prod")
	if err := os.WriteFile(saved, []byte("services:\n  prod-api:\n    image: registry/api@sha256:missing\n"), 0600); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	calls := filepath.Join(bin, "calls")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$RECOVERY_TEST_CALLS\"\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("RECOVERY_TEST_CALLS", calls)
	err := svc.Recover(context.Background(), "prod", path)
	if err == nil || !strings.Contains(err.Error(), "restore the registry/image backup") {
		t.Fatal(err)
	}
	data, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "pull ") || strings.Contains(string(data), "stack deploy") {
		t.Fatal(string(data))
	}
}
