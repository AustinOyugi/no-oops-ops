package app

import (
	"errors"
	"fmt"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

// Target identifies an app and the services a lifecycle command should affect.
// CLI adapters construct it from parsed flags; application code never parses argv.
type Target struct {
	Environment string
	App         string
	All         bool
	Service     string
}

func (a *App) resolveTarget(target Target, allowImplicitSingleService bool) (string, string, []string, error) {
	if target.Environment == "" || target.App == "" {
		return "", "", nil, errors.New("an environment and app name are required")
	}
	if target.All && target.Service != "" {
		return "", "", nil, errors.New("provide exactly one of --service or --all")
	}
	path, err := a.resolveApp(target.App)
	if err != nil {
		return "", "", nil, err
	}
	if target.All {
		names, err := manifest.DeploymentOrder(path)
		return target.Environment, path, names, err
	}
	if target.Service != "" {
		if _, err := manifest.LoadService(path, target.Service); err != nil {
			return "", "", nil, err
		}
		return target.Environment, path, []string{target.Service}, nil
	}
	if allowImplicitSingleService {
		names, err := manifest.Services(path)
		if err != nil {
			return "", "", nil, err
		}
		if len(names) == 1 {
			return target.Environment, path, names, nil
		}
	}
	return "", "", nil, fmt.Errorf("select a service with --service <name> or --all")
}
