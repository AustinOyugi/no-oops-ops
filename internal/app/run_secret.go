package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// SetSecret reads and stores a new version of an environment secret.
func (a *App) SetSecret(ctx context.Context, environment, key string) error {
	value, err := secretValueInput(os.Stdin, os.Stderr)
	if err != nil {
		return err
	}
	result, err := a.secrets.Set(ctx, environment, key, value)
	if err != nil {
		return err
	}
	a.logger.InfoContext(ctx, "secret created", "environment", result.Environment, "key", result.Key, "version", result.Version, "swarm_name", result.SwarmName)
	return nil
}

// DeleteSecret deletes all versions of an environment secret.
func (a *App) DeleteSecret(ctx context.Context, environment, key string) error {
	items, err := a.secrets.Delete(ctx, environment, key)
	if err != nil {
		return err
	}
	for _, item := range items {
		a.logger.InfoContext(ctx, "secret deleted", "environment", item.Environment, "key", item.Key, "version", item.Version, "swarm_name", item.SwarmName)
	}
	return nil
}

// ListSecrets reports environment secret metadata.
func (a *App) ListSecrets(ctx context.Context, environment string) error {
	items, err := a.secrets.List(ctx, environment)
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
