package release

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// acquireBuildLock serializes image builds per workspace. A single-server
// deployment target should never execute competing builds by accident.
func (s *Service) acquireBuildLock(ctx context.Context) (func(), error) {
	if err := os.MkdirAll(s.config.StateDir, 0o700); err != nil {
		return nil, fmt.Errorf("create build lock directory: %w", err)
	}
	file, err := os.OpenFile(filepath.Join(s.config.StateDir, "build.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open build lock: %w", err)
	}
	for {
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() {
				_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
				_ = file.Close()
			}, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			_ = file.Close()
			return nil, fmt.Errorf("acquire build lock: %w", err)
		}
		select {
		case <-ctx.Done():
			_ = file.Close()
			return nil, fmt.Errorf("wait for build lock: %w", ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
	}
}
