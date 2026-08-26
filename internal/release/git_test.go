package release

import (
	"testing"
)

func TestGitFailurePrefersFatalLine(t *testing.T) {
	got := gitFailure([]byte("pull output\nfatal: could not read Username for 'https://github.com': No such device or address\n"))
	if got != "Git credential was rejected or does not have access to the repository" {
		t.Fatalf("gitFailure = %q", got)
	}
}

func TestResolveGitSourcePathStaysInCheckout(t *testing.T) {
	base := t.TempDir()
	if _, err := resolveGitSourcePath(base, "../outside"); err == nil {
		t.Fatal("expected path escape to be rejected")
	}
	if _, err := resolveGitSourcePath(base, "Dockerfile"); err != nil {
		t.Fatalf("relative Git source path should be allowed: %v", err)
	}
}
