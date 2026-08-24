package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/deploy"
	"github.com/AustinOyugi/no-oops-ops/internal/doctor"
)

type failingDoctor struct{}

func (failingDoctor) RunProfile(context.Context, doctor.Profile) (doctor.Result, error) {
	result := doctor.Result{}
	result.Add("registry_service", doctor.StatusFail, "registry service is unavailable", "Run noops install to deploy the registry")
	return result, nil
}

func TestRunRoutesVersionCommand(t *testing.T) {
	var logs bytes.Buffer
	application := &App{
		logger: slog.New(slog.NewTextHandler(&logs, nil)),
		config: config.Config{InstallVersion: "test-version"},
	}

	if err := application.Run(context.Background(), []string{"version"}); err != nil {
		t.Fatalf("Run(version) returned error: %v", err)
	}
	if got := logs.String(); !strings.Contains(got, "version=test-version") {
		t.Errorf("Run(version) logs = %q, want version", got)
	}
}

type recordingDeployer struct {
	runCalls int
}

func (d *recordingDeployer) Run(context.Context, string, string, string) (deploy.Result, error) {
	d.runCalls++
	return deploy.Result{}, errors.New("deployer should not run")
}

func (d *recordingDeployer) RunWithOptions(context.Context, string, string, string, deploy.RunOptions) (deploy.Result, error) {
	d.runCalls++
	return deploy.Result{}, errors.New("deployer should not run")
}

func (d *recordingDeployer) Rollback(context.Context, string, string) (deploy.Result, error) {
	return deploy.Result{}, nil
}

func (d *recordingDeployer) Remove(context.Context, string, string) (deploy.RemoveResult, error) {
	return deploy.RemoveResult{}, nil
}

func TestRunDeployStopsBeforeDeployingWhenPreflightFails(t *testing.T) {
	deployer := &recordingDeployer{}
	application := &App{
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		deployer: deployer,
		doctor:   failingDoctor{},
	}

	err := application.runDeploy(context.Background(), []string{"prod", "app.yml", "--service", "api"})
	if err == nil {
		t.Fatal("runDeploy returned nil error")
	}
	if !strings.Contains(err.Error(), "Run noops install to deploy the registry") {
		t.Errorf("error = %q, want remediation", err)
	}
	if deployer.runCalls != 0 {
		t.Errorf("deployer calls = %d, want 0", deployer.runCalls)
	}
}

func TestParseDeployArgsQuick(t *testing.T) {
	environment, path, services, quick, err := parseDeployArgs([]string{"--quick", "dev", "app.yml", "--service", "api"})
	if err != nil {
		t.Fatalf("parseDeployArgs returned error: %v", err)
	}
	if environment != "dev" || path != "app.yml" || len(services) != 1 || services[0] != "api" || !quick {
		t.Errorf("parseDeployArgs = (%q, %q, %v, %t), want quick dev deployment", environment, path, services, quick)
	}
}
