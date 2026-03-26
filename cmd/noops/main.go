package main

import (
	"context"
	"fmt"
	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"os"
	"os/signal"
	"syscall"

	"github.com/AustinOyugi/no-oops-ops/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg := config.Load()
	application := app.New(cfg)
	if err := application.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
