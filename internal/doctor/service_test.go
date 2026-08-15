package doctor

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
)

type fakeHost struct {
	dockerErr          error
	swarmState         string
	swarmErr           error
	swarmManagerErr    error
	sharedNetworkCalls int
	registryCalls      int
}

func (h *fakeHost) VerifyDocker(context.Context) error {
	return h.dockerErr
}

func (h *fakeHost) InspectSwarmState(context.Context) (string, error) {
	return h.swarmState, h.swarmErr
}

func (h *fakeHost) InspectSwarmManager(context.Context) error {
	return h.swarmManagerErr
}

func (h *fakeHost) InspectSharedNetwork(context.Context) error {
	h.sharedNetworkCalls++
	return nil
}

func (h *fakeHost) InspectRegistryService(context.Context) error {
	h.registryCalls++
	return nil
}

func TestRunInactiveSwarmSkipsSwarmDependentChecks(t *testing.T) {
	host := &fakeHost{swarmState: "inactive"}
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), config.Config{
		StateDir:     t.TempDir(),
		NetworkName:  "noops-net",
		RegistryName: "noops-registry",
	}, host)

	result, err := service.Run(context.Background())
	if err != nil {
		t.Fatalf("run doctor: %v", err)
	}

	if host.sharedNetworkCalls != 0 || host.registryCalls != 0 {
		t.Fatalf("swarm-dependent checks were run: network=%d registry=%d", host.sharedNetworkCalls, host.registryCalls)
	}

	checks := map[string]Check{}
	for _, check := range result.Checks {
		checks[check.Name] = check
	}

	if got := checks["swarm"].Status; got != StatusFail {
		t.Errorf("swarm status = %q, want %q", got, StatusFail)
	}
	for _, name := range []string{"swarm_manager", "shared_network", "registry_service", "registry_config", "registry_stack"} {
		if got := checks[name].Status; got != StatusSkip {
			t.Errorf("%s status = %q, want %q", name, got, StatusSkip)
		}
	}
	if got := checks["install_metadata"].Status; got != StatusFail {
		t.Errorf("install_metadata status = %q, want %q", got, StatusFail)
	}
	if got := checks["install_metadata"].Remediation; got != "Run noops install to create installation metadata" {
		t.Errorf("install_metadata remediation = %q", got)
	}
}

func TestRunDockerUnavailableSkipsDockerDependentChecks(t *testing.T) {
	host := &fakeHost{dockerErr: errors.New("docker is not running")}
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), config.Config{StateDir: t.TempDir()}, host)

	result, err := service.Run(context.Background())
	if err != nil {
		t.Fatalf("run doctor: %v", err)
	}

	if host.sharedNetworkCalls != 0 || host.registryCalls != 0 {
		t.Fatalf("docker-dependent checks were run: network=%d registry=%d", host.sharedNetworkCalls, host.registryCalls)
	}
	if result.Count(StatusFail) != 2 {
		t.Errorf("failed checks = %d, want 2", result.Count(StatusFail))
	}
	if result.Count(StatusSkip) != 6 {
		t.Errorf("skipped checks = %d, want 6", result.Count(StatusSkip))
	}
}

func TestRunNonManagerSkipsManagerDependentChecks(t *testing.T) {
	host := &fakeHost{swarmState: "active", swarmManagerErr: errors.New("not a manager")}
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), config.Config{StateDir: t.TempDir()}, host)

	result, err := service.Run(context.Background())
	if err != nil {
		t.Fatalf("run doctor: %v", err)
	}

	if host.sharedNetworkCalls != 0 || host.registryCalls != 0 {
		t.Fatalf("manager-dependent checks were run: network=%d registry=%d", host.sharedNetworkCalls, host.registryCalls)
	}

	checks := map[string]Check{}
	for _, check := range result.Checks {
		checks[check.Name] = check
	}
	if got := checks["swarm_manager"].Status; got != StatusFail {
		t.Errorf("swarm_manager status = %q, want %q", got, StatusFail)
	}
	for _, name := range []string{"shared_network", "registry_service"} {
		if got := checks[name].Status; got != StatusSkip {
			t.Errorf("%s status = %q, want %q", name, got, StatusSkip)
		}
	}
}

func TestRunDeployReadinessExcludesInstallationArtifactChecks(t *testing.T) {
	host := &fakeHost{swarmState: "active"}
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), config.Config{StateDir: t.TempDir()}, host)

	result, err := service.RunProfile(context.Background(), ProfileDeployReadiness)
	if err != nil {
		t.Fatalf("run deploy-readiness doctor: %v", err)
	}

	if len(result.Checks) != 5 {
		t.Fatalf("checks = %d, want 5", len(result.Checks))
	}
	for _, check := range result.Checks {
		if check.Status != StatusOK {
			t.Errorf("%s status = %q, want %q", check.Name, check.Status, StatusOK)
		}
	}
}
