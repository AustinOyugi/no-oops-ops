package install

import "context"

type Host interface {
	VerifyDocker(ctx context.Context) error
	PrepareStateDir(ctx context.Context) error
}
