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

func target(environment string, args []string, flags targetFlags) app.Target {
	return app.Target{Environment: environment, App: args[len(args)-1], Service: flags.service, All: flags.all}
}

// requiresEnvironment identifies commands whose target is environment-scoped.
func requiresEnvironment(cmd *cobra.Command) bool {
	switch cmd.Name() {
	case "deploy", "release", "rollback", "remove", "logs":
		return true
	}
	parent := cmd.Parent()
	return parent != nil && (parent.Name() == "secret" || parent.Name() == "release")
}

// legacyEnvironment recognizes only unambiguous positional environment forms.
func legacyEnvironment(cmd *cobra.Command, args []string) string {
	expected := 1
	if parent := cmd.Parent(); parent != nil && parent.Name() == "secret" && cmd.Name() == "list" {
		expected = 0
	}
	if requiresEnvironment(cmd) && len(args) == expected+1 {
		return args[0]
	}
	return ""
}
