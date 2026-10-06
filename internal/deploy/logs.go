package deploy

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

type LogOptions struct {
	Follow     bool
	Tail       string
	Since      string
	Timestamps bool
}

func (o LogOptions) Validate() error {
	if o.Tail != "all" {
		n, err := strconv.Atoi(o.Tail)
		if err != nil || n < 0 {
			return fmt.Errorf("--tail must be a non-negative integer or all")
		}
	}
	return nil
}

// Logs writes directly to the caller's writers so a long-running stream does
// not accumulate captured output in memory.
func (s *Service) Logs(ctx context.Context, environment, path string, options LogOptions, stdout, stderr io.Writer) error {
	if err := options.Validate(); err != nil {
		return err
	}
	m, err := manifest.Load(path)
	if err != nil {
		return err
	}
	active, err := s.deployments.Latest(s.config, m.Name, environment)
	if err != nil {
		return err
	}
	name := logServiceName(environment, m.Name, active)
	args := []string{"service", "logs", "--tail", options.Tail}
	if options.Follow {
		args = append(args, "--follow")
	}
	if options.Since != "" {
		args = append(args, "--since", options.Since)
	}
	if options.Timestamps {
		args = append(args, "--timestamps")
	}
	args = append(args, "--", name)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("read logs for service %q: %w", name, err)
	}
	return nil
}

func logServiceName(environment, name string, active Deployment) string {
	if active.ServiceName != "" {
		return active.ServiceName
	}
	if active.StackName != "" {
		if active.StackName != stackName(environment, name) {
			return active.StackName + "_app"
		}
		return active.StackName + "_" + serviceName(environment, name)
	}
	return swarmServiceName(environment, name)
}
