package deploy

import (
	"testing"
	"time"
)

func TestAllDesiredTasksRunningRequiresAllDesiredTasks(t *testing.T) {
	if allDesiredTasksRunning(1, 3) {
		t.Error("1 running task should not satisfy 3 desired tasks")
	}
	if !allDesiredTasksRunning(3, 3) {
		t.Error("3 running tasks should satisfy 3 desired tasks")
	}
}

func TestParseServiceUpdateStatus(t *testing.T) {
	state, message, image := parseServiceUpdateStatus("rollback_completed|rollback completed|registry/sample:v1\n")
	if state != "rollback_completed" || message != "rollback completed" || image != "registry/sample:v1" {
		t.Errorf("parseServiceUpdateStatus() = (%q, %q, %q), want (%q, %q, %q)", state, message, image, "rollback_completed", "rollback completed", "registry/sample:v1")
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

func TestRolloutMonitorAllowsFullMonitorAfterConvergence(t *testing.T) {
	started := time.Date(2026, time.September, 6, 13, 50, 40, 0, time.UTC)
	state := newRolloutMonitorState(started, 90*time.Second, 90*time.Second)

	if timedOut, monitoring, completed, _ := state.observe(started.Add(60*time.Second), true); timedOut || !monitoring || completed {
		t.Fatalf("first convergence observation = timedOut:%t monitoring:%t completed:%t, want false:true:false", timedOut, monitoring, completed)
	}
	if timedOut, monitoring, completed, _ := state.observe(started.Add(90*time.Second), true); timedOut || !monitoring || completed {
		t.Fatalf("at the old convergence deadline = timedOut:%t monitoring:%t completed:%t, want false:true:false", timedOut, monitoring, completed)
	}
	if timedOut, monitoring, completed, _ := state.observe(started.Add(149*time.Second), true); timedOut || !monitoring || completed {
		t.Fatalf("before the full monitor window = timedOut:%t monitoring:%t completed:%t, want false:true:false", timedOut, monitoring, completed)
	}
	if timedOut, monitoring, completed, _ := state.observe(started.Add(150*time.Second), true); timedOut || !monitoring || !completed {
		t.Fatalf("after the full monitor window = timedOut:%t monitoring:%t completed:%t, want false:true:true", timedOut, monitoring, completed)
	}
}

func TestRolloutMonitorTimesOutBeforeConvergence(t *testing.T) {
	started := time.Date(2026, time.September, 6, 13, 50, 40, 0, time.UTC)
	state := newRolloutMonitorState(started, 90*time.Second, 30*time.Second)

	if timedOut, _, _, _ := state.observe(started.Add(91*time.Second), false); !timedOut {
		t.Fatal("expected convergence timeout before any successful convergence")
	}
}
