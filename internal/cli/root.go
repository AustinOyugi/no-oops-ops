package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/AustinOyugi/no-oops-ops/internal/app"
	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/workspace"
	"github.com/spf13/cobra"
)

type runtime struct{ workspace string }

func (r runtime) application() (*app.App, error) {
	root := r.workspace
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}
	cfg, err := config.Load(root)
	if err != nil {
		return nil, err
	}
	return app.New(cfg)
}

type targetFlags struct {
	service string
	all     bool
}

func addTargetFlags(cmd *cobra.Command, flags *targetFlags) {
	cmd.Flags().StringVar(&flags.service, "service", "", "Service name")
	cmd.Flags().BoolVar(&flags.all, "all", false, "Select all services")
	cmd.MarkFlagsMutuallyExclusive("service", "all")
}

func target(args []string, flags targetFlags) app.Target {
	return app.Target{Environment: args[0], App: args[1], Service: flags.service, All: flags.all}
}

// NewRootCommand constructs the No Oops command tree.
func NewRootCommand(ctx context.Context) *cobra.Command {
	rt := runtime{}
	root := &cobra.Command{Use: "noops", Short: "No Oops Ops deployment CLI", SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "noops %s\n", config.Version)
			return err
		},
	}
	root.PersistentFlags().StringVar(&rt.workspace, "workspace", "", "Workspace directory")
	commands := []*cobra.Command{newVersionCommand(), newInitCommand()}
	commands = append(commands, newLifecycleCommands(ctx, &rt)...)
	root.AddCommand(commands...)
	return root
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{Use: "version", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "noops %s\n", config.Version)
		return err
	}}
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
	legacy := func(use string, args cobra.PositionalArgs, command func() []string) *cobra.Command {
		return &cobra.Command{Use: use, Args: args, RunE: func(cmd *cobra.Command, _ []string) error {
			return withApp(func(application *app.App) error { return application.Run(ctx, command()) })
		}}
	}

	var releaseFlags targetFlags
	var deployAfterRelease bool
	releaseCmd := &cobra.Command{Use: "release <environment> <app>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return withApp(func(application *app.App) error {
			return application.Release(ctx, target(args, releaseFlags), deployAfterRelease)
		})
	}}
	addTargetFlags(releaseCmd, &releaseFlags)
	releaseCmd.Flags().BoolVar(&deployAfterRelease, "deploy", false, "Deploy the exact release after it is created")
	var listFlags targetFlags
	listCmd := &cobra.Command{Use: "list <environment> <app>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return withApp(func(application *app.App) error { return application.ListReleases(ctx, target(args, listFlags)) })
	}}
	addTargetFlags(listCmd, &listFlags)
	releaseCmd.AddCommand(listCmd)

	var deployFlags targetFlags
	var quick bool
	deployCmd := &cobra.Command{Use: "deploy <environment> <app>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return withApp(func(application *app.App) error { return application.Deploy(ctx, target(args, deployFlags), quick) })
	}}
	addTargetFlags(deployCmd, &deployFlags)
	deployCmd.Flags().BoolVar(&quick, "quick", false, "Use the health-check start period as the rollout monitor")

	var rollbackFlags targetFlags
	rollbackCmd := &cobra.Command{Use: "rollback <environment> <app>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return withApp(func(application *app.App) error { return application.Rollback(ctx, target(args, rollbackFlags)) })
	}}
	addTargetFlags(rollbackCmd, &rollbackFlags)
	var removeFlags targetFlags
	removeCmd := &cobra.Command{Use: "remove <environment> <app>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return withApp(func(application *app.App) error { return application.Remove(ctx, target(args, removeFlags)) })
	}}
	addTargetFlags(removeCmd, &removeFlags)

	installCmd := legacy("install", cobra.NoArgs, func() []string { return []string{"install"} })
	var purge bool
	uninstallCmd := legacy("uninstall", cobra.NoArgs, func() []string {
		if purge {
			return []string{"uninstall", "--purge"}
		}
		return []string{"uninstall"}
	})
	uninstallCmd.Flags().BoolVar(&purge, "purge", false, "Remove persistent registry data")
	var deployReady bool
	doctorCmd := legacy("doctor", cobra.NoArgs, func() []string {
		if deployReady {
			return []string{"doctor", "--deploy-ready"}
		}
		return []string{"doctor"}
	})
	doctorCmd.Flags().BoolVar(&deployReady, "deploy-ready", false, "Check deployment prerequisites only")
	statusCmd := legacy("status", cobra.NoArgs, func() []string { return []string{"status"} })
	var apply, orphaned bool
	var keep int
	cleanupCmd := legacy("cleanup", cobra.NoArgs, func() []string {
		out := []string{"cleanup"}
		if apply {
			out = append(out, "--apply")
		}
		if orphaned {
			out = append(out, "--orphaned")
		}
		return append(out, "--keep", fmt.Sprint(keep))
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
			return withApp(func(application *app.App) error {
				return application.Run(ctx, append([]string{"secret", s.verb}, args...))
			})
		}})
	}
	certificateCmd := &cobra.Command{Use: "certificate"}
	certificateCmd.AddCommand(&cobra.Command{Use: "import <name> <certificate.pem> <private-key.pem>", Args: cobra.ExactArgs(3), RunE: func(cmd *cobra.Command, args []string) error {
		return withApp(func(application *app.App) error {
			return application.Run(ctx, append([]string{"certificate", "import"}, args...))
		})
	}})
	return []*cobra.Command{installCmd, uninstallCmd, doctorCmd, statusCmd, releaseCmd, deployCmd, rollbackCmd, removeCmd, secretCmd, certificateCmd, cleanupCmd}
}
