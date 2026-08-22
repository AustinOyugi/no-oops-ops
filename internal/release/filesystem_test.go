package release

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

func TestSaveMetadataHistoryRecordsExternalImageDetails(t *testing.T) {
	cfg := config.Config{StateDir: t.TempDir()}
	metadata := Metadata{
		App:           "keycloak",
		Build:         false,
		CreateAt:      time.Date(2026, time.August, 22, 1, 2, 3, 0, time.UTC),
		Environment:   "production",
		Image:         "quay.io/keycloak/keycloak:20260822-010203",
		RegistryImage: "127.0.0.1:5000/quay.io/keycloak/keycloak:20260822-010203",
		SourceTag:     "21.0.1",
		Tag:           "20260822-010203",
	}

	path, err := saveMetadataHistory(cfg, metadata.App, metadata)
	if err != nil {
		t.Fatalf("saveMetadataHistory() error = %v", err)
	}
	if want := filepath.Join(cfg.StateDir, "apps", "keycloak", "production", "releases", "20260822-010203.json"); path != want {
		t.Fatalf("saveMetadataHistory() path = %q, want %q", path, want)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read metadata = %v", err)
	}
	var saved map[string]any
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("decode metadata = %v", err)
	}

	if got := saved["build"]; got != false {
		t.Errorf("build = %#v, want false", got)
	}
	if got := saved["source_tag"]; got != "21.0.1" {
		t.Errorf("source_tag = %#v, want %q", got, "21.0.1")
	}
	if got := saved["tag"]; got != "20260822-010203" {
		t.Errorf("tag = %#v, want %q", got, "20260822-010203")
	}
}

func TestSourceTag(t *testing.T) {
	tests := []struct {
		name  string
		image manifest.Image
		want  string
	}{
		{
			name:  "external image",
			image: manifest.Image{Build: boolPtr(false), Tag: "21.0.1"},
			want:  "21.0.1",
		},
		{
			name:  "source build",
			image: manifest.Image{Build: boolPtr(true), Tag: "ignored"},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sourceTag(manifest.Manifest{Image: tt.image}); got != tt.want {
				t.Errorf("sourceTag() = %q, want %q", got, tt.want)
			}
		})
	}
}

func boolPtr(value bool) *bool {
	return &value
}
