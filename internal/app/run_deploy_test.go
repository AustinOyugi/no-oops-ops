package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/deploy"
	"github.com/AustinOyugi/no-oops-ops/internal/doctor"
)

type failingDoctor struct{}

func (failingDoctor) RunProfile(context.Context, doctor.Profile) (doctor.Result, error) {
	result := doctor.Result{}
	result.Add("registry_service", doctor.StatusFail, "registry service is unavailable", "Run noops install to deploy the registry")
	return result, nil
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

	err := application.runDeploy(context.Background(), []string{"prod", "app.yml"})
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
	environment, path, tag, quick, err := parseDeployArgs([]string{"--quick", "dev", "app.yml", "20260824-133119"})
	if err != nil {
		t.Fatalf("parseDeployArgs returned error: %v", err)
	}
	if environment != "dev" || path != "app.yml" || tag != "20260824-133119" || !quick {
		t.Errorf("parseDeployArgs = (%q, %q, %q, %t), want quick dev deployment", environment, path, tag, quick)
	}
}
