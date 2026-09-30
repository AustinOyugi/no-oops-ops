package cleanup

import (
	"context"
	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"strings"
	"testing"
)

func TestTerminalTaskState(t *testing.T) {
	if terminalTaskState("Running 12 seconds ago") {
		t.Fatal("running task must protect its image")
	}
	if terminalTaskState("Preparing 2 seconds ago") {
		t.Fatal("a task being scheduled must protect its image")
	}
	if !terminalTaskState("Shutdown 2 seconds ago") {
		t.Fatal("shutdown task must not protect its image")
	}
}

func TestImageKeyRemovesDigestButRetainsTag(t *testing.T) {
	if got, want := imageKey("127.0.0.1:5000/sample:v1@sha256:abc"), "127.0.0.1:5000/sample:v1"; got != want {
		t.Fatalf("imageKey = %q, want %q", got, want)
	}
}

func TestLocalImageInUse(t *testing.T) {
	if !localImageInUse("conflict: unable to delete image (must be forced) - container abc is using its referenced image def") {
		t.Fatal("container image conflict should be a non-fatal cache cleanup result")
	}
}

func TestLiveInventoryProtectsSwarmRollbackImage(t *testing.T) {
	s := NewService(nil, config.Config{})
	s.runner = inventoryRunner{}
	live, err := s.liveServices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, image := range []string{"registry/api:current", "registry/api:previous"} {
		if _, ok := live.images[image]; !ok {
			t.Fatalf("unprotected image %s: %#v", image, live)
		}
	}
}

type inventoryRunner struct{}

func (inventoryRunner) Run(_ context.Context, _ string, args []string, _ command.RunOptions) (command.Result, error) {
	output := ""
	switch {
	case args[1] == "ls":
		output = "service-id"
	case args[1] == "inspect" && strings.Contains(args[3], "PreviousSpec"):
		output = "registry/api:previous"
	case args[1] == "inspect":
		output = "prod-api|registry/api:current"
	}
	return command.Result{Output: []byte(output)}, nil
}
