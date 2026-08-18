package app

import (
	"context"
	"errors"
	"github.com/AustinOyugi/no-oops-ops/internal/install"
)

func (a *App) runInstall(ctx context.Context) error {
	a.logger.InfoContext(ctx, "starting noops", "app_name", a.config.AppName)
	result, err := a.installer.Run(ctx)

	if err != nil {
		var prereqErr install.PrerequisiteError
		if errors.As(err, &prereqErr) {
			a.logger.ErrorContext(
				ctx,
				"install prerequisite failed",
				"check", prereqErr.Check,
				"reason", prereqErr.Error(),
			)
		}

		return err
	}

	a.logger.InfoContext(
		ctx,
		"install completed",
		"completed_steps", result.CompletedCount(),
	)

	return nil
}
