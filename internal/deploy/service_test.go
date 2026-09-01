package deploy

import (
	"testing"
)

func TestStaleAppStacks(t *testing.T) {
	stableStack := "prod-vybe-builder-service"
	services := `
prod-vybe-builder-service_prod-vybe-builder-service
prod-vybe-builder-service-r20260827-025448-17878-8635fb22e1_app
prod-vybe-builder-service-r20260901-025813-17882-e7c96d8c40_app
prod-vybe-builder-worker_runnable
dev-vybe-builder-service_dev-vybe-builder-service
prod-vybe-builder-service-other_app
`

	got := staleAppStacks(services, "prod", "vybe-builder-service", stableStack)
	want := []string{
		"prod-vybe-builder-service-r20260827-025448-17878-8635fb22e1",
		"prod-vybe-builder-service-r20260901-025813-17882-e7c96d8c40",
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
