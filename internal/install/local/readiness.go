package local

import (
	"context"
	"fmt"
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
		if taskError != "" {
			return fmt.Errorf("service %q is running %d/%d desired tasks: %s", service, running, desired, taskError)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("service %q did not become ready within %s (running %d/%d desired tasks)", service, serviceReadyTimeout, running, desired)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}
