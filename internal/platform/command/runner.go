package command

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os/exec"
	"time"
)

type Runner struct {
	logger *slog.Logger
}

type RunOptions struct {
	Stdin        io.Reader
	Stdout       io.Writer
	Stderr       io.Writer
	StreamOutput bool
	LogCommand   bool
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
		r.logger.InfoContext(
			ctx,
			"command finished",
			"command", name,
			"args", args,
			"duration", time.Since(start).String(),
		)
	}

	return Result{
		Output: output.Bytes(),
	}, err
}
