package cli

import (
	"fmt"
	"os"

	"github.com/AustinOyugi/no-oops-ops/internal/app"
	"github.com/AustinOyugi/no-oops-ops/internal/config"
)

type runtime struct {
	workspace           string
	stateDir            string
	environment         string
	selectedEnvironment string
}

func (r runtime) options() config.Options {
	return config.Options{Environment: r.selectedEnvironment, StateDir: r.stateDir}
}

func (r runtime) configuration() (config.Config, error) {
	root, err := r.workspaceRoot()
	if err != nil {
		return config.Config{}, err
	}
	return config.LoadWithOptions(root, r.options())
}

func (r runtime) workspaceRoot() (string, error) {
	root := r.workspace
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
	}
	return root, nil
}

func (r runtime) application() (*app.App, error) {
	cfg, err := r.configuration()
	if err != nil {
		return nil, err
	}
	return app.New(cfg)
}
