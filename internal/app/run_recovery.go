package app

import (
	"context"
	"fmt"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

// Recover starts platform infrastructure first and replays saved deployments
// serially in dependency order. It never builds or selects a new release.
func (a *App) Recover(ctx context.Context, target Target) error {
	environment, path, services, err := a.resolveTarget(target, true, true)
	if err != nil {
		return err
	}
	recovery, ok := a.deployer.(interface {
		Recover(context.Context, string, string) error
	})
	if !ok {
		return fmt.Errorf("deployer does not support recovery")
	}
	// Validate the complete graph before starting any infrastructure.
	if _, err := manifest.ReleaseOrder(path); err != nil {
		return err
	}
	if err := a.Install(ctx); err != nil {
		return fmt.Errorf("recover platform infrastructure: %w", err)
	}
	for i, service := range services {
		a.logger.InfoContext(ctx, "recovering service", "service", service, "position", i+1, "total", len(services), "concurrent_startups", 1)
		if err := recovery.Recover(ctx, environment, manifest.WithService(path, service)); err != nil {
			return fmt.Errorf("recovery stopped at %s; remaining services blocked: %w", service, err)
		}
	}
	return nil
}
