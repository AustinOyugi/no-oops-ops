package deploy

import "testing"

func TestReadinessSatisfiedRequiresAllDesiredTasks(t *testing.T) {
	if readinessSatisfied(1, 3) {
		t.Error("1 running task should not satisfy 3 desired tasks")
	}
	if !readinessSatisfied(3, 3) {
		t.Error("3 running tasks should satisfy 3 desired tasks")
	}
}

func TestParseTaskDiagnostics(t *testing.T) {
	diagnostics := parseTaskDiagnostics("abc123|node-1|Running|Rejected 4 seconds ago|No such image\n")
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %d, want 1", len(diagnostics))
	}

	got := diagnostics[0]
	if got.ID != "abc123" || got.Node != "node-1" || got.DesiredState != "Running" || got.CurrentState != "Rejected 4 seconds ago" || got.Error != "No such image" {
		t.Errorf("diagnostic = %#v", got)
	}
}
