package app

import (
	"context"
	"fmt"
)

type App struct{}

func New() *App {
	return &App{}
}

func (app *App) Run(ctx context.Context) error {
	_ = ctx
	fmt.Println("noops")
	return nil
}
