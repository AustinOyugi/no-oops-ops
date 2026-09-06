package cli

import (
	"context"

	"github.com/AustinOyugi/no-oops-ops/internal/app"
	"github.com/AustinOyugi/no-oops-ops/internal/cleanup"
	"github.com/spf13/cobra"
)

func newInstallCommand(ctx context.Context, rt *runtime) *cobra.Command {
	return newAppCommand(rt, "install", cobra.NoArgs, func(application *app.App, _ []string) error {
		return application.Install(ctx)
	})
}

func newUninstallCommand(ctx context.Context, rt *runtime) *cobra.Command {
	var purge bool
	cmd := newAppCommand(rt, "uninstall", cobra.NoArgs, func(application *app.App, _ []string) error {
		return application.Uninstall(ctx, purge)
	})
	cmd.Flags().BoolVar(&purge, "purge", false, "Remove persistent registry data")
	return cmd
}

func newDoctorCommand(ctx context.Context, rt *runtime) *cobra.Command {
	var deployReady bool
	cmd := newAppCommand(rt, "doctor", cobra.NoArgs, func(application *app.App, _ []string) error {
		return application.Doctor(ctx, deployReady)
	})
	cmd.Flags().BoolVar(&deployReady, "deploy-ready", false, "Check deployment prerequisites only")
	return cmd
}

func newStatusCommand(ctx context.Context, rt *runtime) *cobra.Command {
	return newAppCommand(rt, "status", cobra.NoArgs, func(application *app.App, _ []string) error {
		return application.Status(ctx)
	})
}

func newCleanupCommand(ctx context.Context, rt *runtime) *cobra.Command {
	var apply, orphaned bool
	var keep int
	cmd := newAppCommand(rt, "cleanup", cobra.NoArgs, func(application *app.App, _ []string) error {
		return application.Cleanup(ctx, cleanup.Options{Apply: apply, Orphaned: orphaned, Keep: keep})
	})
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply cleanup")
	cmd.Flags().BoolVar(&orphaned, "orphaned", false, "Include orphaned app environments")
	cmd.Flags().IntVar(&keep, "keep", 2, "Number of records to retain")
	return cmd
}
