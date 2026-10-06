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

func TestLogServiceName(t *testing.T) {
	for _, tc := range []struct {
		active Deployment
		want   string
	}{
		{Deployment{}, "prod-api_prod-api"},
		{Deployment{ServiceName: "active_app"}, "active_app"},
		{Deployment{StackName: "prod-api"}, "prod-api_prod-api"},
		{Deployment{StackName: "prod-api-r123"}, "prod-api-r123_app"},
	} {
		if got := logServiceName("prod", "api", tc.active); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}

func TestLogsStreamsDockerOutput(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\"\nprintf 'application error\\n' >&2\n"
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	path := filepath.Join(dir, "app.yml")
	if err := os.WriteFile(path, []byte("services:\n  api:\n    image: api\n"), 0600); err != nil {
		t.Fatal(err)
	}
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), config.Config{StateDir: dir})
	_, err := service.deployments.Save(service.config, Deployment{App: "api", Environment: "prod", CreatedAt: time.Now(), ServiceName: "prod-api-r123_app"})
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	err = service.Logs(context.Background(), "prod", path, LogOptions{Follow: true, Tail: "20", Since: "10m", Timestamps: true}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	want := "service\nlogs\n--tail\n20\n--follow\n--since\n10m\n--timestamps\n--\nprod-api-r123_app\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.String() != "application error\n" {
		t.Fatalf("stderr = %q", stderr.String())
	}
	// Docker errors propagate, while a cancelled follow ends cleanly.
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte("#!/bin/sh\nexit 7\n"), 0700); err != nil {
		t.Fatal(err)
	}
	err = service.Logs(context.Background(), "prod", path, LogOptions{Tail: "all"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "exit status 7") {
		t.Fatalf("error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := service.Logs(ctx, "prod", path, LogOptions{Tail: "0", Follow: true}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
}

func TestLogOptionsValidate(t *testing.T) {
	for _, tail := range []string{"-1", "invalid", ""} {
		if err := (LogOptions{Tail: tail}).Validate(); err == nil {
			t.Fatalf("accepted tail %q", tail)
		}
	}
}
