package cli

import (
	"github.com/AustinOyugi/no-oops-ops/internal/app"
	"github.com/spf13/cobra"
)

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
