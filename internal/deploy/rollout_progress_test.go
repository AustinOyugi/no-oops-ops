package deploy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
)

func TestRolloutTelemetryDoesNotReplaceRecoveryJournal(t *testing.T) {
	cfg := config.Config{StateDir: t.TempDir()}
	p := RolloutProgress{App: "api", Environment: "prod", OldService: "old_app", NewService: "new_app", Stage: "promoting", Traffic: "unknown", StartedAt: time.Now(), UpdatedAt: time.Now()}
	if err := saveRolloutProgress(cfg, p); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(appDir(cfg, "api", "prod"), "rollout.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got RolloutProgress
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Traffic != "unknown" || got.NewService != "new_app" {
		t.Fatalf("telemetry: %+v", got)
	}
	if _, err := os.Stat(journalPath(cfg, "api", "prod")); !os.IsNotExist(err) {
		t.Fatal("telemetry altered recovery journal")
	}
}
