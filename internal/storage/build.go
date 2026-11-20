package storage

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Build guards both isolated release builds and small deployment wrapper builds.
// Cancellation requests build termination; it is not a hard filesystem quota.
func Build(ctx context.Context, r Runner, p Policy, logger *slog.Logger, run func(context.Context) error) error {
	if p.MinFreeBytes == 0 {
		return run(ctx)
	}
	if err := PruneCache(ctx, r, p); err != nil {
		return err
	}
	usage, err := Inspect(ctx, r)
	if err != nil {
		return err
	}
	if err := Check(p, usage, logger); err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
		defer cancel()
		if err := PruneCache(cleanup, r, p); err != nil {
			logger.Warn("post-build cache cleanup failed", "error", err)
		}
	}()
	buildCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				done <- nil
				return
			case <-buildCtx.Done():
				done <- nil
				return
			case <-ticker.C:
				usage, err := Inspect(buildCtx, r)
				if err == nil {
					err = Check(p, usage, logger)
				}
				if err != nil {
					done <- fmt.Errorf("stop build: storage safeguard: %w", err)
					cancel()
					return
				}
			}
		}
	}()
	buildErr := run(buildCtx)
	close(stop)
	if err := <-done; err != nil {
		return err
	}
	return buildErr
}
