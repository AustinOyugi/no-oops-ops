package local

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/install"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

func (h *Host) inspectRegistryService(ctx context.Context) bool {
	err := h.InspectRegistryService(ctx)
	return err == nil
}

func (h *Host) InspectRegistryService(ctx context.Context) error {
	result, err := h.runner.Run(
		ctx,
		"docker",
		[]string{"service", "inspect", h.registryService},
		command.RunOptions{},
	)
	if err != nil {
		return fmt.Errorf("inspect registry service %q: %w: %s", h.registryService, err, strings.TrimSpace(string(result.Output)))
	}

	return nil
}

func (h *Host) InspectRegistryReachability(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+h.registryPort+"/v2/", nil)
	if err != nil {
		return fmt.Errorf("create registry health request: %w", err)
	}

	response, err := (&http.Client{Timeout: 3 * time.Second}).Do(request)
	if err != nil {
		return fmt.Errorf("reach registry API: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("registry API returned %s", response.Status)
	}

	return nil
}

func (h *Host) EnsureRegistry(ctx context.Context) error {
	h.logger.InfoContext(
		ctx,
		"ensuring registry",
		"name", h.registryName,
		"port", h.registryPort,
	)

	if h.inspectRegistryService(ctx) {
		h.registryReady = true
		return nil
	}

	result, err := h.runner.Run(
		ctx,
		"docker",
		[]string{
			"stack", "deploy",
			"--detach=true",
			"--compose-file", h.registryStackPath(),
			h.registryName,
		},
		command.RunOptions{
			StreamOutput: true,
			LogCommand:   true,
			Stdout:       os.Stdout,
			Stderr:       os.Stderr,
		},
	)
	if err != nil {
		return install.PrerequisiteError{
			Check: install.StepEnsureRegistry,
			Err:   fmt.Errorf("deploy registry stack %q: %w: %s", h.registryName, err, strings.TrimSpace(string(result.Output))),
		}
	}

	h.registryReady = h.inspectRegistryService(ctx)

	return nil
}
