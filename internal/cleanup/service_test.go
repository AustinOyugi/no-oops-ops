package cleanup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/deploy"
	"github.com/AustinOyugi/no-oops-ops/internal/release"
)

func TestPlanRetainsRollbackSafeHistory(t *testing.T) {
	state := t.TempDir()
	dir := filepath.Join(state, "apps", "sample", "dev")
	if err := os.MkdirAll(filepath.Join(dir, "releases"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "deployments"), 0o700); err != nil {
		t.Fatal(err)
	}
	for i, tag := range []string{"old", "previous", "active"} {
		created := time.Date(2026, 8, 1+i, 0, 0, 0, 0, time.UTC)
		writeJSON(t, filepath.Join(dir, "releases", tag+".json"), release.Metadata{App: "sample", Environment: "dev", Tag: tag, RegistryImage: "127.0.0.1:5000/sample:" + tag, CreateAt: created})
		writeJSON(t, filepath.Join(dir, "deployments", tag+".json"), deploy.Deployment{App: "sample", Environment: "dev", ReleaseTag: tag, ReleaseImage: "127.0.0.1:5000/sample:" + tag, Outcome: deploy.SwarmOutcomeCompleted, CreatedAt: created})
	}
	svc := NewService(nil, config.Config{StateDir: state})
	plan, err := svc.plan(liveInventory{services: map[string]struct{}{}, images: map[string]struct{}{}}, 2, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.ReleasePaths) != 1 || len(plan.DeploymentPaths) != 1 || len(plan.Images) != 1 || len(plan.LocalImages) != 1 {
		t.Fatalf("plan = %#v, want one old release, deployment, registry image, and local image", plan)
	}
	if plan.Images[0] != "127.0.0.1:5000/sample:old" {
		t.Errorf("image = %q", plan.Images[0])
	}
}

func TestPlanSelectsEntireOrphanedEnvironment(t *testing.T) {
	state := t.TempDir()
	dir := filepath.Join(state, "apps", "sample", "dev")
	if err := os.MkdirAll(filepath.Join(dir, "releases"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "deployments"), 0o700); err != nil {
		t.Fatal(err)
	}
	image := "127.0.0.1:5000/sample:active"
	writeJSON(t, filepath.Join(dir, "releases", "active.json"), release.Metadata{App: "sample", Environment: "dev", Tag: "active", RegistryImage: image, CreateAt: time.Now()})
	writeJSON(t, filepath.Join(dir, "deployments", "active.json"), deploy.Deployment{App: "sample", Environment: "dev", ReleaseImage: image, ServiceName: "dev-sample_app", Outcome: deploy.SwarmOutcomeCompleted, CreatedAt: time.Now()})
	plan, err := NewService(nil, config.Config{StateDir: state}).plan(liveInventory{services: map[string]struct{}{}, images: map[string]struct{}{}}, 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.ReleasePaths) != 1 || len(plan.DeploymentPaths) != 1 || len(plan.Images) != 0 || len(plan.LocalImages) != 0 {
		t.Fatalf("orphan plan = %#v, want only stale history selected", plan)
	}
}

func TestPlanProtectsLiveImageWithoutMatchingHistoricalServiceName(t *testing.T) {
	state := t.TempDir()
	dir := filepath.Join(state, "apps", "sample", "dev")
	if err := os.MkdirAll(filepath.Join(dir, "releases"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "deployments"), 0o700); err != nil {
		t.Fatal(err)
	}
	image := "127.0.0.1:5000/sample:current"
	writeJSON(t, filepath.Join(dir, "releases", "current.json"), release.Metadata{App: "sample", Environment: "dev", Tag: "current", RegistryImage: image, CreateAt: time.Now()})
	// The historical name is deliberately stale: image liveness is not derived
	// from it. A retained Swarm image must still be safe from cleanup.
	writeJSON(t, filepath.Join(dir, "deployments", "current.json"), deploy.Deployment{App: "sample", Environment: "dev", ReleaseImage: image, ServiceName: "old-service-name", Outcome: deploy.SwarmOutcomeCompleted, CreatedAt: time.Now()})
	live := liveInventory{services: map[string]struct{}{"dev-sample_dev-sample": {}}, images: map[string]struct{}{imageKey(image + "@sha256:abc"): {}}}
	plan, err := NewService(nil, config.Config{StateDir: state}).plan(live, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Images) != 0 || len(plan.ReleasePaths) != 0 {
		t.Fatalf("live image must not be selected: %#v", plan)
	}
}

func TestPlanOrphanedChecksServiceNameOnlyForHistoryCleanup(t *testing.T) {
	state := t.TempDir()
	dir := filepath.Join(state, "apps", "sample", "dev")
	if err := os.MkdirAll(filepath.Join(dir, "releases"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "deployments"), 0o700); err != nil {
		t.Fatal(err)
	}
	image := "127.0.0.1:5000/sample:current"
	writeJSON(t, filepath.Join(dir, "releases", "current.json"), release.Metadata{App: "sample", Environment: "dev", Tag: "current", RegistryImage: image, CreateAt: time.Now()})
	writeJSON(t, filepath.Join(dir, "deployments", "current.json"), deploy.Deployment{App: "sample", Environment: "dev", ReleaseImage: image, ServiceName: "dev-sample_dev-sample", Outcome: deploy.SwarmOutcomeCompleted, CreatedAt: time.Now()})
	live := liveInventory{services: map[string]struct{}{"dev-sample_dev-sample": {}}, images: map[string]struct{}{image: {}}}
	plan, err := NewService(nil, config.Config{StateDir: state}).plan(live, 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.ReleasePaths) != 0 || len(plan.DeploymentPaths) != 0 || len(plan.Images) != 0 {
		t.Fatalf("running service must not be treated as orphaned: %#v", plan)
	}
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
