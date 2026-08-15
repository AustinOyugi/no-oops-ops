package deploy

import (
	"strings"
	"testing"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
)

func TestFilesystemDeploymentStorePreviousReturnsDeploymentBeforeLatest(t *testing.T) {
	store := newFilesystemDeploymentStore()
	cfg := config.Config{StateDir: t.TempDir()}
	first := time.Date(2026, 8, 14, 8, 0, 0, 0, time.UTC)
	second := first.Add(time.Minute)

	for _, deployment := range []Deployment{
		{App: "lango", Environment: "prod", CreatedAt: first, ReleaseTag: "first", ReleaseImage: "registry/lango:first"},
		{App: "lango", Environment: "prod", CreatedAt: second, ReleaseTag: "second", ReleaseImage: "registry/lango:second"},
	} {
		if _, err := store.Save(cfg, deployment); err != nil {
			t.Fatalf("save deployment: %v", err)
		}
	}

	previous, err := store.Previous(cfg, "lango", "prod")
	if err != nil {
		t.Fatalf("find previous deployment: %v", err)
	}

	if previous.ReleaseTag != "first" {
		t.Fatalf("previous release tag = %q, want %q", previous.ReleaseTag, "first")
	}
}

func TestFilesystemDeploymentStorePreviousRequiresTwoDeployments(t *testing.T) {
	store := newFilesystemDeploymentStore()
	cfg := config.Config{StateDir: t.TempDir()}

	if _, err := store.Save(cfg, Deployment{App: "lango", Environment: "prod", CreatedAt: time.Now().UTC(), ReleaseTag: "only"}); err != nil {
		t.Fatalf("save deployment: %v", err)
	}

	_, err := store.Previous(cfg, "lango", "prod")
	if err == nil || !strings.Contains(err.Error(), "at least two successful deployments") {
		t.Fatalf("previous deployment error = %v, want insufficient history error", err)
	}
}

func TestDeploymentIDUsesSecondPrecision(t *testing.T) {
	createdAt := time.Date(2026, 8, 15, 1, 47, 3, 78954000, time.FixedZone("EAT", 3*60*60))

	if got, want := deploymentID(createdAt), "20260814-224703"; got != want {
		t.Errorf("deploymentID() = %q, want %q", got, want)
	}
}
