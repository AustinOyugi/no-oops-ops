package cli

import (
	"context"
	"fmt"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/workspace"
	"github.com/spf13/cobra"
)

func NewRootCommand(ctx context.Context) *cobra.Command {
	rt := runtime{}
	root := &cobra.Command{
		Use:          "noops",
		Short:        "No Oops Ops deployment CLI",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return printVersion(cmd)
		},
	}
	root.Flags().BoolP("version", "v", false, "Print version information")
	root.PersistentFlags().StringVar(&rt.workspace, "workspace", "", "Workspace directory")
	root.AddCommand(
		newVersionCommand(), newInitCommand(), newInstallCommand(ctx, &rt),
		newUninstallCommand(ctx, &rt), newDoctorCommand(ctx, &rt), newStatusCommand(ctx, &rt),
		newReleaseCommand(ctx, &rt), newDeployCommand(ctx, &rt), newRollbackCommand(ctx, &rt),
		newRemoveCommand(ctx, &rt), newSecretCommand(ctx, &rt), newCertificateCommand(ctx, &rt),
		newCleanupCommand(ctx, &rt),
	)
	return root
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{Use: "version", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		return printVersion(cmd)
	}}
}

func printVersion(cmd *cobra.Command) error {
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "noops %s\n", config.Version)
	return err
}

func newInitCommand() *cobra.Command {
	return &cobra.Command{Use: "init <workspace>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := workspace.Initialize(args[0], config.Version)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "initialized No Oops workspace at %s\n", paths.Root)
		return err
	}}
}
