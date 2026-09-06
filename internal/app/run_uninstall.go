package app

import (
	"context"

	"github.com/AustinOyugi/no-oops-ops/internal/uninstall"
)

// Uninstall removes the local deployment platform.
func (a *App) Uninstall(ctx context.Context, purge bool) error {
	a.logger.InfoContext(ctx, "starting uninstall", "purge", purge)
	if err := a.uninstaller.Run(ctx, uninstall.Options{Purge: purge}); err != nil {
		return err
	}
	a.logger.InfoContext(ctx, "uninstall completed", "purge", purge)
	return nil
}
