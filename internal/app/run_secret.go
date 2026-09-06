package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/AustinOyugi/no-oops-ops/internal/secret"
	"golang.org/x/term"
)

func (a *App) runSecret(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("secret requires a subcommand: set, delete, or list")
	}

	switch args[0] {
	case "set":
		if len(args) != 3 {
			return errors.New("secret set requires an environment and key; provide the value through standard input")
		}

		value, err := secretValueInput(os.Stdin, os.Stderr)
		if err != nil {
			return err
		}

		result, err := a.secrets.Set(ctx, args[1], args[2], value)
		if err != nil {
			return err
		}

		a.logger.InfoContext(ctx, "secret created", "environment", result.Environment, "key", result.Key, "version", result.Version, "swarm_name", result.SwarmName)
		return nil
	case "delete":
		if len(args) != 3 {
			return errors.New("secret delete requires an environment and key")
		}

		items, err := a.secrets.Delete(ctx, args[1], args[2])
		if err != nil {
			return err
		}
		for _, item := range items {
			a.logger.InfoContext(ctx, "secret deleted", "environment", item.Environment, "key", item.Key, "version", item.Version, "swarm_name", item.SwarmName)
		}
		return nil
	case "list":
		if len(args) != 2 {
			return errors.New("secret list requires an environment")
		}

		items, err := a.secrets.List(ctx, args[1])
		if err != nil {
			return err
		}

		for _, item := range items {
			a.logger.InfoContext(ctx, "secret", "environment", item.Environment, "key", item.Key, "version", item.Version, "swarm_name", item.SwarmName, "created_at", item.CreatedAt)
		}
		a.logger.InfoContext(ctx, "secret list completed", "environment", args[1], "secrets", len(items))
		return nil
	default:
		return errors.New("unknown secret subcommand")
	}
}

// SetSecret stores a new version of an environment secret.
func (a *App) SetSecret(ctx context.Context, environment, key string, value io.Reader) (secret.Metadata, error) {
	return a.secrets.Set(ctx, environment, key, value)
}

// SetSecretFromStdin reads a secret value from standard input or a hidden prompt.
func (a *App) SetSecretFromStdin(ctx context.Context, environment, key string) error {
	value, err := secretValueInput(os.Stdin, os.Stderr)
	if err != nil {
		return err
	}
	result, err := a.SetSecret(ctx, environment, key, value)
	if err != nil {
		return err
	}
	a.logger.InfoContext(ctx, "secret created", "environment", result.Environment, "key", result.Key, "version", result.Version, "swarm_name", result.SwarmName)
	return nil
}

// DeleteSecret deletes all versions of an environment secret.
func (a *App) DeleteSecret(ctx context.Context, environment, key string) ([]secret.Metadata, error) {
	return a.secrets.Delete(ctx, environment, key)
}

// DeleteSecretAndLog deletes all secret versions and reports each deletion.
func (a *App) DeleteSecretAndLog(ctx context.Context, environment, key string) error {
	items, err := a.DeleteSecret(ctx, environment, key)
	if err != nil {
		return err
	}
	for _, item := range items {
		a.logger.InfoContext(ctx, "secret deleted", "environment", item.Environment, "key", item.Key, "version", item.Version, "swarm_name", item.SwarmName)
	}
	return nil
}

// ListSecrets lists metadata for environment secrets.
func (a *App) ListSecrets(ctx context.Context, environment string) ([]secret.Metadata, error) {
	return a.secrets.List(ctx, environment)
}

// ListSecretsAndLog reports environment secret metadata.
func (a *App) ListSecretsAndLog(ctx context.Context, environment string) error {
	items, err := a.ListSecrets(ctx, environment)
	if err != nil {
		return err
	}
	for _, item := range items {
		a.logger.InfoContext(ctx, "secret", "environment", item.Environment, "key", item.Key, "version", item.Version, "swarm_name", item.SwarmName, "created_at", item.CreatedAt)
	}
	a.logger.InfoContext(ctx, "secret list completed", "environment", environment, "secrets", len(items))
	return nil
}

func secretValueInput(stdin *os.File, stderr io.Writer) (io.Reader, error) {
	info, err := stdin.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect secret input: %w", err)
	}

	if info.Mode()&os.ModeCharDevice == 0 {
		return stdin, nil
	}

	if _, err := fmt.Fprint(stderr, "Secret value: "); err != nil {
		return nil, fmt.Errorf("write secret prompt: %w", err)
	}
	value, err := term.ReadPassword(int(stdin.Fd()))
	if _, writeErr := fmt.Fprintln(stderr); writeErr != nil && err == nil {
		return nil, fmt.Errorf("finish secret prompt: %w", writeErr)
	}
	if err != nil {
		return nil, fmt.Errorf("read secret value: %w", err)
	}

	return bytes.NewReader(value), nil
}
