package app

import (
	"context"
	"fmt"
	"io"

	"github.com/AustinOyugi/no-oops-ops/internal/deploy"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

// Logs streams the selected service without running deployment preflight.
func (a *App) Logs(ctx context.Context, target Target, options deploy.LogOptions, stdout, stderr io.Writer) error {
	if target.All {
		return fmt.Errorf("logs requires a single service; use --service <name>")
	}
	environment, path, services, err := a.resolveTarget(target, true, true)
	if err != nil {
		return err
	}
	return deploy.NewService(a.logger, a.config).Logs(ctx, environment, manifest.WithService(path, services[0]), options, stdout, stderr)
}
