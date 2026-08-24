package app

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func TestRunInstallRejectsUnexpectedArguments(t *testing.T) {
	application := &App{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	err := application.Run(context.Background(), []string{"install", "."})
	if err == nil || !strings.Contains(err.Error(), "does not accept arguments") {
		t.Fatalf("install argument error = %v", err)
	}
}
