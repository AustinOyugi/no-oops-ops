package manifest

import "testing"

func TestImageShouldBuildDefaultsToTrue(t *testing.T) {
	if !(Image{}).ShouldBuild() {
		t.Fatal("image.build should default to true")
	}
}

func TestImageShouldBuildHonorsExplicitValue(t *testing.T) {
	for _, build := range []bool{true, false} {
		image := Image{Build: &build}
		if got := image.ShouldBuild(); got != build {
			t.Errorf("ShouldBuild() = %v, want %v", got, build)
		}
	}
}

func TestApplyDefaultsDerivesConservativeSwarmRolloutPolicy(t *testing.T) {
	m := Manifest{}
	m.applyDefaults()

	if got, want := m.Rollout.Monitor, "1m50s"; got != want {
		t.Errorf("rollout.monitor = %q, want %q", got, want)
	}
	if got, want := m.Rollout.ConvergenceTimeout, "2m"; got != want {
		t.Errorf("rollout.convergence_timeout = %q, want %q", got, want)
	}
	if got, want := m.Rollout.FailureAction, "rollback"; got != want {
		t.Errorf("rollout.failure_action = %q, want %q", got, want)
	}
	if got, want := m.Rollout.Rollback.Monitor, m.Rollout.Monitor; got != want {
		t.Errorf("rollout.rollback.monitor = %q, want %q", got, want)
	}
	if got, want := m.Rollout.Rollback.FailureAction, "pause"; got != want {
		t.Errorf("rollout.rollback.failure_action = %q, want %q", got, want)
	}
}
