package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/workspace"
)

func TestVersionForms(t *testing.T) {
	for _, args := range [][]string{{}, {"version"}, {"--version"}, {"-v"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			root := NewRootCommand(context.Background())
			var output strings.Builder
			root.SetOut(&output)
			root.SetErr(&output)
			root.SetArgs(args)
			if err := root.Execute(); err != nil {
				t.Fatalf("noops %v returned error: %v", args, err)
			}
			if !strings.HasPrefix(output.String(), "noops ") {
				t.Errorf("output = %q, want version", output.String())
			}
		})
	}
}

func TestUpgradeRequiresExplicitAction(t *testing.T) {
	root := NewRootCommand(context.Background())
	root.SetArgs([]string{"upgrade"})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "requires --to") {
		t.Fatalf("upgrade error = %v, want explicit target requirement", err)
	}
}

func TestUpgradeRejectsUnpinnedNonInteractiveSwitch(t *testing.T) {
	root := NewRootCommand(context.Background())
	root.SetArgs([]string{"upgrade", "--to", "latest", "--yes"})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "pin an exact release tag") {
		t.Fatalf("upgrade error = %v, want pinned-version requirement", err)
	}
}

func TestUpgradeDryRunShowsSwitchWithoutChangingCatalog(t *testing.T) {
	workspaceRoot := t.TempDir()
	if _, err := workspace.Initialize(workspaceRoot, config.Version); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(workspaceRoot, "apps.yml")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	root := NewRootCommand(context.Background())
	var output strings.Builder
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"--workspace", workspaceRoot, "upgrade", "--to", "v9.8.7", "--dry-run"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Current version: dev", "Target version:  v9.8.7", "Repository:      AustinOyugi/no-oops-ops", "Dry run; no changes made."} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("upgrade output = %q, want %q", output.String(), want)
		}
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Errorf("dry run changed apps.yml:\nbefore: %s\nafter: %s", before, after)
	}
}

func TestConfirmUpgradeDefaultsToNo(t *testing.T) {
	var output strings.Builder
	confirmed, err := confirmUpgrade(strings.NewReader("\n"), &output)
	if err != nil {
		t.Fatal(err)
	}
	if confirmed {
		t.Fatal("empty confirmation unexpectedly approved upgrade")
	}
	if !strings.Contains(output.String(), "[y/N]") {
		t.Errorf("prompt = %q", output.String())
	}
}

func TestRootCommandRendersReleaseHelp(t *testing.T) {
	root := NewRootCommand(context.Background())
	var output strings.Builder
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"release", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("release help returned error: %v", err)
	}
	for _, want := range []string{"--all", "--service", "--deploy", "<environment> <app>"} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("release help = %q, want %q", output.String(), want)
		}
	}
}

func TestTargetFlagsAreMutuallyExclusive(t *testing.T) {
	root := NewRootCommand(context.Background())
	root.SetArgs([]string{"release", "prod", "app", "--all", "--service", "api"})
	if err := root.Execute(); err == nil {
		t.Fatal("release with both selectors returned nil error")
	}
}
