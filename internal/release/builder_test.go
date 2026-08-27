package release

import (
	"reflect"
	"testing"
)

func TestSortedBuildArgKeys(t *testing.T) {
	got := sortedBuildArgKeys(map[string]string{"ZETA": "z", "ALPHA": "a", "MID": "m"})
	want := []string{"ALPHA", "MID", "ZETA"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sortedBuildArgKeys() = %v, want %v", got, want)
	}
}
