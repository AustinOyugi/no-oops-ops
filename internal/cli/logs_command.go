package cli

import (
	"context"

	"github.com/AustinOyugi/no-oops-ops/internal/deploy"
	"github.com/spf13/cobra"
)

func newLogsCommand(ctx context.Context, rt *runtime) *cobra.Command {
	var service string
	options := deploy.LogOptions{}
	cmd := &cobra.Command{
		Use:   "logs <environment> <app>",
		Short: "Stream a deployed service's logs (Ctrl+C to stop)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.Validate(); err != nil {
				return err
			}
			application, err := rt.application()
			if err != nil {
				return err
			}
			return application.Logs(ctx, target(args, targetFlags{service: service}), options, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	cmd.Flags().StringVar(&service, "service", "", "Service name (required for multi-service apps)")
	cmd.Flags().BoolVarP(&options.Follow, "follow", "f", true, "Follow new log output; use --follow=false for a snapshot")
	cmd.Flags().StringVar(&options.Tail, "tail", "100", "Number of recent lines per task, or all")
	cmd.Flags().StringVar(&options.Since, "since", "", "Show logs since a timestamp or relative duration (e.g. 10m)")
	cmd.Flags().BoolVarP(&options.Timestamps, "timestamps", "t", false, "Include timestamps")
	return cmd
}
