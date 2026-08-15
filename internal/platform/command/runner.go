package command

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

const maxLoggedOutput = 4096

type Runner struct {
	logger *slog.Logger
}

type RunOptions struct {
	Stdin        io.Reader
	Stdout       io.Writer
	Stderr       io.Writer
	StreamOutput bool
	LogCommand   bool
	Workdir      string
}

type Result struct {
	Output []byte
}

func NewRunner(logger *slog.Logger) *Runner {
	return &Runner{
		logger: logger,
	}
}

func (r *Runner) Run(ctx context.Context, name string, args []string, opts RunOptions) (Result, error) {
	if opts.LogCommand {
		r.logger.InfoContext(ctx, "running command", "command", name, "args", args)
	}

	start := time.Now()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = opts.Stdin

	if opts.Workdir != "" {
		cmd.Dir = opts.Workdir
	}

	var output bytes.Buffer

	if opts.StreamOutput {
		if opts.Stdout != nil {
			cmd.Stdout = io.MultiWriter(opts.Stdout, &output)
		} else {
			cmd.Stdout = &output
		}

		if opts.Stderr != nil {
			cmd.Stderr = io.MultiWriter(opts.Stderr, &output)
		} else {
			cmd.Stderr = &output
		}
	} else {
		cmd.Stdout = &output
		cmd.Stderr = &output
	}

	err := cmd.Run()

	if opts.LogCommand {
		duration := time.Since(start).String()
		if err != nil {
			attrs := []any{
				"command", name,
				"args", args,
				"duration", duration,
				"error", err.Error(),
				"output", logOutput(output.String()),
			}

			var exitError *exec.ExitError
			if errors.As(err, &exitError) {
				attrs = append(attrs, "exit_code", exitError.ExitCode())
			}

			r.logger.ErrorContext(ctx, "command failed", attrs...)
		} else {
			r.logger.InfoContext(
				ctx,
				"command completed",
				"command", name,
				"args", args,
				"duration", duration,
			)
		}
	}

	return Result{
		Output: output.Bytes(),
	}, err
}

func logOutput(output string) string {
	output = strings.TrimSpace(output)
	if len(output) <= maxLoggedOutput {
		return output
	}

	return output[:maxLoggedOutput] + "… (truncated)"
}
