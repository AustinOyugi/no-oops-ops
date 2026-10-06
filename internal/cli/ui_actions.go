package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/catalog"
	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"github.com/AustinOyugi/no-oops-ops/internal/tui"
	"golang.org/x/sys/unix"
)

func uiActions(cfg config.Config, row tui.Row) ([]tui.Action, error) {
	base := runtimeCLIArgs(cfg)
	actions := []tui.Action{{Label: "Platform status", Args: append(append([]string{}, base...), "status")}, {Label: "Doctor", Args: append(append([]string{}, base...), "doctor")}}
	if row.Untracked || row.Environment == "platform" {
		return actions, nil
	}
	actions = append(actions, tui.Action{Label: "Environment logs", Args: append(append([]string{}, base...), "logs", row.Environment), HideTasks: true})
	apps, err := catalog.Load(cfg.Workspace)
	if err != nil {
		return nil, err
	}
	type targetMatch struct {
		app, service string
		deployable   bool
	}
	var matches []targetMatch
	names := make([]string, 0, len(apps.Apps))
	for name := range apps.Apps {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		path, err := catalog.Resolve(cfg.Workspace, name)
		if err != nil {
			return nil, err
		}
		services, err := manifest.Services(path)
		if err != nil {
			return nil, err
		}
		for _, service := range services {
			m, err := manifest.LoadService(path, service)
			if err != nil {
				return nil, err
			}
			if m.Name == row.App {
				matches = append(matches, targetMatch{name, service, m.ShouldDeploy()})
			}
		}
	}
	if len(matches) != 1 {
		return nil, fmt.Errorf("cannot uniquely map deployment %q to a catalog app/service (%d matches); use the CLI with an explicit target", row.App, len(matches))
	}
	target := matches[0]
	for _, item := range []struct {
		label   string
		command []string
	}{{"Logs", []string{"logs"}}, {"List releases", []string{"release", "list"}}, {"Release", []string{"release"}}, {"Deploy", []string{"deploy"}}, {"Rollback", []string{"rollback"}}, {"Remove", []string{"remove"}}} {
		if !target.deployable && (item.label == "Deploy" || item.label == "Rollback" || item.label == "Remove" || item.label == "Logs") {
			continue
		}
		args := append([]string{}, base...)
		args = append(args, item.command...)
		args = append(args, row.Environment, target.app, "--service", target.service)
		actions = append(actions, tui.Action{Label: item.label, Args: args, HideTasks: item.label == "Logs"})
	}
	return actions, nil
}

// Child CLI processes must retain the dashboard's selected runtime store.
func runtimeCLIArgs(cfg config.Config) []string {
	args := []string{"--workspace", cfg.Workspace}
	if cfg.RuntimeDir != "" && (cfg.StateDirExplicit || cfg.RuntimeDir != filepath.Join(cfg.Workspace, ".noops")) {
		args = append(args, "--state-dir", cfg.RuntimeDir)
	}
	if cfg.Environment != "" {
		args = append(args, "--environment", cfg.Environment)
	}
	return args
}

func uiCommandArgs(args []string) []string {
	for len(args) >= 2 && (args[0] == "--workspace" || args[0] == "--state-dir" || args[0] == "--environment") {
		args = args[2:]
	}
	return args
}

func runUIAction(ctx context.Context, action tui.Action) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, tui.CommandText(action))
	command := exec.CommandContext(ctx, executable, action.Args...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	err = command.Run()
	if err != nil {
		fmt.Fprintln(os.Stdout, "\nCommand failed:", err)
	} else {
		fmt.Fprintln(os.Stdout, "\nCommand completed successfully.")
	}
	fmt.Fprintln(os.Stdout, "Press Enter to return to the dashboard.")
	for ctx.Err() == nil {
		fds := []unix.PollFd{{Fd: int32(os.Stdin.Fd()), Events: unix.POLLIN}}
		_, pollErr := unix.Poll(fds, 100)
		if pollErr == unix.EINTR {
			continue
		}
		if pollErr != nil {
			break
		}
		if fds[0].Revents&(unix.POLLHUP|unix.POLLERR|unix.POLLNVAL) != 0 {
			break
		}
		if fds[0].Revents&unix.POLLIN != 0 {
			var buf [256]byte
			n, readErr := os.Stdin.Read(buf[:])
			if readErr != nil || n == 0 || strings.Contains(string(buf[:n]), "\n") {
				break
			}
		}
	}
	return err
}
