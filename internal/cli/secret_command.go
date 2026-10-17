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
	}{{"set", "set <key>", 1}, {"delete", "delete <key>", 1}, {"list", "list", 0}} {
		spec := spec
		cmd.AddCommand(newAppCommand(rt, spec.use, cobra.RangeArgs(spec.count, spec.count+1), func(application *app.App, args []string) error {
			switch spec.verb {
			case "set":
				return application.SetSecret(ctx, rt.selectedEnvironment, args[len(args)-1])
			case "delete":
				return application.DeleteSecret(ctx, rt.selectedEnvironment, args[len(args)-1])
			case "list":
				return application.ListSecrets(ctx, rt.selectedEnvironment)
			default:
				return fmt.Errorf("unsupported secret command %q", spec.verb)
			}
		}))
	}
	return cmd
}
