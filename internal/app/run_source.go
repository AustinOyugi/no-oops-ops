package app

import (
	"context"
	"errors"
	"os"

	"github.com/AustinOyugi/no-oops-ops/internal/release"
)

func (a *App) runSource(ctx context.Context, args []string) error {
	if len(args) != 4 || args[0] != "credential" || args[1] != "set" {
		return errors.New("usage: noops source credential set <environment> <key>")
	}
	value, err := secretValueInput(os.Stdin, os.Stderr)
	if err != nil {
		return err
	}
	result, err := a.secrets.Set(ctx, args[2], release.GitCredentialSecretKey(args[3]), value)
	if err != nil {
		return err
	}
	a.logger.InfoContext(ctx, "source credential saved", "environment", args[2], "key", args[3], "swarm_name", result.SwarmName, "version", result.Version)
	return nil
}
