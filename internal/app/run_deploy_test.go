package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/deploy"
	"github.com/AustinOyugi/no-oops-ops/internal/doctor"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
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
	workspace := t.TempDir()
	writeCatalogApp(t, workspace)
	deployer := &recordingDeployer{}
	application := &App{
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		deployer: deployer,
		doctor:   failingDoctor{},
		config:   config.Config{Workspace: workspace},
	}

	err := application.runDeploy(context.Background(), []string{"prod", "api", "--service", "api"})
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
	resolve := func(name string) (string, error) { return name + ".yml", nil }
	environment, path, services, quick, err := parseDeployArgs([]string{"--quick", "dev", "api", "--service", "api"}, resolve)
	if err != nil {
		t.Fatalf("parseDeployArgs returned error: %v", err)
	}
	if environment != "dev" || path != "api.yml" || len(services) != 1 || services[0] != "api" || !quick {
		t.Errorf("parseDeployArgs = (%q, %q, %v, %t), want quick dev deployment", environment, path, services, quick)
	}
}

func TestParseDeployArgsSelectsTheOnlyServiceImplicitly(t *testing.T) {
	manifestPath := filepath.Join(t.TempDir(), "app.yml")
	if err := os.WriteFile(manifestPath, []byte("services:\n  api:\n    image: api\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolve := func(string) (string, error) { return manifestPath, nil }

	environment, path, services, quick, err := parseDeployArgs([]string{"dev", "api"}, resolve)
	if err != nil {
		t.Fatalf("parseDeployArgs returned error: %v", err)
	}
	if environment != "dev" || path != manifestPath || len(services) != 1 || services[0] != "api" || quick {
		t.Errorf("parseDeployArgs = (%q, %q, %v, %t), want implicit api deployment", environment, path, services, quick)
	}
}

func TestParseDeployArgsRequiresSelectionForMultipleServices(t *testing.T) {
	manifestPath := filepath.Join(t.TempDir(), "app.yml")
	if err := os.WriteFile(manifestPath, []byte("services:\n  api:\n    image: api\n  worker:\n    image: worker\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolve := func(string) (string, error) { return manifestPath, nil }

	_, _, _, _, err := parseDeployArgs([]string{"dev", "api"}, resolve)
	if err == nil || !strings.Contains(err.Error(), "multiple services") {
		t.Fatalf("parseDeployArgs error = %v, want multiple-services selection error", err)
	}
}

func TestRequiresACME(t *testing.T) {
	plainTLS := manifest.Manifest{Expose: manifest.Expose{TLS: true}}
	if !requiresACME(plainTLS, false) {
		t.Fatal("expected plain TLS route to require ACME")
	}
	if requiresACME(plainTLS, true) {
		t.Fatal("Cloudflare TLS route must not require ACME")
	}
	imported := manifest.Manifest{Expose: manifest.Expose{TLSCertificate: "cloudflare-origin"}}
	if requiresACME(imported, false) {
		t.Fatal("imported certificate must not require ACME")
	}
}

func TestValidateIngressTLSRequiresImportedCloudflareCertificate(t *testing.T) {
	err := validateIngressTLS(manifest.Manifest{Expose: manifest.Expose{TLS: true}}, true)
	if err == nil || !strings.Contains(err.Error(), "tls_certificate") {
		t.Fatalf("error = %v, want missing tls_certificate error", err)
	}
	if err := validateIngressTLS(manifest.Manifest{Expose: manifest.Expose{TLS: true, TLSCertificate: "cloudflare-origin"}}, true); err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
}

func writeCatalogApp(t *testing.T, workspace string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(workspace, "apps", "api"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "apps.yml"), []byte("apps:\n  api:\n    manifest: ./apps/api/app.yml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "apps", "api", "app.yml"), []byte("services:\n  api:\n    image: api\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}
