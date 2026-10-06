package cli

import (
	"context"
	"github.com/AustinOyugi/no-oops-ops/internal/deploy"
	"os"

	"github.com/AustinOyugi/no-oops-ops/internal/tui"
	"github.com/spf13/cobra"
)

func newUICommand(ctx context.Context, rt *runtime) *cobra.Command {
	return &cobra.Command{Use: "ui", Short: "Open the live service dashboard", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := rt.configuration()
			if err != nil {
				return err
			}
			return tui.Run(ctx, os.Stdin, os.Stdout, func(ctx context.Context) ([]tui.Row, error) { return tui.Services(ctx, cfg) }, tui.Tasks, tui.Execution{Rollouts: func(ctx context.Context) ([]deploy.RolloutProgress, error) { return tui.Rollouts(ctx, cfg) }, CanStream: func(action tui.Action) bool { return canStreamUIAction(cfg, action) }, Stream: streamUIAction, Palette: func(row *tui.Row) ([]tui.Command, error) { return uiPalette(cfg, row) }, Resolve: func(row tui.Row) ([]tui.Action, error) { return uiActions(cfg, row) }, Run: runUIAction})
		},
	}
}
