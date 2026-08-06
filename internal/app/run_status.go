package app

import (
	"context"
	"github.com/AustinOyugi/no-oops-ops/internal/status"
)

func (a *App) runStatus(ctx context.Context) error {
	a.logger.InfoContext(ctx, "starting noops status", "app_name", a.config.AppName)

	result, err := a.status.Run(ctx)
	if err != nil {
		return err
	}

	a.logger.InfoContext(
		ctx,
		"status metadata",
		"version", result.Metadata.Version,
		"installed_at", result.Metadata.InstalledAt,
		"swarm_state", result.Metadata.Swarm.LocalNodeState,
		"network", result.Metadata.Network.Name,
		"registry", result.Metadata.Registry.Name,
		"registry_port", result.Metadata.Registry.Port,
	)

	for _, component := range result.Components {
		if component.Status == status.ComponentStatusMissing {
			a.logger.ErrorContext(
				ctx,
				"status component",
				"name", component.Name,
				"status", component.Status,
				"message", component.Message,
			)
			continue
		}

		a.logger.InfoContext(
			ctx,
			"status component",
			"name", component.Name,
			"status", component.Status,
			"message", component.Message,
		)
	}

	a.logger.InfoContext(ctx, "status completed", "components", len(result.Components))
	return nil
}
