package deploy

import (
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

func TestQuickRolloutDurationsUseHealthcheckStartPeriod(t *testing.T) {
	monitor, timeout, err := quickRolloutDurations(manifest.Manifest{Healthcheck: manifest.Healthcheck{
		StartPeriod: "60s",
	}})
	if err != nil {
		t.Fatalf("quickRolloutDurations returned error: %v", err)
	}
	if monitor != "1m0s" || timeout != "1m10s" {
		t.Errorf("quickRolloutDurations = (%q, %q), want (1m0s, 1m10s)", monitor, timeout)
	}
}

func TestIsActiveReleaseStack(t *testing.T) {
	expected := releaseStackName("dev", "lango", "20260824-133728")
	active := Deployment{ReleaseTag: "20260824-133728", StackName: expected}
	if !isActiveReleaseStack(active, "20260824-133728", expected) {
		t.Fatal("expected matching promoted release stack to be reusable")
	}
	if isActiveReleaseStack(active, "20260824-133729", releaseStackName("dev", "lango", "20260824-133729")) {
		t.Fatal("a different release must not reuse the active release stack")
	}
	if isActiveReleaseStack(Deployment{ReleaseTag: active.ReleaseTag, StackName: "dev-lango"}, active.ReleaseTag, expected) {
		t.Fatal("the stable stack must not be treated as a release-specific stack")
	}
}
