package app

import (
	"context"
	"github.com/AustinOyugi/no-oops-ops/internal/cleanup"
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
	cleaner     *cleanup.Service
}

type deployer interface {
	Run(ctx context.Context, environment string, path string, optionalReleaseVersion string) (deploy.Result, error)
	RunWithOptions(ctx context.Context, environment string, path string, optionalReleaseVersion string, options deploy.RunOptions) (deploy.Result, error)
	Rollback(ctx context.Context, environment string, path string) (deploy.Result, error)
	Remove(ctx context.Context, environment string, path string) (deploy.RemoveResult, error)
}

type doctorRunner interface {
	RunProfile(ctx context.Context, profile doctor.Profile) (doctor.Result, error)
}

type secretRunner interface {
	Set(context.Context, string, string, io.Reader) (secret.Metadata, error)
	Delete(context.Context, string, string) ([]secret.Metadata, error)
	List(context.Context, string) ([]secret.Metadata, error)
}

func New(cfg config.Config) (*App, error) {

	logger := logging.New()

	localHost := local.NewHost(
		logger, cfg.StateDir, cfg.DataDir, cfg.InstallVersion,
		cfg.NetworkName, cfg.RegistryName, cfg.RegistryPort,
		cfg.NginxName, cfg.NginxHTTPPort, cfg.NginxHTTPSPort)

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
		cleaner:     cleanup.NewService(logger, cfg),
	}, nil
}
