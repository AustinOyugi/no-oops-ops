package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/state"
)

// operationJournal is a durable intent record. It prevents a process crash
// between Docker, ingress, and metadata mutations from being silently treated
// as a clean slate by the next command.
type operationJournal struct {
	Kind       string    `json:"kind"`
	Stage      string    `json:"stage"`
	StartedAt  time.Time `json:"started_at"`
	StackName  string    `json:"stack_name"`
	BlueGreen  bool      `json:"blue_green"`
	ReleaseTag string    `json:"release_tag"`
}

func journalPath(cfg config.Config, app, environment string) string {
	return filepath.Join(appDir(cfg, app, environment), "operation.json")
}

func loadJournal(cfg config.Config, app, environment string) (operationJournal, bool, error) {
	path := journalPath(cfg, app, environment)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return operationJournal{}, false, nil
	}
	if err != nil {
		return operationJournal{}, false, fmt.Errorf("read operation journal: %w", err)
	}
	var journal operationJournal
	if err := json.Unmarshal(data, &journal); err != nil {
		return operationJournal{}, false, fmt.Errorf("decode operation journal: %w", err)
	}
	return journal, true, nil
}

func saveJournal(cfg config.Config, app, environment string, journal operationJournal) error {
	data, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return fmt.Errorf("encode operation journal: %w", err)
	}
	return state.WriteFile(journalPath(cfg, app, environment), append(data, '\n'), 0o600)
}

func clearJournal(cfg config.Config, app, environment string) error {
	if err := os.Remove(journalPath(cfg, app, environment)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove operation journal: %w", err)
	}
	return nil
}

// recoverJournal removes an unpromoted blue/green candidate. In-place work
// can be retried only when Swarm confirms a stopped rollout before promotion.
func (s *Service) recoverJournal(ctx context.Context, app, environment string) error {
	journal, exists, err := loadJournal(s.config, app, environment)
	if err != nil || !exists {
		return err
	}
	if journal.Kind == "deploy" && journal.BlueGreen && journal.Stage != "ingress_reconciled" && journal.StackName != "" {
		if err := s.removeStack(ctx, journal.StackName); err != nil {
			return fmt.Errorf("recover interrupted blue/green deploy: %w", err)
		}
		return clearJournal(s.config, app, environment)
	}
	if journal.Kind == "deploy" && !journal.BlueGreen && journal.Stage == "stack_deployed" && journal.StackName == stackName(environment, app) {
		service := swarmServiceName(environment, app)
		status, message, _, err := s.serviceUpdateStatus(ctx, service)
		if err != nil {
			return fmt.Errorf("inspect unfinished deploy for %s/%s: %w", environment, app, err)
		}
		switch status {
		case "paused", "rollback_paused", "rollback_completed":
			// Preserve the intent record for diagnosis without blocking the next
			// attempt. This does not roll back or otherwise mutate the service.
			path := journalPath(s.config, app, environment)
			archive := path + ".failed-" + time.Now().UTC().Format("20060102-150405.000000000")
			if err := os.Rename(path, archive); err != nil {
				return fmt.Errorf("archive failed deployment journal: %w", err)
			}
			s.logger.WarnContext(ctx, "retrying stopped Swarm rollout", "service", service, "status", status, "reason", message, "journal_archive", archive)
			return nil
		}
	}
	return fmt.Errorf("unfinished %s operation for %s/%s at stage %q; inspect the live stack and journal %q before retrying", journal.Kind, environment, app, journal.Stage, journalPath(s.config, app, environment))
}
