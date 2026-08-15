package app

import (
	"context"
	"errors"
	"github.com/AustinOyugi/no-oops-ops/internal/doctor"
)

func (a *App) runDoctor(ctx context.Context) error {
	a.logger.InfoContext(ctx, "starting noops doctor", "app_name", a.config.AppName)

	result, err := a.doctor.Run(ctx)
	if err != nil {
		return err
	}

	for _, check := range result.Checks {
		if check.Status == doctor.StatusFail {
			a.logger.ErrorContext(
				ctx,
				"doctor check",
				"name", check.Name,
				"status", check.Status,
				"message", check.Message,
				"remediation", check.Remediation)
			continue
		}

		a.logger.InfoContext(
			ctx,
			"doctor check",
			"name",
			check.Name,
			"status",
			check.Status,
			"message",
			check.Message,
			"remediation", check.Remediation)
	}

	a.logger.InfoContext(ctx, "doctor summary",
		"passed", result.Count(doctor.StatusOK),
		"failed", result.Count(doctor.StatusFail),
		"skipped", result.Count(doctor.StatusSkip),
	)

	if result.Failed() {
		a.logger.ErrorContext(ctx, "doctor remediation", "next_step", result.FirstRemediation())
		return errors.New("doctor failed")
	}

	a.logger.InfoContext(ctx, "doctor completed", "checks", len(result.Checks), "failed", result.Failed())
	return nil
}
