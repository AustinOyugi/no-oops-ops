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
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestActiveRolloutHidesTasksAndRestoresThemWhenFinished(t *testing.T) {
	d := newDashboard(context.Background(), nil, nil)
	hasPane := func(p tview.Primitive) bool {
		for i := 0; i < d.layout.GetItemCount(); i++ {
			if d.layout.GetItem(i) == p {
				return true
			}
		}
		return false
	}
	d.app.SetFocus(d.tasks)
	d.rollouts = []deploy.RolloutProgress{{App: "api", Stage: "waiting_readiness"}}
	d.updateRolloutPane()
	if hasPane(d.tasks) || hasPane(d.detail) || !hasPane(d.rolloutPane) {
		t.Fatal("active rollout must replace tasks and detail with rollout progress")
	}
	if d.app.GetFocus() != d.services {
		t.Fatal("focus remained on hidden tasks")
	}
	d.app.GetInputCapture()(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if d.app.GetFocus() == d.tasks {
		t.Fatal("Tab focused hidden tasks")
	}
	d.rollouts[0].Finished = true
	d.rollouts[0].Stage = "completed"
	d.updateRolloutPane()
	if !hasPane(d.tasks) || !hasPane(d.detail) {
		t.Fatal("finished rollout did not restore tasks")
	}
	// A different, still-active rollout continues to suppress the tasks panel.
	d.rollouts = append(d.rollouts, deploy.RolloutProgress{App: "worker"})
	d.updateRolloutPane()
	if hasPane(d.tasks) {
		t.Fatal("another active rollout restored tasks")
	}
	d.rollouts = nil
	d.updateRolloutPane()
	if !hasPane(d.tasks) || hasPane(d.rolloutPane) {
		t.Fatal("clearing rollout did not restore normal layout")
	}
}

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
