package main

import "testing"

func TestIsVersionCommand(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"--version"}, {"-v"}} {
		if !isVersionCommand(args) {
			t.Errorf("isVersionCommand(%q) = false, want true", args)
		}
	}
	if isVersionCommand([]string{"--workspace", "/tmp/workspace", "--version"}) {
		t.Error("workspace-scoped version invocation must not be treated as a standalone version command")
	}
}
