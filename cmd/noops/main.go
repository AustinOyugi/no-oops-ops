package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/AustinOyugi/no-oops-ops/internal/app"
	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/workspace"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	rawArgs := os.Args[1:]
	if isVersionCommand(rawArgs) {
		_, _ = fmt.Fprintf(os.Stdout, "noops %s\n", config.Version)
		return
	}
	workspaceRoot, args, err := resolveWorkspaceArgs(rawArgs)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(args) > 0 && args[0] == "init" {
		if len(args) != 2 {
			_, _ = fmt.Fprintln(os.Stderr, "init requires exactly one workspace directory")
			os.Exit(1)
		}
		paths, initErr := workspace.Initialize(args[1], config.Version)
		if initErr != nil {
			_, _ = fmt.Fprintln(os.Stderr, initErr)
			os.Exit(1)
		}
		_, _ = fmt.Fprintf(os.Stdout, "initialized No Oops workspace at %s\n", paths.Root)
		return
	}
	cfg, err := config.Load(workspaceRoot)

	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, err)
		if err != nil {
			return
		}
		os.Exit(1)
	}

	application, err := app.New(cfg)

	if err != nil {
		_, err := fmt.Fprintln(os.Stderr, err)
		if err != nil {
			return
		}
		os.Exit(1)
	}

	if err := application.Run(ctx, args); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func isVersionCommand(args []string) bool {
	return len(args) == 0 || (len(args) == 1 && (args[0] == "version" || args[0] == "--version" || args[0] == "-v"))
}

func resolveWorkspaceArgs(args []string) (string, []string, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", nil, fmt.Errorf("get working directory: %w", err)
	}
	if len(args) == 0 || args[0] != "--workspace" {
		return root, args, nil
	}
	if len(args) < 3 || args[1] == "" {
		return "", nil, fmt.Errorf("--workspace requires a directory and command")
	}
	return args[1], args[2:], nil
}
