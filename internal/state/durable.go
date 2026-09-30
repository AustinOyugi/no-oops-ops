// Package state contains the small durable primitives used to keep the
// filesystem projection of platform state consistent with Docker.
package state

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// WriteFile atomically replaces path with data and synchronizes both the file
// and its containing directory before reporting success.
func WriteFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create state directory %q: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary state file: %w", err)
	}

	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set state file permissions: %w", err)
	}

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary state file: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temporary state file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary state file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace state file %q: %w", path, err)
	}

	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open state directory %q: %w", dir, err)
	}
	defer d.Close()

	if err := d.Sync(); err != nil {
		return fmt.Errorf("sync state directory %q: %w", dir, err)
	}

	return nil
}

// AcquireLock serializes a state transition. The advisory lock is released if
// the process dies, and honors cancellation while waiting.
func AcquireLock(ctx context.Context, path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock: %w", err)
	}

	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() {
				_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
				_ = f.Close()
			}, nil
		}

		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			_ = f.Close()
			return nil, fmt.Errorf("acquire lock: %w", err)
		}

		select {
		case <-ctx.Done():
			_ = f.Close()
			return nil, ctx.Err()

		case <-time.After(100 * time.Millisecond):
		}
	}
}
