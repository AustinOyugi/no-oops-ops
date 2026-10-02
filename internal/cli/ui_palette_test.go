package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/tui"
)

func paletteFixture(t *testing.T) []tui.Command {
	t.Helper()
	root := t.TempDir()
	for name, contents := range map[string]string{"apps.yml": "apps:\n  shop:\n    manifest: compose.yml\n", "compose.yml": "services:\n  api:\n    image: nginx:alpine\n  worker:\n    image: busybox:latest\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	commands, err := uiPalette(config.Config{Workspace: root}, &tui.Row{Environment: "prod", App: "api"})
	if err != nil {
		t.Fatal(err)
	}
	return commands
}
func TestPaletteCommandsMatchCLI(t *testing.T) {
	commands := paletteFixture(t)
	if len(commands) != 18 {
		t.Fatalf("command count: %d", len(commands))
	}
	for _, command := range commands {
		t.Run(command.Label, func(t *testing.T) {
			values := map[string]string{}
			for _, field := range command.Fields {
				values[field.Key] = field.Default
				if !field.Boolean && values[field.Key] == "" {
					values[field.Key] = "example"
				}
			}
			if command.Label == "Upgrade" {
				values["to"] = "v0.42.0"
			}
			action, err := command.Build(values)
			if err != nil {
				t.Fatal(err)
			}
			root := NewRootCommand(context.Background())
			cmd, args, err := root.Find(action.Args)
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.ParseFlags(args); err != nil {
				t.Fatal(err)
			}
			if err := cmd.ValidateArgs(cmd.Flags().Args()); err != nil {
				t.Fatalf("%v: %v", action.Args, err)
			}
			if err := cmd.ValidateFlagGroups(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestPaletteTargetsFlagsAndSecrets(t *testing.T) {
	for _, command := range paletteFixture(t) {
		values := map[string]string{}
		for _, field := range command.Fields {
			values[field.Key] = field.Default
		}
		switch command.Label {
		case "Deploy":
			values["all"] = "true"
			values["quick"] = "true"
			action, err := command.Build(values)
			if err != nil {
				t.Fatal(err)
			}
			args := strings.Join(action.Args, " ")
			if !strings.Contains(args, "--all") || !strings.Contains(args, "--quick") || strings.Contains(args, "--service") {
				t.Fatalf("args %s", args)
			}
			values["all"] = "false"
			values["service"] = "unknown"
			if _, err := command.Build(values); err == nil {
				t.Fatal("unknown service accepted")
			}
		case "Cleanup":
			action, err := command.Build(values)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(strings.Join(action.Args, " "), "--apply") {
				t.Fatal("cleanup default deletes")
			}
			values["keep"] = "bad"
			if _, err := command.Build(values); err == nil {
				t.Fatal("bad retain count accepted")
			}
		case "Upgrade":
			values["yes"] = "true"
			if _, err := command.Build(values); err == nil {
				t.Fatal("latest with yes accepted")
			}
		case "Secret set":
			for _, field := range command.Fields {
				if field.Key == "value" {
					t.Fatal("secret value exposed in form")
				}
			}
			values["key"] = "TOKEN"
			action, err := command.Build(values)
			if err != nil {
				t.Fatal(err)
			}
			if len(action.Args) != 6 {
				t.Fatalf("secret argv: %v", action.Args)
			}
		}
	}
}

func TestReleaseCanTargetUndeployedCatalogService(t *testing.T) {
	for _, command := range paletteFixture(t) {
		if command.Label == "Release" {
			action, err := command.Build(map[string]string{"environment": "prod", "app": "shop", "service": "worker", "deploy": "true"})
			if err != nil {
				t.Fatal(err)
			}
			args := strings.Join(action.Args, " ")
			if !strings.Contains(args, "--service worker") || !strings.Contains(args, "--deploy") {
				t.Fatalf("new service release: %s", args)
			}
			return
		}
	}
	t.Fatal("missing release form")
}
