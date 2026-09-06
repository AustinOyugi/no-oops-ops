package cli

import (
	"context"
	"fmt"

	"github.com/AustinOyugi/no-oops-ops/internal/app"
	"github.com/AustinOyugi/no-oops-ops/internal/cleanup"
	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/workspace"
	"github.com/spf13/cobra"
)

// NewRootCommand constructs the No Oops command tree.
func NewRootCommand(ctx context.Context) *cobra.Command {
	rt := runtime{}
	root := &cobra.Command{Use: "noops", Short: "No Oops Ops deployment CLI", SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return printVersion(cmd)
		},
	}
	root.Flags().BoolP("version", "v", false, "Print version information")
	root.PersistentFlags().StringVar(&rt.workspace, "workspace", "", "Workspace directory")
	commands := []*cobra.Command{newVersionCommand(), newInitCommand()}
	commands = append(commands, newLifecycleCommands(ctx, &rt)...)
	root.AddCommand(commands...)
	return root
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{Use: "version", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
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

func newLifecycleCommands(ctx context.Context, rt *runtime) []*cobra.Command {
	withApp := func(run func(*app.App) error) error {
		application, err := rt.application()
		if err != nil {
			return err
		}
		return run(application)
	}
	
	newAppCommand := func(use string, args cobra.PositionalArgs, run func(*app.App, []string) error) *cobra.Command {
		return &cobra.Command{
			Use:  use,
			Args: args,
			RunE: func(_ *cobra.Command, commandArgs []string) error {
				return withApp(func(application *app.App) error {
					return run(application, commandArgs)
				})
			},
		}
	}

	var releaseFlags targetFlags
	var deployAfterRelease bool
	releaseCmd := newAppCommand("release <environment> <app>", cobra.ExactArgs(2), func(application *app.App, args []string) error {
		return application.Release(ctx, target(args, releaseFlags), deployAfterRelease)
	})
	addTargetFlags(releaseCmd, &releaseFlags)
	releaseCmd.Flags().BoolVar(&deployAfterRelease, "deploy", false, "Deploy the exact release after it is created")
	var listFlags targetFlags
	listCmd := newAppCommand("list <environment> <app>", cobra.ExactArgs(2), func(application *app.App, args []string) error {
		return application.ListReleases(ctx, target(args, listFlags))
	})
	addTargetFlags(listCmd, &listFlags)
	releaseCmd.AddCommand(listCmd)

	var deployFlags targetFlags
	var quick bool
	deployCmd := newAppCommand("deploy <environment> <app>", cobra.ExactArgs(2), func(application *app.App, args []string) error {
		return application.Deploy(ctx, target(args, deployFlags), quick)
	})
	addTargetFlags(deployCmd, &deployFlags)
	deployCmd.Flags().BoolVar(&quick, "quick", false, "Use the health-check start period as the rollout monitor")

	var rollbackFlags targetFlags
	rollbackCmd := newAppCommand("rollback <environment> <app>", cobra.ExactArgs(2), func(application *app.App, args []string) error {
		return application.Rollback(ctx, target(args, rollbackFlags))
	})
	addTargetFlags(rollbackCmd, &rollbackFlags)
	var removeFlags targetFlags
	removeCmd := newAppCommand("remove <environment> <app>", cobra.ExactArgs(2), func(application *app.App, args []string) error {
		return application.Remove(ctx, target(args, removeFlags))
	})
	addTargetFlags(removeCmd, &removeFlags)

	installCmd := newAppCommand("install", cobra.NoArgs, func(application *app.App, _ []string) error {
		return application.Install(ctx)
	})
	var purge bool
	uninstallCmd := newAppCommand("uninstall", cobra.NoArgs, func(application *app.App, _ []string) error {
		return application.Uninstall(ctx, purge)
	})
	uninstallCmd.Flags().BoolVar(&purge, "purge", false, "Remove persistent registry data")
	var deployReady bool
	doctorCmd := newAppCommand("doctor", cobra.NoArgs, func(application *app.App, _ []string) error {
		return application.Doctor(ctx, deployReady)
	})
	doctorCmd.Flags().BoolVar(&deployReady, "deploy-ready", false, "Check deployment prerequisites only")
	statusCmd := newAppCommand("status", cobra.NoArgs, func(application *app.App, _ []string) error {
		return application.Status(ctx)
	})
	var apply, orphaned bool
	var keep int
	cleanupCmd := newAppCommand("cleanup", cobra.NoArgs, func(application *app.App, _ []string) error {
		return application.Cleanup(ctx, cleanup.Options{Apply: apply, Orphaned: orphaned, Keep: keep})
	})
	cleanupCmd.Flags().BoolVar(&apply, "apply", false, "Apply cleanup")
	cleanupCmd.Flags().BoolVar(&orphaned, "orphaned", false, "Include orphaned app environments")
	cleanupCmd.Flags().IntVar(&keep, "keep", 2, "Number of records to retain")

	secretCmd := &cobra.Command{Use: "secret"}
	for _, spec := range []struct {
		verb, use string
		count     int
	}{{"set", "set <environment> <key>", 2}, {"delete", "delete <environment> <key>", 2}, {"list", "list <environment>", 1}} {
		s := spec
		secretCmd.AddCommand(&cobra.Command{Use: s.use, Args: cobra.ExactArgs(s.count), RunE: func(cmd *cobra.Command, args []string) error {
			return runSecretCommand(ctx, rt, s.verb, args)
		}})
	}
	certificateCmd := &cobra.Command{Use: "certificate"}
	certificateCmd.AddCommand(newAppCommand("import <name> <certificate.pem> <private-key.pem>", cobra.ExactArgs(3), func(application *app.App, args []string) error {
		return application.ImportCertificate(args[0], args[1], args[2])
	}))
	return []*cobra.Command{installCmd, uninstallCmd, doctorCmd, statusCmd, releaseCmd, deployCmd, rollbackCmd, removeCmd, secretCmd, certificateCmd, cleanupCmd}
}

func runSecretCommand(ctx context.Context, rt *runtime, verb string, args []string) error {
	application, err := rt.application()
	if err != nil {
		return err
	}
	switch verb {
	case "set":
		return application.SetSecret(ctx, args[0], args[1])
	case "delete":
		return application.DeleteSecret(ctx, args[0], args[1])
	case "list":
		return application.ListSecrets(ctx, args[0])
	default:
		return fmt.Errorf("unsupported secret command %q", verb)
	}
}
