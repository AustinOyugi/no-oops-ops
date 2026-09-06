package app

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
)

func TestResolveTarget(t *testing.T) {
	workspace := t.TempDir()
	manifestPath := filepath.Join(workspace, "app.yml")
	if err := os.WriteFile(manifestPath, []byte("services:\n  api:\n    image: api\n  worker:\n    image: worker\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "apps.yml"), []byte("apps:\n  example:\n    manifest: ./app.yml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	application := &App{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), config: config.Config{Workspace: workspace}}

	tests := []struct {
		name     string
		target   Target
		implicit bool
		want     string
		wantErr  string
	}{
		{name: "all", target: Target{Environment: "prod", App: "example", All: true}, want: "api,worker"},
		{name: "named", target: Target{Environment: "prod", App: "example", Service: "worker"}, want: "worker"},
		{name: "missing selection", target: Target{Environment: "prod", App: "example"}, implicit: true, wantErr: "select a service"},
		{name: "conflicting selection", target: Target{Environment: "prod", App: "example", All: true, Service: "api"}, wantErr: "exactly one"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, services, err := application.resolveTarget(tt.target, tt.implicit, false)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("resolveTarget error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveTarget returned error: %v", err)
			}
			if got := strings.Join(services, ","); got != tt.want {
				t.Errorf("services = %q, want %q", got, tt.want)
			}
		})
	}
}
