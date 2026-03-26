package install

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

type LocalHost struct {
	logger   *slog.Logger
	stateDir string
}

func NewLocalHost(logger *slog.Logger, stateDir string) *LocalHost {
	return &LocalHost{
		logger:   logger,
		stateDir: stateDir,
	}
}

func (h *LocalHost) VerifyDocker(ctx context.Context) error {
	h.logger.InfoContext(ctx, "checking docker installation")

	cmd := exec.CommandContext(ctx, "docker", "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("verify docker: %w: %s", err, strings.TrimSpace(string(output)))
	}

	return nil
}

const stateDirMode = 0o700

func (h *LocalHost) PrepareStateDir(ctx context.Context) error {
	h.logger.InfoContext(ctx, "preparing local state directory", "path", h.stateDir)
	err := os.MkdirAll(h.stateDir, stateDirMode)
	if err != nil {
		return fmt.Errorf("create state dir %q: %w", h.stateDir, err)
	}
	return nil
}
