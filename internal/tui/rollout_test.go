package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/deploy"
)

func TestRolloutShowsActualIngressAndReplicaState(t *testing.T) {
	p := deploy.RolloutProgress{App: "web", Environment: "prod", OldService: "old_app", NewService: "new_app", Stage: "promoting", Traffic: "unknown", UpdatedAt: time.Now()}
	text := rolloutText(p, []Row{{Service: "old_app", Replicas: "1/1", State: "running"}, {Service: "new_app", Replicas: "1/1", State: "running"}})
	for _, word := range []string{"OLD", "NEW", "1/1 running", "handoff unresolved", "→ Promote"} {
		if !strings.Contains(text, word) {
			t.Fatalf("missing %s: %s", word, text)
		}
	}
	p.Stage = "failed"
	p.FailedStage = "promoting"
	p.Error = "reload failed"
	if !strings.Contains(rolloutText(p, nil), "! Promote") {
		t.Fatal("failed stage not shown")
	}
	p.Stage = "completed"
	p.FailedStage = ""
	p.Error = ""
	p.Traffic = "new"
	p.Finished = true
	if !strings.Contains(rolloutText(p, nil), "no HTTP traffic probe") {
		t.Fatal("claimed traffic verification")
	}
}
func TestRolloutPollingIgnoresOldCompletion(t *testing.T) {
	cfg := config.Config{StateDir: t.TempDir()}
	dir := filepath.Join(cfg.StateDir, "apps", "web", "prod")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rollout.json"), []byte(`{"app":"web","stage":"completed","finished":true,"updated_at":"2020-01-01T00:00:00Z"}`), 0600); err != nil {
		t.Fatal(err)
	}
	rows, err := Rollouts(context.Background(), cfg)
	if err != nil || len(rows) != 0 {
		t.Fatalf("old rollout visible: %v %v", rows, err)
	}
}
