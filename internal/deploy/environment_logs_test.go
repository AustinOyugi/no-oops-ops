package deploy

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
)

func environmentLogsFixture(t *testing.T, script string) *Service {
	t.Helper()
	dir := t.TempDir()
	for name, data := range map[string]string{
		"apps.yml": "apps:\n  backend:\n    manifest: app.yml\n  duplicate:\n    manifest: app.yml\n",
		"app.yml":  "services:\n  api:\n    image: api\n  worker:\n    image: worker\n  library:\n    image: library\n    x-noops: {deploy: false}\n",
		"docker":   "#!/bin/sh\n" + script,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("LOG_TEST_DIR", dir)
	return NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), config.Config{Workspace: dir, StateDir: filepath.Join(dir, "state")})
}

func TestEnvironmentLogDiscovery(t *testing.T) {
	service := environmentLogsFixture(t, `printf '%s\n' prod-api_prod-api prod-worker-r123_app dev-api_dev-api production-api_production-api unrelated prod-library_prod-library prod-api-r123_other compact-r123_app prod-orphan_prod-orphan prod-api_prod-api`)
	for _, record := range []Deployment{
		{App: "worker", Environment: "prod", ServiceName: "compact-r123_app", CreatedAt: time.Now()},
		{App: "orphan", Environment: "prod", StackName: "prod-orphan", CreatedAt: time.Now()},
	} {
		if _, err := service.deployments.Save(service.config, record); err != nil {
			t.Fatal(err)
		}
	}
	names, err := service.environmentLogServices(context.Background(), "prod")
	if err != nil {
		t.Fatal(err)
	}
	want := "compact-r123_app,prod-api_prod-api,prod-orphan_prod-orphan,prod-worker-r123_app"
	if strings.Join(names, ",") != want {
		t.Fatalf("services = %v, want %s", names, want)
	}
}

func TestEnvironmentLogsStreamConcurrently(t *testing.T) {
	service := environmentLogsFixture(t, `
if [ "$2" = ls ]; then
 printf '%s\n' prod-api_prod-api prod-worker_prod-worker
 exit 0
fi
for name do :; done
touch "$LOG_TEST_DIR/$name.started"
attempt=0
while [ ! -f "$LOG_TEST_DIR/prod-api_prod-api.started" ] || [ ! -f "$LOG_TEST_DIR/prod-worker_prod-worker.started" ]; do
 attempt=$((attempt + 1))
 [ "$attempt" -lt 100 ] || exit 9
 sleep 0.01
done
printf '%s | stdout\n' "$name"
printf '%s | stderr\n' "$name" >&2
`)
	var output strings.Builder
	err := service.EnvironmentLogs(context.Background(), "prod", LogOptions{Tail: "100"}, &output, &output)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"prod-api_prod-api", "prod-worker_prod-worker"} {
		for _, stream := range []string{"stdout", "stderr"} {
			if !strings.Contains(output.String(), name+" | "+stream) {
				t.Errorf("missing %s %s: %q", name, stream, output.String())
			}
		}
	}
}

func TestEnvironmentLogsFailureDoesNotStopOtherServices(t *testing.T) {
	service := environmentLogsFixture(t, `
if [ "$2" = ls ]; then
 printf '%s\n' prod-api_prod-api prod-worker_prod-worker
 exit 0
fi
for name do :; done
if [ "$name" = prod-api_prod-api ]; then exit 7; fi
printf 'worker still streaming\n'
touch "$LOG_TEST_DIR/worker.started"
exec sleep 10
`)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- service.EnvironmentLogs(ctx, "prod", LogOptions{Tail: "0", Follow: true}, io.Discard, io.Discard)
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(service.config.Workspace, "worker.started")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("worker stream did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	select {
	case err := <-done:
		t.Fatalf("worker stream stopped early: %v", err)
	default:
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("cancelled stream = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancel did not stop all streams")
	}
}

func TestEnvironmentLogsEmptyAndDockerErrors(t *testing.T) {
	service := environmentLogsFixture(t, "exit 0\n")
	err := service.EnvironmentLogs(context.Background(), "prod", LogOptions{Tail: "100"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "no deployed services") {
		t.Fatalf("empty environment = %v", err)
	}
	err = service.EnvironmentLogs(context.Background(), "../prod", LogOptions{Tail: "100"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "invalid environment") {
		t.Fatalf("invalid environment = %v", err)
	}
	if err := os.WriteFile(filepath.Join(service.config.Workspace, "docker"), []byte("#!/bin/sh\necho unavailable >&2\nexit 7\n"), 0700); err != nil {
		t.Fatal(err)
	}
	err = service.EnvironmentLogs(context.Background(), "prod", LogOptions{Tail: "100"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("docker error = %v", err)
	}
}
