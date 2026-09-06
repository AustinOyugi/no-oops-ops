package cli

import (
	"context"

	"github.com/AustinOyugi/no-oops-ops/internal/app"
	"github.com/spf13/cobra"
)

func newCertificateCommand(_ context.Context, rt *runtime) *cobra.Command {
	cmd := &cobra.Command{Use: "certificate"}
	cmd.AddCommand(newAppCommand(rt, "import <name> <certificate.pem> <private-key.pem>", cobra.ExactArgs(3), func(application *app.App, args []string) error {
		return application.ImportCertificate(args[0], args[1], args[2])
	}))
	return cmd
}
