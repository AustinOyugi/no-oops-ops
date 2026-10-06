package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/tui"
)

func TestUIActionsResolveCatalogAlias(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "apps.yml"), []byte("apps:\n  shop:\n    manifest: compose.yml\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "compose.yml"), []byte("services:\n  api:\n    image: nginx:alpine\n  worker:\n    image: busybox:latest\n"), 0600); err != nil {
		t.Fatal(err)
	}
	actions, err := uiActions(config.Config{Workspace: root}, tui.Row{Environment: "prod", App: "api", Service: "prod-api-v42_app"})
	if err != nil {
		t.Fatal(err)
	}
	var deploy, logs, environmentLogs tui.Action
	for _, action := range actions {
		if action.Label == "Environment logs" {
			environmentLogs = action
		}
		if action.Label == "Logs" {
			logs = action
		}
		if action.Label == "Deploy" {
			deploy = action
		}
	}
	if !environmentLogs.HideTasks || strings.Join(environmentLogs.Args, "|") != "--workspace|"+root+"|logs|prod" {
		t.Fatalf("wrong environment logs action: %+v", environmentLogs)
	}
	if !logs.HideTasks || strings.Join(logs.Args, "|") != "--workspace|"+root+"|logs|prod|shop|--service|api" {
		t.Fatalf("wrong logs action: %+v", logs)
	}
	if got := strings.Join(deploy.Args, "|"); got != "--workspace|"+root+"|deploy|prod|shop|--service|api" {
		t.Fatalf("wrong target: %s", got)
	}
	if _, err := uiActions(config.Config{Workspace: root}, tui.Row{Environment: "prod", App: "unknown"}); err == nil {
		t.Fatal("unmapped service accepted")
	}
	if err := os.WriteFile(filepath.Join(root, "apps.yml"), []byte("apps:\n  shop:\n    manifest: compose.yml\n  duplicate:\n    manifest: compose.yml\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := uiActions(config.Config{Workspace: root}, tui.Row{Environment: "prod", App: "api"}); err == nil {
		t.Fatal("ambiguous target accepted")
	}
}
func TestPlatformActionsExcludeLifecycleCommands(t *testing.T) {
	actions, err := uiActions(config.Config{Workspace: t.TempDir()}, tui.Row{Environment: "platform"})
	if err != nil || len(actions) != 2 {
		t.Fatalf("platform actions: %v %v", actions, err)
	}
	for _, action := range actions {
		if action.Label != "Platform status" && action.Label != "Doctor" {
			t.Fatalf("unsafe platform action: %v", action)
		}
	}
}

func TestDashboardChildrenRetainStateAndEnvironment(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "apps.yml"), []byte("apps:\n  shop:\n    manifest: app.yml\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app.yml"), []byte("services:\n  api:\n    image: nginx:alpine\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Workspace: root, RuntimeDir: filepath.Join(root, ".noops-prod"), Environment: "prod"}
	actions, err := uiActions(cfg, tui.Row{Environment: "prod", App: "api"})
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range actions {
		prefix := strings.Join(action.Args[:6], "|")
		want := "--workspace|" + root + "|--state-dir|" + cfg.RuntimeDir + "|--environment|prod"
		if prefix != want {
			t.Fatalf("lost installation selection: %v", action.Args)
		}
		if action.Label == "Release" && !canStreamUIAction(cfg, action) {
			t.Fatal("global selection flags broke release streaming")
		}
	}
	commands, err := uiPalette(cfg, &tui.Row{Environment: "prod", App: "api"})
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range commands {
		if command.Label != "Deploy" {
			continue
		}
		values := map[string]string{}
		for _, field := range command.Fields {
			values[field.Key] = field.Default
		}
		action, err := command.Build(values)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Join(uiCommandArgs(action.Args), "|"); got != "deploy|prod|shop|--service|api" {
			t.Fatalf("incorrect scoped target: %s", got)
		}
		values["environment"] = "dev"
		if _, err := command.Build(values); err == nil {
			t.Fatal("scoped dashboard accepted another environment")
		}
	}
}

func TestUntrackedServiceDoesNotOfferTargetedActions(t *testing.T) {
	actions, err := uiActions(config.Config{Workspace: t.TempDir()}, tui.Row{Untracked: true, Environment: "prod", App: "api"})
	if err != nil || len(actions) != 2 {
		t.Fatalf("untracked actions: %v %v", actions, err)
	}
	for _, action := range actions {
		if action.Label != "Platform status" && action.Label != "Doctor" {
			t.Fatalf("target action for untracked service: %v", action)
		}
	}
}

func TestReleaseOnlyServiceActions(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "apps.yml"), []byte("apps:\n  shop:\n    manifest: compose.yml\n"), 0600)
	os.WriteFile(filepath.Join(root, "compose.yml"), []byte("services:\n  library:\n    image: busybox\n    x-noops: {deploy: false}\n"), 0600)
	actions, err := uiActions(config.Config{Workspace: root}, tui.Row{Environment: "prod", App: "library"})
	if err != nil {
		t.Fatal(err)
	}
	release := false
	for _, action := range actions {
		if action.Label == "Release" {
			release = true
		}
		if action.Label == "Deploy" || action.Label == "Rollback" || action.Label == "Remove" {
			t.Fatalf("deployment action for release-only service: %+v", action)
		}
	}
	if !release {
		t.Fatal("release unavailable")
	}
}
