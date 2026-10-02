package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/tui"
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
	if err := streamCommand(ctx, "/bin/sh", []string{"-c", "sleep 10"}, &out); err == nil {
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
		{[]string{"status"}, true}, {[]string{"release", "prod", "shop"}, true},
		{[]string{"secret", "set", "prod", "TOKEN"}, false}, {[]string{"secret", "list", "prod"}, true},
		{[]string{"upgrade", "--to", "latest"}, false}, {[]string{"upgrade", "--check"}, true},
		{[]string{"install"}, false}, {[]string{"deploy", "prod", "shop"}, false},
	} {
		args := append([]string{"--workspace", "missing"}, item.args...)
		if got := canStreamUIAction(config.Config{Workspace: "missing"}, tui.Action{Args: args}); got != item.want {
			t.Errorf("%v: %v", item.args, got)
		}
	}
}
