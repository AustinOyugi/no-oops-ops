package deploy

import (
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

func TestQuickRolloutMonitorUsesHealthcheckStartPeriod(t *testing.T) {
	monitor, err := quickRolloutMonitor(manifest.Manifest{Healthcheck: manifest.Healthcheck{
		StartPeriod: "60s",
	}})
	if err != nil {
		t.Fatalf("quickRolloutMonitor returned error: %v", err)
	}
	if monitor != "1m0s" {
		t.Errorf("quickRolloutMonitor = %q, want 1m0s", monitor)
	}
}
