package deploy

import (
	"testing"
	"time"
)

func TestFormatSwarmProgress(t *testing.T) {
	tests := []struct {
		name        string
		updateState string
		running     int
		desired     int
		monitoring  bool
		want        string
	}{
		{name: "starting", running: 1, desired: 2, want: "Starting tasks (1/2, 15s elapsed)"},
		{name: "updating", updateState: "updating", running: 1, desired: 2, want: "Rolling out: updating (1/2 tasks, 15s elapsed)"},
		{name: "monitoring", updateState: "updating", running: 2, desired: 2, monitoring: true, want: "Validating rollout (2/2 tasks, 15s/1m0s)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSwarmProgress(tt.updateState, tt.running, tt.desired, tt.monitoring, 15*time.Second, 15*time.Second, time.Minute); got != tt.want {
				t.Fatalf("formatSwarmProgress() = %q, want %q", got, tt.want)
			}
		})
	}
}
