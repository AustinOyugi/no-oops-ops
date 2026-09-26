package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/catalog"
	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/upgrade"
	"github.com/AustinOyugi/no-oops-ops/internal/workspace"
	"github.com/spf13/cobra"
)

func newUpgradeCommand(ctx context.Context, rt *runtime) *cobra.Command {
	var target string
	var check, dryRun, yes, allowDowngrade bool
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Explicitly upgrade the No Oops CLI",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if check && target != "" {
				return fmt.Errorf("--check and --to cannot be used together")
			}
			if !check && target == "" {
				return fmt.Errorf("upgrade requires --to <version|latest> or --check")
			}
			if yes && target == "latest" {
				return fmt.Errorf("--yes cannot be used with --to latest; pin an exact release tag for non-interactive upgrades")
			}
			root, err := rt.workspaceRoot()
			if err != nil {
				return err
			}
			paths, err := workspace.Open(root)
			if err != nil {
				return err
			}
			catalogFile, err := catalog.Load(paths.Root)
			if err != nil {
				return err
			}
			repository := strings.TrimSpace(catalogFile.Settings.Upgrade.Repository)
			if repository == "" {
				repository = upgrade.DefaultRepository
			}
			if err := upgrade.ValidateRepository(repository); err != nil {
				return err
			}
			client := upgrade.NewClient()
			if check {
				latest, err := client.LatestVersion(ctx, repository)
				if err != nil {
					return err
				}
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Current version: %s\nLatest version:  %s\nRepository:      %s\n", config.Version, latest, repository)
				return err
			}
			if target == "latest" {
				target, err = client.LatestVersion(ctx, repository)
				if err != nil {
					return err
				}
			}
			if err := upgrade.ValidateReleaseTag(target); err != nil {
				return err
			}
			if comparison, err := upgrade.CompareVersions(config.Version, target); err == nil && comparison > 0 && !allowDowngrade {
				return fmt.Errorf("target %s is older than current version %s; pass --allow-downgrade to proceed", target, config.Version)
			}

			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "No Oops upgrade\n\nCurrent version: %s\nTarget version:  %s\nCatalog version: %s\nRepository:      %s\nWorkspace:       %s\n", config.Version, target, catalogFile.Version, repository, paths.Root); err != nil {
				return err
			}
			catalogTarget := upgrade.CatalogVersion(target)
			if upgrade.CatalogVersion(config.Version) == catalogTarget && upgrade.CatalogVersion(catalogFile.Version) == catalogTarget {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "\nNo change required.")
				return err
			}
			if dryRun {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "\nDry run; no changes made.")
				return err
			}
			if !yes {
				confirmed, err := confirmUpgrade(cmd.InOrStdin(), cmd.OutOrStdout())
				if err != nil {
					return err
				}
				if !confirmed {
					_, err := fmt.Fprintln(cmd.OutOrStdout(), "Upgrade cancelled.")
					return err
				}
			}

			if upgrade.CatalogVersion(config.Version) == catalogTarget {
				if _, _, err := adoptCatalogVersion(paths.Root, catalogTarget); err != nil {
					return err
				}
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "Catalog adopted %s; the CLI was already at that version.\n", catalogTarget)
				return err
			}
			temporary, err := os.MkdirTemp("", "noops-upgrade-*")
			if err != nil {
				return fmt.Errorf("create upgrade directory: %w", err)
			}
			defer os.RemoveAll(temporary)
			candidate, err := client.Download(ctx, repository, target, temporary)
			if err != nil {
				return err
			}
			rollback, changed, err := adoptCatalogVersion(paths.Root, catalogTarget)
			if err != nil {
				return err
			}
			if err := upgrade.ReplaceExecutable(candidate); err != nil {
				if changed {
					if rollbackErr := rollback(); rollbackErr != nil {
						return fmt.Errorf("install upgraded executable: %w; also failed to restore apps.yml: %v", err, rollbackErr)
					}
				}
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Upgraded No Oops from %s to %s.\nRun noops install to reconcile managed platform services.\n", config.Version, target)
			return err
		},
	}
	cmd.Flags().StringVar(&target, "to", "", "Target release tag, or latest")
	cmd.Flags().BoolVar(&check, "check", false, "Show the latest available release without upgrading")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the version switch without making changes")
	cmd.Flags().BoolVar(&yes, "yes", false, "Proceed without the interactive confirmation")
	cmd.Flags().BoolVar(&allowDowngrade, "allow-downgrade", false, "Allow switching to an older release")
	return cmd
}

func adoptCatalogVersion(root, target string) (func() error, bool, error) {
	file, err := catalog.Load(root)
	if err != nil {
		return nil, false, err
	}
	if file.Version == target {
		return func() error { return nil }, false, nil
	}
	rollback, err := upgrade.UpdateCatalogVersion(root, target)
	return rollback, err == nil, err
}

func confirmUpgrade(input io.Reader, output io.Writer) (bool, error) {
	if _, err := fmt.Fprint(output, "\nContinue? [y/N] "); err != nil {
		return false, err
	}
	answer, err := bufio.NewReader(input).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("read upgrade confirmation: %w", err)
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}
