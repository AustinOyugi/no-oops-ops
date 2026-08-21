package app

import (
	"context"
	"errors"
	"github.com/AustinOyugi/no-oops-ops/internal/release"
	"io"
	"log/slog"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/deploy"
	"github.com/AustinOyugi/no-oops-ops/internal/doctor"
	"github.com/AustinOyugi/no-oops-ops/internal/install"
	"github.com/AustinOyugi/no-oops-ops/internal/install/local"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/logging"
	"github.com/AustinOyugi/no-oops-ops/internal/secret"
	"github.com/AustinOyugi/no-oops-ops/internal/status"
	"github.com/AustinOyugi/no-oops-ops/internal/uninstall"
)

type App struct {
	logger      *slog.Logger
	config      config.Config
	installer   *install.Installer
	uninstaller *uninstall.Service
	deployer    deployer
	releaser    *release.Service
	doctor      doctorRunner
	status      *status.Service
	secrets     secretRunner
}

type deployer interface {
	Run(ctx context.Context, environment string, path string, optionalReleaseVersion string) (deploy.Result, error)
	Rollback(ctx context.Context, environment string, path string) (deploy.Result, error)
	Remove(ctx context.Context, environment string, path string) (deploy.RemoveResult, error)
}

type doctorRunner interface {
	RunProfile(ctx context.Context, profile doctor.Profile) (doctor.Result, error)
}

type secretRunner interface {
	Set(context.Context, string, string, io.Reader) (secret.Metadata, error)
	List(context.Context, string) ([]secret.Metadata, error)
}

func New(cfg config.Config) (*App, error) {

	logger := logging.New()

	localHost := local.NewHost(
		logger, cfg.StateDir, cfg.DataDir, cfg.InstallVersion,
		cfg.NetworkName, cfg.RegistryName, cfg.RegistryPort)

	installer, err := install.New(logger, localHost)

	if err != nil {
		return nil, err
	}
	uninstaller, err := uninstall.New(localHost)
	if err != nil {
		return nil, err
	}

	return &App{
		logger:      logger,
		config:      cfg,
		installer:   installer,
		uninstaller: uninstaller,
		deployer:    deploy.NewService(logger, cfg),
		releaser:    release.NewService(logger, cfg),
		doctor:      doctor.NewService(logger, cfg, localHost),
		status:      status.NewService(logger, cfg, localHost),
		secrets:     secret.NewService(logger, cfg),
	}, nil
}

func (a *App) Run(ctx context.Context, args []string) error {
	if len(args) > 0 && args[0] == "version" {
		a.logger.InfoContext(ctx, "No Oops Ops", "version", a.config.InstallVersion)
		return nil
	}

	if len(args) > 0 && args[0] == "doctor" {
		return a.runDoctor(ctx, args[1:])
	}

	if len(args) > 0 && args[0] == "status" {
		return a.runStatus(ctx)
	}

	if len(args) > 0 && args[0] == "install" {
		return a.runInstall(ctx)
	}

	if len(args) > 0 && args[0] == "uninstall" {
		return a.runUninstall(ctx, args[1:])
	}

	if len(args) > 0 && args[0] == "deploy" {
		return a.runDeploy(ctx, args[1:])
	}

	if len(args) > 0 && args[0] == "rollback" {
		return a.runRollback(ctx, args[1:])
	}

	if len(args) > 0 && args[0] == "remove" {
		return a.runRemove(ctx, args[1:])
	}

	if len(args) > 0 && args[0] == "release" {
		return a.runRelease(ctx, args[1:])
	}

	if len(args) > 0 && args[0] == "secret" {
		return a.runSecret(ctx, args[1:])
	}

	if len(args) > 0 {
		a.logger.ErrorContext(ctx, "unknown command", "command", args[0])
		return errors.New("unknown command")
	}

	return a.runInstall(ctx)
}
