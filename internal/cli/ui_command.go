package cli

import (
	"context"
	"os"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/tui"
	"github.com/spf13/cobra"
)

func newUICommand(ctx context.Context, rt *runtime) *cobra.Command {
	return &cobra.Command{Use: "ui", Short: "Open the live, read-only service dashboard", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := rt.workspaceRoot()
			if err != nil {
				return err
			}
			cfg, err := config.Load(root)
			if err != nil {
				return err
			}
			return tui.Run(ctx, os.Stdin, os.Stdout, func(ctx context.Context) ([]tui.Row, error) { return tui.Services(ctx, cfg) }, tui.Tasks)
		},
	}
}
