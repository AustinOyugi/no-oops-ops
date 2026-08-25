package local

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const serviceReadyTimeout = 45 * time.Second

func (h *Host) waitForServiceReady(ctx context.Context, service string) error {
	deadline := time.Now().Add(serviceReadyTimeout)
	for {
		desired, running, taskError, err := h.InspectServiceReadiness(ctx, service)
		if err != nil {
			return err
		}
		if desired > 0 && desired == running {
			return nil
		}
		// Docker Desktop can briefly reject a bind mount immediately after its
		// source file is created on macOS. Swarm retries the task while the file
		// share catches up, so keep polling instead of failing the install early.
		if taskError != "" && !isRetryableDockerDesktopBindMountError(taskError) {
			return fmt.Errorf("service %q is running %d/%d desired tasks: %s", service, running, desired, taskError)
		}
		if time.Now().After(deadline) {
			if taskError != "" {
				return fmt.Errorf("service %q did not become ready within %s (running %d/%d desired tasks): %s", service, serviceReadyTimeout, running, desired, taskError)
			}
			return fmt.Errorf("service %q did not become ready within %s (running %d/%d desired tasks)", service, serviceReadyTimeout, running, desired)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func isRetryableDockerDesktopBindMountError(taskError string) bool {
	return strings.Contains(taskError, `invalid mount config for type "bind"`) &&
		strings.Contains(taskError, "bind source path does not exist:") &&
		strings.Contains(taskError, "/host_mnt/")
}
