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
	dir := filepath.Join(state, "apps", "lango", "dev")
	if err := os.MkdirAll(filepath.Join(dir, "releases"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "deployments"), 0o700); err != nil {
		t.Fatal(err)
	}
	for i, tag := range []string{"old", "previous", "active"} {
		created := time.Date(2026, 8, 1+i, 0, 0, 0, 0, time.UTC)
		writeJSON(t, filepath.Join(dir, "releases", tag+".json"), release.Metadata{App: "lango", Environment: "dev", Tag: tag, RegistryImage: "127.0.0.1:5000/lango:" + tag, CreateAt: created})
		writeJSON(t, filepath.Join(dir, "deployments", tag+".json"), deploy.Deployment{App: "lango", Environment: "dev", ReleaseTag: tag, ReleaseImage: "127.0.0.1:5000/lango:" + tag, Outcome: deploy.SwarmOutcomeCompleted, CreatedAt: created})
	}
	svc := NewService(nil, config.Config{StateDir: state})
	plan, err := svc.plan(map[string]struct{}{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.ReleasePaths) != 1 || len(plan.DeploymentPaths) != 1 || len(plan.Images) != 1 {
		t.Fatalf("plan = %#v, want one old release, deployment, and image", plan)
	}
	if plan.Images[0] != "127.0.0.1:5000/lango:old" {
		t.Errorf("image = %q", plan.Images[0])
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
