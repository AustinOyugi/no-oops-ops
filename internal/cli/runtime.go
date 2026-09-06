package cli

import (
	"fmt"
	"os"

	"github.com/AustinOyugi/no-oops-ops/internal/app"
	"github.com/AustinOyugi/no-oops-ops/internal/config"
)

type runtime struct{ workspace string }

func (r runtime) application() (*app.App, error) {
	root := r.workspace
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}
	cfg, err := config.Load(root)
	if err != nil {
		return nil, err
	}
	return app.New(cfg)
}

type appRunner func(func(*app.App) error) error
