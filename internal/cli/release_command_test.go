package cli

import (
	"context"
	"testing"
)

func TestReleaseForceFlag(t *testing.T) {
	for _, flag := range []string{"--force", "-f"} {
		cmd := newReleaseCommand(context.Background(), nil)
		if err := cmd.ParseFlags([]string{flag, "--deploy", "--all"}); err != nil {
			t.Fatal(err)
		}
		force, err := cmd.Flags().GetBool("force")
		if err != nil || !force {
			t.Fatalf("%s did not enable force: %v", flag, err)
		}
	}
	cmd := newReleaseCommand(context.Background(), nil)
	force, err := cmd.Flags().GetBool("force")
	if err != nil || force {
		t.Fatalf("force should default off: %v", err)
	}
}
