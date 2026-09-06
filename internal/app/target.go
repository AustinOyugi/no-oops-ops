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

func (a *App) resolveTarget(target Target, allowImplicitSingleService, deployableOnly bool) (string, string, []string, error) {
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
		var names []string
		if deployableOnly {
			names, err = manifest.DeploymentOrder(path)
		} else {
			names, err = manifest.ReleaseOrder(path)
		}
		return target.Environment, path, names, err
	}
	if target.Service != "" {
		m, err := manifest.LoadService(path, target.Service)
		if err != nil {
			return "", "", nil, err
		}
		if deployableOnly && !m.ShouldDeploy() {
			return "", "", nil, fmt.Errorf("service %q has x-noops.deploy disabled", target.Service)
		}
		return target.Environment, path, []string{target.Service}, nil
	}
	if allowImplicitSingleService {
		names, err := manifest.Services(path)
		if err != nil {
			return "", "", nil, err
		}
		if len(names) == 1 {
			if deployableOnly {
				m, err := manifest.LoadService(path, names[0])
				if err != nil {
					return "", "", nil, err
				}
				if !m.ShouldDeploy() {
					return "", "", nil, fmt.Errorf("service %q has x-noops.deploy disabled", names[0])
				}
			}
			return target.Environment, path, names, nil
		}
	}
	return "", "", nil, fmt.Errorf("select a service with --service <name> or --all")
}
