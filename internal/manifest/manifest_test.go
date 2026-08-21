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
