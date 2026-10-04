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
