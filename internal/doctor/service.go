package doctor

import (
	"context"
	"fmt"
	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"log/slog"
)

type Host interface {
	VerifyDocker(ctx context.Context) error
	InspectSwarmState(ctx context.Context) (string, error)
	InspectSharedNetwork(ctx context.Context) error
	InspectRegistryService(ctx context.Context) error
}

type Service struct {
	logger *slog.Logger
	config config.Config
	host   Host
}

func NewService(logger *slog.Logger, cfg config.Config, host Host) *Service {
	return &Service{
		logger: logger,
		config: cfg,
		host:   host,
	}
}

func (s *Service) Run(ctx context.Context) (Result, error) {
	s.logger.InfoContext(ctx, "starting doctor")

	result := Result{}
	statuses := make(map[string]Status)

	for _, definition := range s.checks() {
		skipped := false
		for _, prerequisite := range definition.requires {
			status, ok := statuses[prerequisite]
			if !ok {
				return Result{}, fmt.Errorf("doctor check %q has unknown prerequisite %q", definition.name, prerequisite)
			}
			if status != StatusOK {
				result.Add(definition.name, StatusSkip, definition.skipMessage, definition.remediation)
				statuses[definition.name] = StatusSkip
				skipped = true
				break
			}
		}
		if skipped {
			continue
		}

		check := definition.run(ctx)
		result.Add(check.Name, check.Status, check.Message, check.Remediation)
		statuses[definition.name] = check.Status
	}

	return result, nil
}
