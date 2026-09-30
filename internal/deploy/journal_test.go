package deploy

import (
	"testing"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
)

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
