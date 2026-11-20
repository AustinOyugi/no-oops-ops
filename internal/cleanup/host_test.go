package cleanup

import (
	"context"
	"fmt"
	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"github.com/AustinOyugi/no-oops-ops/internal/storage"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"
)

type hostRunner struct {
	calls []string
	fail  bool
	live  bool
}

func (r *hostRunner) Run(_ context.Context, _ string, args []string, _ command.RunOptions) (command.Result, error) {
	key := strings.Join(args, " ")
	r.calls = append(r.calls, key)
	var output string
	switch {
	case key == "service ls -q":
		if r.fail {
			return command.Result{}, fmt.Errorf("offline")
		}
		if r.live {
			output = "svc"
		}
	case strings.Contains(key, "PreviousSpec"):
		output = "registry/old@sha256:b"
	case strings.HasPrefix(key, "service inspect"):
		output = "prod-app_app|registry/live@sha256:a"
	case strings.HasPrefix(key, "service ps"):
		output = "Running 1 minute|registry/task@sha256:c"
	case strings.HasPrefix(key, "image inspect"):
		switch {
		case strings.HasSuffix(key, "sha256:a"):
			output = "sha256:live"
		case strings.HasSuffix(key, "sha256:b"):
			output = "sha256:old"
		case strings.HasSuffix(key, "sha256:c"):
			output = "sha256:task"
		default:
			return command.Result{}, fmt.Errorf("unexpected image inspection %s", key)
		}
	case key == "container ls --all --quiet":
		output = "failed running stopped"
	case strings.HasSuffix(key, " failed") && strings.HasPrefix(key, "container inspect"):
		if strings.Contains(key, "{{.Image}}") {
			output = "sha256:failed|exited|1"
		} else {
			output = "exited|1"
		}
	case strings.HasSuffix(key, " running") && strings.HasPrefix(key, "container inspect"):
		output = "sha256:running|running|0"
	case strings.HasSuffix(key, " stopped") && strings.HasPrefix(key, "container inspect"):
		output = "sha256:stopped|exited|0"
	case strings.HasPrefix(key, "image ls"):
		for _, id := range []string{"live", "old", "task", "failed", "running", "stopped", "unused"} {
			output += fmt.Sprintf("{\"ID\":\"sha256:%s\"}\n", id)
		}
	case strings.HasPrefix(key, "builder prune"), key == "container rm failed", key == "image rm sha256:unused":
	default:
		return command.Result{}, fmt.Errorf("unexpected command %s", key)
	}
	return command.Result{Output: []byte(output)}, nil
}
func TestHostCleanupProtectsLiveRollbackTasksAndContainers(t *testing.T) {
	r := &hostRunner{live: true}
	s := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), config.Config{StateDir: t.TempDir(), Storage: storage.Policy{}.Defaults()})
	s.runner = r
	plan, err := s.Run(context.Background(), Options{Host: true})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plan.LocalImages, []string{"sha256:unused"}) || !reflect.DeepEqual(plan.Containers, []string{"failed"}) {
		t.Fatalf("%+v", plan)
	}
	for _, call := range r.calls {
		if strings.Contains(call, " prune") || strings.Contains(call, " rm ") {
			t.Fatalf("dry run mutated: %s", call)
		}
	}
	r.calls = nil
	if _, err := s.Run(context.Background(), Options{Host: true, Apply: true}); err != nil {
		t.Fatal(err)
	}
	commands := strings.Join(r.calls, "\n")
	if !strings.Contains(commands, "container rm failed") || !strings.Contains(commands, "image rm sha256:unused") {
		t.Fatal(commands)
	}
	if strings.Contains(commands, "volume") || strings.Contains(commands, "--force sha256") || strings.Contains(commands, "system prune") {
		t.Fatal(commands)
	}
}
func TestHostCleanupFailsClosedWithoutInventory(t *testing.T) {
	r := &hostRunner{fail: true}
	s := NewService(nil, config.Config{StateDir: t.TempDir()})
	s.runner = r
	if _, err := s.Run(context.Background(), Options{Host: true, Apply: true}); err == nil {
		t.Fatal("accepted unknown liveness")
	}
	if len(r.calls) != 1 {
		t.Fatal(r.calls)
	}
}
func TestFailedContainerRequiresStoppedNonzeroExit(t *testing.T) {
	for value, want := range map[string]bool{"exited|1": true, "exited|0": false, "running|1": false, "dead|1": false, "exited|garbage": false} {
		if failedContainer(value) != want {
			t.Fatal(value)
		}
	}
}
