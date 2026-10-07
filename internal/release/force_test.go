package release

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
)

func TestForceReleaseTagOverride(t *testing.T) {
	cfg := config.Config{StateDir: t.TempDir()}
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)
	ctx := context.Background()
	for _, force := range []bool{false, true} {
		if err := service.ensureTagUnused(ctx, "api", "prod", "sha-test", Options{Force: force}); err != nil {
			t.Fatal(err)
		}
	}
	path, err := saveMetadataHistory(cfg, "api", Metadata{App: "api", Environment: "prod", Tag: "sha-test", Digest: "old"})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ensureTagUnused(ctx, "api", "prod", "sha-test", Options{}); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected duplicate rejection with force hint, got %v", err)
	}
	if err := service.ensureTagUnused(ctx, "api", "prod", "sha-test", Options{Force: true}); err != nil {
		t.Fatal(err)
	}
	existing, err := NewFilesystemStore().Find(cfg, "api", "prod", "sha-test")
	if err != nil || existing.Digest != "old" {
		t.Fatalf("force check changed metadata: %+v, %v", existing, err)
	}
	if _, err := saveMetadataHistory(cfg, "api", Metadata{App: "api", Environment: "prod", Tag: "sha-test", Digest: "new"}); err != nil {
		t.Fatal(err)
	}
	existing, err = NewFilesystemStore().Find(cfg, "api", "prod", "sha-test")
	if err != nil || existing.Digest != "new" {
		t.Fatalf("metadata not replaced: %+v, %v", existing, err)
	}
	if err := os.WriteFile(path, []byte("invalid JSON"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := service.ensureTagUnused(ctx, "api", "prod", "sha-test", Options{Force: true}); err == nil {
		t.Fatal("force bypassed corrupt metadata")
	}
}
