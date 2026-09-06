package cli

import (
	"context"
	"fmt"

	"github.com/AustinOyugi/no-oops-ops/internal/app"
	"github.com/spf13/cobra"
)

func newSecretCommand(ctx context.Context, rt *runtime) *cobra.Command {
	cmd := &cobra.Command{Use: "secret"}
	for _, spec := range []struct {
		verb, use string
		count     int
	}{{"set", "set <environment> <key>", 2}, {"delete", "delete <environment> <key>", 2}, {"list", "list <environment>", 1}} {
		spec := spec
		cmd.AddCommand(newAppCommand(rt, spec.use, cobra.ExactArgs(spec.count), func(application *app.App, args []string) error {
			switch spec.verb {
			case "set":
				return application.SetSecret(ctx, args[0], args[1])
			case "delete":
				return application.DeleteSecret(ctx, args[0], args[1])
			case "list":
				return application.ListSecrets(ctx, args[0])
			default:
				return fmt.Errorf("unsupported secret command %q", spec.verb)
			}
		}))
	}
	return cmd
}
