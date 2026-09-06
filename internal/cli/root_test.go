package cli

import (
	"context"
	"strings"
	"testing"
)

func TestRootCommandRendersReleaseHelp(t *testing.T) {
	root := NewRootCommand(context.Background())
	var output strings.Builder
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"release", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("release help returned error: %v", err)
	}
	for _, want := range []string{"--all", "--service", "--deploy", "<environment> <app>"} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("release help = %q, want %q", output.String(), want)
		}
	}
}

func TestTargetFlagsAreMutuallyExclusive(t *testing.T) {
	root := NewRootCommand(context.Background())
	root.SetArgs([]string{"release", "prod", "app", "--all", "--service", "api"})
	if err := root.Execute(); err == nil {
		t.Fatal("release with both selectors returned nil error")
	}
}
