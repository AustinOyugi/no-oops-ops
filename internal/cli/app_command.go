package cli

import (
	"github.com/AustinOyugi/no-oops-ops/internal/app"
	"github.com/spf13/cobra"
)

func newAppCommand(rt *runtime, use string, args cobra.PositionalArgs, run func(*app.App, []string) error) *cobra.Command {
	return &cobra.Command{Use: use, Args: args, RunE: func(_ *cobra.Command, commandArgs []string) error {
		application, err := rt.application()
		if err != nil {
			return err
		}
		return run(application, commandArgs)
	}}
}
