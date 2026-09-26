package cli

import (
	"fmt"
	"os"

	"github.com/AustinOyugi/no-oops-ops/internal/app"
	"github.com/AustinOyugi/no-oops-ops/internal/config"
)

type runtime struct{ workspace string }

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
	root, err := r.workspaceRoot()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return nil, err
	}
	return app.New(cfg)
}
