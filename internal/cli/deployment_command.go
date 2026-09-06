package cli

import (
	"context"
	"github.com/AustinOyugi/no-oops-ops/internal/app"
	"github.com/spf13/cobra"
)

func newDeployCommand(ctx context.Context, rt *runtime) *cobra.Command {
	var flags targetFlags
	var quick bool
	cmd := newAppCommand(rt, "deploy <environment> <app>", cobra.ExactArgs(2), func(application *app.App, args []string) error {
		return application.Deploy(ctx, target(args, flags), quick)
	})
	addTargetFlags(cmd, &flags)
	cmd.Flags().BoolVar(&quick, "quick", false, "Use the health-check start period as the rollout monitor")
	return cmd
}

func newRollbackCommand(ctx context.Context, rt *runtime) *cobra.Command {
	var flags targetFlags
	cmd := newAppCommand(rt, "rollback <environment> <app>", cobra.ExactArgs(2), func(application *app.App, args []string) error { return application.Rollback(ctx, target(args, flags)) })
	addTargetFlags(cmd, &flags)
	return cmd
}

func newRemoveCommand(ctx context.Context, rt *runtime) *cobra.Command {
	var flags targetFlags
	cmd := newAppCommand(rt, "remove <environment> <app>", cobra.ExactArgs(2), func(application *app.App, args []string) error { return application.Remove(ctx, target(args, flags)) })
	addTargetFlags(cmd, &flags)
	return cmd
}
