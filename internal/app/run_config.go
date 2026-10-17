package app

import (
	"context"

	"github.com/AustinOyugi/no-oops-ops/internal/environment"
)

// InstallConfig installs shared configuration without requiring platform initialization.
func InstallConfig(ctx context.Context, root, name, source string) (string, error) {
	return environment.Install(ctx, root, name, source)
}

// UpgradeConfig replaces an installed shared configuration.
func UpgradeConfig(ctx context.Context, root, name, source string) (string, error) {
	return environment.Upgrade(ctx, root, name, source)
}
