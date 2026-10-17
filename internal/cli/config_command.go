package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/AustinOyugi/no-oops-ops/internal/environment"
	"github.com/AustinOyugi/no-oops-ops/internal/state"
	"github.com/spf13/cobra"
)

func newConfigCommand(rt *runtime) *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Manage reusable environment config files"}
	for _, verb := range []string{"install", "upgrade"} {
		cmd.AddCommand(&cobra.Command{Use: verb + " <name> <file>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
			if !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`).MatchString(args[0]) {
				return fmt.Errorf("config name must contain only letters, digits, underscores and hyphens")
			}
			root, err := rt.workspaceRoot()
			if err != nil {
				return err
			}
			target := filepath.Join(root, "configs", args[0]+".yml")
			unlock, err := state.AcquireLock(cmd.Context(), target+".lock")
			if err != nil {
				return err
			}
			defer unlock()
			_, err = os.Stat(target)
			if cmd.Name() == "install" && err == nil {
				return fmt.Errorf("config %q already installed; use config upgrade", args[0])
			}
			if cmd.Name() == "upgrade" && os.IsNotExist(err) {
				return fmt.Errorf("config %q is not installed; use config install", args[0])
			}
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			if _, err := environment.Load(args[1]); err != nil {
				return err
			}
			data, err := os.ReadFile(args[1])
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				return err
			}
			if err := state.WriteFile(target, data, 0600); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "config %s: %s\n", cmd.Name(), target)
			return err
		}})
	}
	return cmd
}
