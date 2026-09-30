package release

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

func TestPostReleaseCleanupRunsOnlyForPublishedReleases(t *testing.T) {
	s := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), config.Config{})
	calls := 0
	s.SetAfterRelease(func(_ context.Context, result Result) error {
		calls++
		if result.Manifest.Name != "api" || result.Environment != "prod" {
			t.Fatalf("wrong release scope: %#v", result)
		}
		return errors.New("cleanup unavailable")
	})
	s.completeRelease(context.Background(), Result{Manifest: manifest.Manifest{Name: "api"}, Environment: "prod"})
	if calls != 0 {
		t.Fatal("cleanup ran for an unpublished release")
	}
	s.completeRelease(context.Background(), Result{Manifest: manifest.Manifest{Name: "api"}, Environment: "prod", Pushed: true})
	if calls != 1 {
		t.Fatalf("cleanup calls=%d", calls)
	}
	if _, err := s.Run(context.Background(), "prod", "missing-manifest.yml"); err == nil {
		t.Fatal("expected release failure")
	}
	if calls != 1 {
		t.Fatal("cleanup ran after a release failure")
	}
}
