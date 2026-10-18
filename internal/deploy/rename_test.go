package deploy

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"github.com/AustinOyugi/no-oops-ops/internal/release"
)

func TestTraceRenamedStacks(t *testing.T) {
	for _, tc := range []struct {
		name, otherService, source, image, environment, liveStack, manifestPath string
		want                                                                    bool
	}{
		{name: "legacy stable stack", want: true},
		{name: "new name already deployed", want: true},
		{name: "ambiguous old identities"},
		{name: "legacy blue green stack", liveStack: "dev-rolengi-dhibitisha-service-r20261018", want: true},
		{name: "new history same manifest", manifestPath: "same", want: true},
		{name: "other manifest", manifestPath: "other"},
		{name: "other environment", environment: "prod"},
		{name: "stack already absent", liveStack: "unrelated"},
		{name: "other repository", source: "https://github.com/Rolengi/other.git"},
		{name: "other image", image: "worker:old"},
		{name: "old service still declared", otherService: "rolengi-dhibitisha-service"},
		{name: "repository shared with worker", otherService: "worker"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			cfg := config.Config{StateDir: filepath.Join(root, "state"), Workspace: root}
			path := filepath.Join(root, "app.yml")
			service := "    image: dhibitisha-service\n    build: {context: ., dockerfile: Dockerfile}\n    x-noops:\n      build:\n        source:\n          git:\n            url: https://github.com/Rolengi/dhibitisha-service.git\n            environments:\n              dev: {ref: develop}\n"
			text := "services:\n  dhibitisha-service:\n" + service
			if tc.otherService != "" {
				text += "  " + tc.otherService + ":\n" + service
			}
			if err := os.WriteFile(path, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			m, err := manifest.LoadService(path, "dhibitisha-service")
			if err != nil {
				t.Fatal(err)
			}
			previous := Deployment{App: "rolengi-dhibitisha-service", Environment: "dev", StackName: "dev-rolengi-dhibitisha-service", ReleaseTag: "old", CreatedAt: time.Now()}
			if tc.environment != "" {
				previous.Environment = tc.environment
			}
			if strings.Contains(tc.liveStack, "-r2026") {
				previous.StackName = tc.liveStack
			}
			if tc.manifestPath == "same" {
				previous.ManifestPath = path
			}
			if tc.manifestPath == "other" {
				previous.ManifestPath = filepath.Join(root, "other.yml")
			}
			s := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)
			if _, err := s.deployments.Save(cfg, previous); err != nil {
				t.Fatal(err)
			}
			if tc.name == "new name already deployed" {
				if _, err := s.deployments.Save(cfg, Deployment{App: m.Name, Environment: "dev", StackName: "dev-dhibitisha-service", CreatedAt: time.Now()}); err != nil {
					t.Fatal(err)
				}
			}
			record := release.Metadata{App: previous.App, Environment: previous.Environment, Tag: "old", Image: "dhibitisha-service:old", Git: &release.GitMetadata{URL: "https://github.com/Rolengi/dhibitisha-service.git"}}
			if tc.source != "" {
				record.Git.URL = tc.source
			}
			if tc.image != "" {
				record.Image = tc.image
			}
			// Use the real store format to exercise compatibility with existing history.
			dir := filepath.Join(cfg.StateDir, "apps", previous.App, previous.Environment, "releases")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "old.json"), data, 0600); err != nil {
				t.Fatal(err)
			}
			extra := "dev-unrelated"
			if tc.name == "ambiguous old identities" {
				second := previous
				second.App = "another-old-api"
				second.StackName = "dev-another-old-api"
				if _, err := s.deployments.Save(cfg, second); err != nil {
					t.Fatal(err)
				}
				record.App = second.App
				secondDir := filepath.Join(cfg.StateDir, "apps", second.App, "dev", "releases")
				if err := os.MkdirAll(secondDir, 0700); err != nil {
					t.Fatal(err)
				}
				secondData, err := json.Marshal(record)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(secondDir, "old.json"), secondData, 0600); err != nil {
					t.Fatal(err)
				}
				extra = second.StackName
			}
			live := tc.liveStack
			if live == "" {
				live = previous.StackName
			}
			bin := filepath.Join(root, "bin")
			if err := os.Mkdir(bin, 0700); err != nil {
				t.Fatal(err)
			}
			script := `#!/bin/sh
case "$1 $2" in
 "service ls") printf '%s_app\n%s_app\n' "$RENAME_TEST_STACK" "$RENAME_TEST_EXTRA_STACK";;
 "stack rm") printf '%s\n' "$3" >> "$RENAME_TEST_REMOVED";;
 *) exit 2;;
esac
`
			if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("RENAME_TEST_STACK", live)
			t.Setenv("RENAME_TEST_EXTRA_STACK", extra)
			removed := filepath.Join(root, "removed")
			t.Setenv("RENAME_TEST_REMOVED", removed)
			got, err := s.retiredDeployments(context.Background(), m, "dev")
			if tc.name == "ambiguous old identities" {
				if err == nil || !strings.Contains(err.Error(), "multiple previous stacks") {
					t.Fatalf("expected ambiguous ownership error, got %v", err)
				}
				if _, statErr := os.Stat(removed); !os.IsNotExist(statErr) {
					t.Fatal("ambiguous trace removed a stack")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if (len(got) == 1) != tc.want {
				t.Fatalf("retired deployments = %+v, want match %v", got, tc.want)
			}
			if _, err := os.Stat(removed); !os.IsNotExist(err) {
				t.Fatal("tracing removed a stack before promotion")
			}
			if tc.want {
				s.removeStaleAppStacks(context.Background(), "dev", got[0].App, "dev-dhibitisha-service")
				data, err := os.ReadFile(removed)
				if err != nil || strings.TrimSpace(string(data)) != previous.StackName {
					t.Fatalf("removed=%q err=%v", data, err)
				}
			}
			if tc.want && got[0].StackName != previous.StackName {
				t.Fatalf("wrong previous stack: %+v", got)
			}
		})
	}
}
