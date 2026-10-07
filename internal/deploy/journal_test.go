package deploy

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
)

func TestRecoverInPlaceJournalChecksLiveRollout(t *testing.T) {
	for _, tc := range []struct {
		name, status, stage   string
		inspectFails, recover bool
	}{
		{name: "paused", status: "paused", stage: "stack_deployed", recover: true},
		{name: "rollback paused", status: "rollback_paused", stage: "stack_deployed", recover: true},
		{name: "rolled back", status: "rollback_completed", stage: "stack_deployed", recover: true},
		{name: "updating", status: "updating", stage: "stack_deployed"},
		{name: "rolling back", status: "rollback_started", stage: "stack_deployed"},
		{name: "completed needs reconciliation", status: "completed", stage: "stack_deployed"},
		{name: "unknown", stage: "stack_deployed"},
		{name: "inspect fails", stage: "stack_deployed", inspectFails: true},
		{name: "promotion uncertain", status: "paused", stage: "ingress_reconciled"},
		{name: "started uncertain", status: "paused", stage: "started"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin := t.TempDir()
			script := "#!/bin/sh\n[ \"$1 $2\" = 'service inspect' ] || exit 2\n[ \"$5\" = 'prod-api_prod-api' ] || exit 3\nprintf '%s|test failure|image\\n' \"$TEST_ROLLOUT_STATUS\"\n"
			if tc.inspectFails {
				script = "#!/bin/sh\nexit 1\n"
			}
			if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("TEST_ROLLOUT_STATUS", tc.status)
			cfg := config.Config{StateDir: t.TempDir()}
			journal := operationJournal{Kind: "deploy", Stage: tc.stage, StackName: "prod-api", ReleaseTag: "release"}
			if err := saveJournal(cfg, "api", "prod", journal); err != nil {
				t.Fatal(err)
			}
			s := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)
			err := s.recoverJournal(context.Background(), "api", "prod")
			if (err == nil) != tc.recover {
				t.Fatalf("recoverJournal error = %v, want recovery %t", err, tc.recover)
			}
			_, exists, err := loadJournal(cfg, "api", "prod")
			if err != nil || exists == tc.recover {
				t.Fatalf("journal exists=%t err=%v", exists, err)
			}
			archives, err := filepath.Glob(journalPath(cfg, "api", "prod") + ".failed-*")
			if err != nil {
				t.Fatal(err)
			}
			if tc.recover {
				if len(archives) != 1 {
					t.Fatalf("archives = %v", archives)
				}
				data, err := os.ReadFile(archives[0])
				if err != nil || !strings.Contains(string(data), "release") {
					t.Fatalf("archived journal missing: %v", err)
				}
			} else if len(archives) != 0 {
				t.Fatalf("unexpected archives = %v", archives)
			}
		})
	}
}

func TestJournalRoundTripAndClear(t *testing.T) {
	cfg := config.Config{StateDir: t.TempDir()}
	want := operationJournal{Kind: "deploy", Stage: "stack_deployed", StartedAt: time.Date(2026, 9, 29, 1, 2, 3, 0, time.UTC), StackName: "prod-api", BlueGreen: true, ReleaseTag: "release"}
	if err := saveJournal(cfg, "api", "prod", want); err != nil {
		t.Fatal(err)
	}
	got, exists, err := loadJournal(cfg, "api", "prod")
	if err != nil || !exists {
		t.Fatalf("loadJournal = (%#v, %t, %v)", got, exists, err)
	}
	if got != want {
		t.Errorf("journal = %#v, want %#v", got, want)
	}
	if err := clearJournal(cfg, "api", "prod"); err != nil {
		t.Fatal(err)
	}
	if _, exists, err := loadJournal(cfg, "api", "prod"); err != nil || exists {
		t.Fatalf("journal remains: exists=%t err=%v", exists, err)
	}
}

func TestCleanupCancelledDeploy(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		blueGreen, previous, fail bool
		stage                     string
	}{
		{name: "candidate", blueGreen: true, stage: "stack_deployed"},
		{name: "rollback", previous: true, stage: "stack_deployed"},
		{name: "first deployment", stage: "stack_deployed"},
		{name: "cancelled during stack deploy", stage: "started"},
		{name: "cleanup failure", fail: true, stage: "stack_deployed"},
		{name: "promoted candidate retained", blueGreen: true, stage: "ingress_reconciled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			script := "#!/bin/sh\ncase \"$1 $2\" in\n'stack ls') echo prod-api;;\n'stack rm') exit 0;;\n'service inspect') case \"$4\" in\n*PreviousSpec*) echo false;;\n*) echo 'completed|0';;\nesac;;\n'service scale'|'service update') exit 0;;\n*) exit 1;;\nesac\n"
			if tc.previous {
				script = strings.ReplaceAll(script, "echo false", "echo true")
				script = strings.ReplaceAll(script, "completed|0", "rollback_completed|1")
			}
			if tc.fail {
				script = "#!/bin/sh\nexit 1\n"
			}
			if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			cfg := config.Config{StateDir: t.TempDir()}
			journal := operationJournal{Kind: "deploy", Stage: tc.stage, StackName: "prod-api", BlueGreen: tc.blueGreen}
			if err := saveJournal(cfg, "api", "prod", journal); err != nil {
				t.Fatal(err)
			}
			s := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)
			parent, cancel := context.WithCancel(context.Background())
			cancel()
			recovery, finish := context.WithTimeout(context.WithoutCancel(parent), time.Second)
			defer finish()
			err := s.cleanupCancelledDeploy(recovery, "api", "prod", "prod-api_prod-api", journal)
			wantFailure := tc.fail || tc.stage == "ingress_reconciled"
			if (err != nil) != wantFailure {
				t.Fatalf("cleanup error=%v", err)
			}
			_, exists, loadErr := loadJournal(cfg, "api", "prod")
			if loadErr != nil || exists != wantFailure {
				t.Fatalf("journal exists=%t err=%v", exists, loadErr)
			}
			archives, _ := filepath.Glob(journalPath(cfg, "api", "prod") + ".cancelled-*")
			if !wantFailure && len(archives) != 1 {
				t.Fatalf("archives=%v", archives)
			}
		})
	}
}
