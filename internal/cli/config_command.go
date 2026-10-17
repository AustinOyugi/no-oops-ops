package cli

import (
	"fmt"

	"github.com/AustinOyugi/no-oops-ops/internal/app"
	"github.com/spf13/cobra"
)

func newConfigCommand(rt *runtime) *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Manage reusable environment config files"}
	for _, verb := range []string{"install", "upgrade"} {
		cmd.AddCommand(&cobra.Command{Use: verb + " <name> <file>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
			root, err := rt.workspaceRoot()
			if err != nil {
				return err
			}
			var target string
			if verb == "install" {
				target, err = app.InstallConfig(cmd.Context(), root, args[0], args[1])
			} else {
				target, err = app.UpgradeConfig(cmd.Context(), root, args[0], args[1])
			}
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "config %s: %s\n", cmd.Name(), target)
			return err
		}})
	}
	return cmd
}
