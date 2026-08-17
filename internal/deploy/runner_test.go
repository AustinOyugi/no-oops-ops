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

func TestHealthChecksSatisfiedRequiresEveryContainerToBeHealthy(t *testing.T) {
	for _, test := range []struct {
		name     string
		statuses []string
		desired  int
		want     bool
	}{
		{name: "all healthy", statuses: []string{"healthy", "healthy"}, desired: 2, want: true},
		{name: "not enough containers", statuses: []string{"healthy"}, desired: 2, want: false},
		{name: "starting", statuses: []string{"healthy", "starting"}, desired: 2, want: false},
		{name: "unhealthy", statuses: []string{"unhealthy"}, desired: 1, want: false},
		{name: "missing healthcheck", statuses: []string{"missing"}, desired: 1, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := healthChecksSatisfied(test.statuses, test.desired); got != test.want {
				t.Errorf("healthChecksSatisfied(%v, %d) = %t, want %t", test.statuses, test.desired, got, test.want)
			}
		})
	}
}

func TestOutputLines(t *testing.T) {
	got := outputLines("\nabc\n\ndef\n")
	if len(got) != 2 || got[0] != "abc" || got[1] != "def" {
		t.Errorf("outputLines = %v", got)
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
