package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

func TestDeterministicTagUsesGitSHAAndBuildHash(t *testing.T) {
	root := t.TempDir()
	dockerfile := filepath.Join(root, "Dockerfile")
	if err := os.WriteFile(dockerfile, []byte("FROM scratch\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := manifest.Manifest{Name: "api", Image: manifest.Image{Repository: "example/api"}, Source: manifest.Source{Context: ".", Dockerfile: "Dockerfile"}}
	git := &GitMetadata{Commit: "0123456789abcdef0123456789abcdef01234567"}
	first, err := deterministicTag(m, "prod", root, dockerfile, git)
	if err != nil {
		t.Fatal(err)
	}
	second, err := deterministicTag(m, "prod", root, dockerfile, git)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Errorf("tag is not deterministic: %q != %q", first, second)
	}
	if !strings.HasPrefix(first, "sha-0123456789ab-") || len(first) != len("sha-")+12+1+10 {
		t.Errorf("tag %q is not conventional sha format", first)
	}
}

func TestDeterministicTagUsesContextWhenGitIsUnavailable(t *testing.T) {
	root := t.TempDir()
	dockerfile := filepath.Join(root, "Dockerfile")
	if err := os.WriteFile(dockerfile, []byte("FROM scratch\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := manifest.Manifest{Name: "api", Image: manifest.Image{Repository: "example/api"}, Source: manifest.Source{Context: ".", Dockerfile: "Dockerfile"}}
	tag, err := deterministicTag(m, "prod", root, dockerfile, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(tag, "ctx-") || len(tag) != len("ctx-")+12+1+10 {
		t.Errorf("tag %q is not conventional context format", tag)
	}
}
