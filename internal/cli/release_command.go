package cli

import (
	"context"
	"github.com/AustinOyugi/no-oops-ops/internal/app"
	"github.com/spf13/cobra"
)

func newReleaseCommand(ctx context.Context, rt *runtime) *cobra.Command {
	var releaseFlags targetFlags
	var deployAfterRelease bool
	releaseCmd := newAppCommand(rt, "release <environment> <app>", cobra.ExactArgs(2), func(application *app.App, args []string) error {
		return application.Release(ctx, target(args, releaseFlags), deployAfterRelease)
	})
	addTargetFlags(releaseCmd, &releaseFlags)
	releaseCmd.Flags().BoolVar(&deployAfterRelease, "deploy", false, "Deploy the exact release after it is created")
	var listFlags targetFlags
	listCmd := newAppCommand(rt, "list <environment> <app>", cobra.ExactArgs(2), func(application *app.App, args []string) error {
		return application.ListReleases(ctx, target(args, listFlags))
	})
	addTargetFlags(listCmd, &listFlags)
	releaseCmd.AddCommand(listCmd)
	return releaseCmd
}
