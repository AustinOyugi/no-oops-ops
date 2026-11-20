package doctor

import (
	"context"
	"fmt"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"github.com/AustinOyugi/no-oops-ops/internal/storage"
)

func (s *Service) checkStorage(ctx context.Context) Check {
	usage, err := storage.Inspect(ctx, command.NewRunner(s.logger))
	if err != nil {
		return Check{Name: "docker_storage", Status: StatusFail, Message: err.Error(), Remediation: "Run noops on the Docker host and verify its storage directory"}
	}
	status := StatusOK
	if usage.Free < s.config.Storage.WarnFreeBytes {
		status = StatusFail
	}
	return Check{Name: "docker_storage", Status: status, Message: fmt.Sprintf("%s: %d free bytes (warning threshold %d)", usage.Path, usage.Free, s.config.Storage.WarnFreeBytes), Remediation: "Run noops cleanup --host to review reclaimable storage; expand storage if necessary"}
}
