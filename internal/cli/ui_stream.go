package cli

import (
	"context"
	"io"
	"os"
	"os/exec"
	"slices"
	"syscall"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/catalog"
	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"github.com/AustinOyugi/no-oops-ops/internal/tui"
)

func canStreamUIAction(cfg config.Config, action tui.Action) bool {
	if len(action.Args) < 3 {
		return false
	}
	args := action.Args[2:]
	verb := args[0]
	switch verb {
	case "secret":
		return len(args) > 1 && args[1] != "set"
	case "upgrade":
		return slices.Contains(args, "--check") || slices.Contains(args, "--dry-run")
	case "deploy", "release":
		if verb == "release" && !slices.Contains(args, "--deploy") {
			return true
		}
		current, err := config.Load(cfg.Workspace)
		if err != nil {
			return false
		}
		if current.ACMEEmail != "" || current.NginxCloudflare {
			return true
		}
		if len(args) < 3 {
			return false
		}
		environment, app := args[1], args[2]
		path, err := catalog.Resolve(cfg.Workspace, app)
		if err != nil {
			return false
		}
		services, err := manifest.Services(path)
		if err != nil {
			return false
		}
		if index := slices.Index(args, "--service"); index >= 0 && index+1 < len(args) {
			services = []string{args[index+1]}
		}
		for _, service := range services {
			m, err := manifest.LoadService(path, service)
			if err != nil {
				return false
			}
			m = m.ForEnvironment(environment)
			if m.Expose.TLS && m.Expose.TLSCertificate == "" {
				return false
			}
		}
		return true
	case "install":
		return false
	default:
		return true
	}
}

func streamUIAction(ctx context.Context, action tui.Action, output io.Writer) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	return streamCommand(ctx, executable, action.Args, output)
}
func streamCommand(ctx context.Context, executable string, args []string, output io.Writer) error {
	command := exec.CommandContext(ctx, executable, args...)
	command.Stdout = output
	command.Stderr = output // stdin remains closed: no terminal input can be consumed.
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
	command.WaitDelay = 2 * time.Second
	return command.Run()
}
