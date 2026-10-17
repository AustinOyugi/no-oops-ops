package cli

import (
	"context"
	"github.com/AustinOyugi/no-oops-ops/internal/app"
	"github.com/spf13/cobra"
)

func newDeployCommand(ctx context.Context, rt *runtime) *cobra.Command {
	var flags targetFlags
	var quick bool
	cmd := newAppCommand(rt, "deploy <app>", cobra.RangeArgs(1, 2), func(application *app.App, args []string) error {
		return application.Deploy(ctx, target(rt.selectedEnvironment, args, flags), quick)
	})
	addTargetFlags(cmd, &flags)
	cmd.Flags().BoolVar(&quick, "quick", false, "Use the health-check start period as the rollout monitor")
	return cmd
}

func newRollbackCommand(ctx context.Context, rt *runtime) *cobra.Command {
	var flags targetFlags
	cmd := newAppCommand(rt, "rollback <app>", cobra.RangeArgs(1, 2), func(application *app.App, args []string) error {
		return application.Rollback(ctx, target(rt.selectedEnvironment, args, flags))
	})
	addTargetFlags(cmd, &flags)
	return cmd
}

func newRemoveCommand(ctx context.Context, rt *runtime) *cobra.Command {
	var flags targetFlags
	cmd := newAppCommand(rt, "remove <app>", cobra.RangeArgs(1, 2), func(application *app.App, args []string) error {
		return application.Remove(ctx, target(rt.selectedEnvironment, args, flags))
	})
	addTargetFlags(cmd, &flags)
	return cmd
}
