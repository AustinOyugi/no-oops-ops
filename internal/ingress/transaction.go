package ingress

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// RecoveryError means ingress may still serve the candidate. Deployment cleanup
// must retain that service until an operator can complete recovery.
type RecoveryError struct{ Err error }

func (e *RecoveryError) Error() string { return "ingress recovery incomplete: " + e.Err.Error() }
func (e *RecoveryError) Unwrap() error { return e.Err }

// rollbackOnFailure preserves the last working files and route state across
// validation, reload and certificate failures. Call while holding operation.lock.
func (s *Service) rollbackOnFailure(ctx context.Context) (func(*error), error) {
	saved := map[string][]byte{}
	for _, root := range []string{filepath.Join(s.ingressDir(), "nginx.conf"), s.routesPath()} {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			saved[path] = data
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("snapshot ingress: %w", err)
		}
	}
	return func(operationErr *error) {
		if *operationErr == nil {
			return
		}
		// Validation can fail before anything is published. Keep the healthy
		// workers untouched when both files still match the snapshot.
		changed := false
		for path, data := range saved {
			current, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(current, data) {
				changed = true
			}
		}
		if _, exists := saved[s.routesPath()]; !exists {
			if _, err := os.Stat(s.routesPath()); err == nil || !os.IsNotExist(err) {
				changed = true
			}
		}
		if !changed {
			return
		}
		var restoreErr error
		if _, exists := saved[s.routesPath()]; !exists {
			if err := os.Remove(s.routesPath()); err != nil && !os.IsNotExist(err) {
				restoreErr = errors.Join(restoreErr, err)
			}
		}
		for path, data := range saved {
			restoreErr = errors.Join(restoreErr, atomicWrite(path, data))
		}
		if restoreErr != nil {
			*operationErr = errors.Join(*operationErr, &RecoveryError{Err: fmt.Errorf("restore ingress: %w", restoreErr)})
			return
		}
		if _, exists := saved[filepath.Join(s.ingressDir(), "nginx.conf")]; !exists {
			return
		}
		// A previous reload may already have succeeded before a later step failed.
		restoreCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if err := s.reload(restoreCtx); err != nil {
			*operationErr = errors.Join(*operationErr, &RecoveryError{Err: fmt.Errorf("reload restored ingress: %w", err)})
		}
	}, nil
}
