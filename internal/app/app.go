package app

import (
	"context"
	"errors"
	"github.com/AustinOyugi/no-oops-ops/internal/release"
	"log/slog"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/deploy"
	"github.com/AustinOyugi/no-oops-ops/internal/doctor"
	"github.com/AustinOyugi/no-oops-ops/internal/install"
	"github.com/AustinOyugi/no-oops-ops/internal/install/local"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/logging"
	"github.com/AustinOyugi/no-oops-ops/internal/status"
)

type App struct {
	logger    *slog.Logger
	config    config.Config
	installer *install.Installer
	deployer  deployer
	releaser  *release.Service
	doctor    doctorRunner
	status    *status.Service
}

type deployer interface {
	Run(ctx context.Context, environment string, path string, optionalReleaseVersion string) (deploy.Result, error)
	Rollback(ctx context.Context, environment string, path string) (deploy.Result, error)
}

type doctorRunner interface {
	RunProfile(ctx context.Context, profile doctor.Profile) (doctor.Result, error)
}

func New(cfg config.Config) (*App, error) {

	logger := logging.New()

	localHost := local.NewHost(
		logger, cfg.StateDir, cfg.InstallVersion,
		cfg.NetworkName, cfg.RegistryName, cfg.RegistryPort)

	installer, err := install.New(logger, localHost)

	if err != nil {
		return nil, err
	}

	return &App{
		logger:    logger,
		config:    cfg,
		installer: installer,
		deployer:  deploy.NewService(logger, cfg),
		releaser:  release.NewService(logger, cfg),
		doctor:    doctor.NewService(logger, cfg, localHost),
		status:    status.NewService(logger, cfg, localHost),
	}, nil
}

func (a *App) Run(ctx context.Context, args []string) error {
	if len(args) > 0 && args[0] == "doctor" {
		return a.runDoctor(ctx, args[1:])
	}

	if len(args) > 0 && args[0] == "status" {
		return a.runStatus(ctx)
	}

	if len(args) > 0 && args[0] == "install" {
		return a.runInstall(ctx)
	}

	if len(args) > 0 && args[0] == "deploy" {
		return a.runDeploy(ctx, args[1:])
	}

	if len(args) > 0 && args[0] == "rollback" {
		return a.runRollback(ctx, args[1:])
	}

	if len(args) > 0 && args[0] == "release" {
		return a.runRelease(ctx, args[1:])
	}

	if len(args) > 0 {
		a.logger.ErrorContext(ctx, "unknown command", "command", args[0])
		return errors.New("unknown command")
	}

	return a.runInstall(ctx)
}
