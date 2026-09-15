package cleanup

import "testing"

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
