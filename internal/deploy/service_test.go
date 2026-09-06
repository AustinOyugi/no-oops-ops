package deploy

import (
	"testing"
)

func TestStaleAppStacks(t *testing.T) {
	stableStack := "prod-sample-builder-service"
	services := `
prod-sample-builder-service_prod-sample-builder-service
prod-sample-builder-service-r20260827-025448-17878-8635fb22e1_app
prod-sample-builder-service-r20260901-025813-17882-e7c96d8c40_app
prod-sample-builder-worker_runnable
dev-sample-builder-service_dev-sample-builder-service
prod-sample-builder-service-other_app
`

	got := staleAppStacks(services, "prod", "sample-builder-service", stableStack)
	want := []string{
		"prod-sample-builder-service-r20260827-025448-17878-8635fb22e1",
		"prod-sample-builder-service-r20260901-025813-17882-e7c96d8c40",
	}
	if len(got) != len(want) {
		t.Fatalf("staleAppStacks() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("staleAppStacks()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
