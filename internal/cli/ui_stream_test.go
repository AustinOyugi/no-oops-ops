package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/tui"
	"github.com/AustinOyugi/no-oops-ops/internal/workspace"
)

func TestStreamingCapturesOutputAndFailure(t *testing.T) {
	var out bytes.Buffer
	err := streamCommand(context.Background(), "/bin/sh", []string{"-c", "echo stdout; echo stderr >&2; exit 7"}, &out)
	if err == nil || !strings.Contains(err.Error(), "7") {
		t.Fatalf("exit status: %v", err)
	}
	if !strings.Contains(out.String(), "stdout") || !strings.Contains(out.String(), "stderr") {
		t.Fatalf("output: %q", out.String())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	if err := streamCommand(ctx, "/bin/sh", []string{"-c", "exec sleep 10"}, &out); err == nil {
		t.Fatal("cancellation succeeded unexpectedly")
	}
	if time.Since(started) > 3*time.Second {
		t.Fatal("cancellation left subprocess waiting")
	}
}
func TestStreamingRouting(t *testing.T) {
	for _, item := range []struct {
		args []string
		want bool
	}{
		{[]string{"logs"}, true}, {[]string{"logs", "shop", "--service", "api"}, true}, {[]string{"status"}, true}, {[]string{"release", "shop"}, true},
		{[]string{"secret", "set", "TOKEN"}, false}, {[]string{"secret", "list"}, true},
		{[]string{"upgrade", "--to", "latest"}, false}, {[]string{"upgrade", "--check"}, true},
		{[]string{"install"}, false}, {[]string{"deploy", "shop"}, false},
	} {
		args := append([]string{"--workspace", "missing", "--environment", "prod"}, item.args...)
		if got := canStreamUIAction(config.Config{Workspace: "missing"}, tui.Action{Args: args}); got != item.want {
			t.Errorf("%v: %v", item.args, got)
		}
	}
}

func TestStreamingCancellationAllowsCleanup(t *testing.T) {
	root := t.TempDir()
	ready, cleaned := filepath.Join(root, "ready"), filepath.Join(root, "cleaned")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	var out bytes.Buffer
	go func() {
		done <- streamCommand(ctx, "/bin/sh", []string{"-c", `trap 'echo cleaned > "$2"; exit 0' TERM; echo ready > "$1"; while :; do sleep 0.05; done`, "sh", ready, cleaned}, &out)
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("child did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("cleanup did not finish")
	}
	if _, err := os.Stat(cleaned); err != nil {
		t.Fatalf("child cleanup bypassed: %v", err)
	}
}

func TestStreamingDeploymentUsesFlagEnvironmentForTLS(t *testing.T) {
	root := t.TempDir()
	if _, err := workspace.Initialize(root, config.Version); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"apps.yml":    "version: " + config.Version + "\napps:\n  shop:\n    manifest: compose.yml\n",
		"compose.yml": "services:\n  api:\n    image: nginx:alpine\n    x-noops:\n      service: {internal_port: 80}\n      ingress:\n        environments:\n          prod: {domain: api.example.com, tls: true}\n",
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		environment string
		want        bool
	}{{"prod", false}, {"dev", true}} {
		for _, command := range [][]string{{"deploy", "shop", "--service", "api"}, {"release", "shop", "--service", "api", "--deploy"}} {
			args := append([]string{"--workspace", root, "--environment", test.environment}, command...)
			if got := canStreamUIAction(config.Config{Workspace: root}, tui.Action{Args: args}); got != test.want {
				t.Fatalf("%v stream=%v, want %v", args, got, test.want)
			}
		}
	}
}
