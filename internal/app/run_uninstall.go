package app

import (
	"context"
	"errors"

	"github.com/AustinOyugi/no-oops-ops/internal/uninstall"
)

func (a *App) runUninstall(ctx context.Context, args []string) error {
	if len(args) > 1 || (len(args) == 1 && args[0] != "--purge") {
		return errors.New("uninstall supports only the --purge option")
	}

	purge := len(args) == 1
	a.logger.InfoContext(ctx, "starting uninstall", "purge", purge)
	if err := a.uninstaller.Run(ctx, uninstall.Options{Purge: purge}); err != nil {
		return err
	}
	a.logger.InfoContext(ctx, "uninstall completed", "purge", purge)
	return nil
}
