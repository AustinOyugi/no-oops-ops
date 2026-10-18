package ingressnet

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

type overlayRunner struct {
	output                string
	missing, denied, race bool
	creates               int
}

func (r *overlayRunner) Run(_ context.Context, _ string, args []string, _ command.RunOptions) (command.Result, error) {
	if args[1] == "create" {
		r.creates++
		r.missing = false
		if r.race {
			return command.Result{}, errors.New("already exists")
		}
		return command.Result{}, nil
	}
	if r.denied {
		return command.Result{Output: []byte("permission denied")}, errors.New("denied")
	}
	if r.missing {
		return command.Result{Output: []byte("network not found")}, errors.New("missing")
	}
	return command.Result{Output: []byte(r.output)}, nil
}
func TestEnsureOverlay(t *testing.T) {
	for _, tc := range []struct {
		name      string
		runner    overlayRunner
		wantError string
		creates   int
	}{
		{name: "existing", runner: overlayRunner{output: "overlay|swarm"}},
		{name: "missing", runner: overlayRunner{missing: true, output: "overlay|swarm"}, creates: 1},
		{name: "concurrent create", runner: overlayRunner{missing: true, race: true, output: "overlay|swarm"}, creates: 1},
		{name: "bridge", runner: overlayRunner{output: "bridge|local"}, wantError: "must be a Swarm overlay"},
		{name: "denied", runner: overlayRunner{denied: true}, wantError: "permission denied"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := EnsureOverlay(context.Background(), &tc.runner, "shared")
			if tc.wantError == "" && err != nil {
				t.Fatal(err)
			}
			if tc.wantError != "" && (err == nil || !strings.Contains(err.Error(), tc.wantError)) {
				t.Fatalf("error=%v", err)
			}
			if tc.runner.creates != tc.creates {
				t.Fatalf("creates=%d", tc.runner.creates)
			}
		})
	}
}
func TestEnsureOverlayRejectsInvalidName(t *testing.T) {
	for _, name := range []string{"", "--internal", "bad,name", "bad\nname"} {
		r := &overlayRunner{}
		if EnsureOverlay(context.Background(), r, name) == nil {
			t.Fatalf("accepted %q", name)
		}
	}
}
