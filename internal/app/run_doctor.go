package app

import (
	"context"
	"errors"
	"github.com/AustinOyugi/no-oops-ops/internal/doctor"
)

func (a *App) runDoctor(ctx context.Context, args []string) error {
	if len(args) > 1 || (len(args) == 1 && args[0] != "--deploy-ready") {
		return errors.New("doctor supports only the --deploy-ready option")
	}
	return a.Doctor(ctx, len(args) == 1)
}

// Doctor checks the local deployment platform.
func (a *App) Doctor(ctx context.Context, deployReady bool) error {
	profile := doctor.ProfileFull
	if deployReady {
		profile = doctor.ProfileDeployReadiness
	}

	a.logger.InfoContext(ctx, "starting noops doctor", "app_name", a.config.AppName, "profile", profile)

	result, err := a.doctor.RunProfile(ctx, profile)
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
		"profile", profile,
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
