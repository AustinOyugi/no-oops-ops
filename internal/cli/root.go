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
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			rt.selectedEnvironment = rt.environment
			if len(args) == 0 {
				return nil
			}
			positional := false
			switch cmd.Name() {
			case "release", "deploy", "rollback", "remove", "logs":
				positional = true
			}
			if parent := cmd.Parent(); parent != nil && (parent.Name() == "secret" || parent.Name() == "release") {
				positional = true
			}
			if positional {
				if rt.environment != "" && rt.environment != args[0] {
					return fmt.Errorf("--environment %q conflicts with command environment %q", rt.environment, args[0])
				}
				rt.selectedEnvironment = args[0]
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return printVersion(cmd)
		},
	}
	root.Flags().BoolP("version", "v", false, "Print version information")
	root.PersistentFlags().StringVar(&rt.workspace, "workspace", "", "Workspace directory")
	root.PersistentFlags().StringVar(&rt.stateDir, "state-dir", "", "Runtime store directory (config, state and data); relative to the workspace")
	root.PersistentFlags().StringVarP(&rt.environment, "environment", "e", "", "Environment installation to use for platform commands and the dashboard")
	root.AddCommand(
		newVersionCommand(), newInitCommand(&rt), newUpgradeCommand(ctx, &rt), newInstallCommand(ctx, &rt),
		newUninstallCommand(ctx, &rt), newDoctorCommand(ctx, &rt), newStatusCommand(ctx, &rt),
		newReleaseCommand(ctx, &rt), newDeployCommand(ctx, &rt), newRollbackCommand(ctx, &rt),
		newRemoveCommand(ctx, &rt), newSecretCommand(ctx, &rt), newCertificateCommand(ctx, &rt),
		newCleanupCommand(ctx, &rt), newUICommand(ctx, &rt), newLogsCommand(ctx, &rt),
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

func newInitCommand(rt *runtime) *cobra.Command {
	return &cobra.Command{Use: "init <workspace>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		directory, err := config.StateDirectory(args[0], rt.options())
		if err != nil {
			return err
		}
		paths, err := workspace.InitializeAt(args[0], directory, config.Version)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "initialized No Oops workspace at %s (runtime store: %s)\n", paths.Root, paths.Store)
		return err
	}}
}
