package ingress

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// rollbackOnFailure preserves the last working files and route state across
// validation, reload and certificate failures. Call while holding operation.lock.
func (s *Service) rollbackOnFailure(ctx context.Context) (func(*error), error) {
	saved := map[string][]byte{}
	for _, root := range []string{s.configDir(), s.routesPath()} {
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
		var restoreErr error
		if err := os.RemoveAll(s.configDir()); err != nil {
			restoreErr = errors.Join(restoreErr, err)
		}
		if err := os.Remove(s.routesPath()); err != nil && !os.IsNotExist(err) {
			restoreErr = errors.Join(restoreErr, err)
		}
		for path, data := range saved {
			restoreErr = errors.Join(restoreErr, atomicWrite(path, data))
		}
		if restoreErr != nil {
			*operationErr = errors.Join(*operationErr, fmt.Errorf("restore ingress: %w", restoreErr))
			return
		}
		// A previous reload may already have succeeded before a later step failed.
		restoreCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if err := s.reload(restoreCtx); err != nil {
			*operationErr = errors.Join(*operationErr, fmt.Errorf("reload restored ingress: %w", err))
		}
	}, nil
}
