package deploy

import (
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/state"
)

// RolloutProgress is display telemetry, independent of the recovery journal.
// Traffic records ingress reconciliation, not an HTTP traffic probe.
type RolloutProgress struct {
	FailedStage string    `json:"failed_stage,omitempty"`
	App         string    `json:"app"`
	Environment string    `json:"environment"`
	OldService  string    `json:"old_service"`
	NewService  string    `json:"new_service"`
	OldRelease  string    `json:"old_release"`
	NewRelease  string    `json:"new_release"`
	Stage       string    `json:"stage"`
	Traffic     string    `json:"traffic"`
	Error       string    `json:"error,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Finished    bool      `json:"finished"`
}

func saveRolloutProgress(cfg config.Config, p RolloutProgress) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return state.WriteFile(filepath.Join(appDir(cfg, p.App, p.Environment), "rollout.json"), append(data, '\n'), 0600)
}
