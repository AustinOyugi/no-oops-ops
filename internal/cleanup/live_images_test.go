package cleanup

import "testing"

func TestHasRunningTask(t *testing.T) {
	if !hasRunningTask("Running 12 seconds ago\nPreparing 2 seconds ago\n") {
		t.Fatal("expected Running task to be detected")
	}
	if hasRunningTask("Shutdown 2 seconds ago\nPreparing 1 second ago\n") {
		t.Fatal("non-running tasks must not protect an image")
	}
}
