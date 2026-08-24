package deploy

import "testing"

func TestFormatSwarmProgress(t *testing.T) {
	tests := []struct {
		name        string
		updateState string
		running     int
		desired     int
		monitoring  bool
		want        string
	}{
		{name: "starting", running: 1, desired: 2, want: "Starting tasks (1/2)"},
		{name: "updating", updateState: "updating", running: 1, desired: 2, want: "Rolling out: updating (1/2 tasks)"},
		{name: "monitoring", updateState: "updating", running: 2, desired: 2, monitoring: true, want: "Validating rollout (2/2 tasks)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSwarmProgress(tt.updateState, tt.running, tt.desired, tt.monitoring); got != tt.want {
				t.Fatalf("formatSwarmProgress() = %q, want %q", got, tt.want)
			}
		})
	}
}
