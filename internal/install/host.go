package install

import "context"

type Host interface {
	VerifyDocker(ctx context.Context) error
	EnsureSwarmInitialized(ctx context.Context) error
	EnsureSharedNetwork(ctx context.Context) error
	WriteRegistryConfig(ctx context.Context) error
	EnsureRegistry(ctx context.Context) error

	PrepareStateDir(ctx context.Context) error
	InitializeLocalState(ctx context.Context) error
	WriteInstallMetadata(ctx context.Context) error
}
