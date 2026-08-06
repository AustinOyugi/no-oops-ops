package app

import (
	"context"
	"errors"
	"fmt"
)

func (a *App) runDeploy(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return errors.New("deploy requires an environment and manifest path")
	}

	environment := args[0]
	manifestPath := args[1]

	result, err := a.deployer.Run(ctx, environment, manifestPath)
	if err != nil {
		a.logger.ErrorContext(ctx, "deploy failed", "environment", environment, "manifest_path", manifestPath, "reason", err.Error())
		return err
	}

	manifest := result.Manifest

	a.logger.InfoContext(
		ctx,
		"deploy manifest",
		"path", result.ManifestPath,
		"stack_name", result.StackName,
		"executed", result.Executed,
		"verified", result.Verified,
		"running_tasks", result.RunningTasks,
		"readiness_checked", true,
		"environment", result.Environment,
		"name", manifest.Name,
		"service_name", result.ServiceName,
		"source_context", manifest.Source.Context,
		"source_dockerfile", manifest.Source.Dockerfile,
		"image", fmt.Sprintf("%s:%s", manifest.Image.Repository, manifest.Image.Tag),
		"release_image", result.ReleaseImage,
		"release_tag", result.ReleaseTag,
		"internal_port", manifest.Service.InternalPort,
		"replicas", manifest.Service.Replicas,
		"network", manifest.Service.Network,
	)

	a.logger.InfoContext(
		ctx,
		"deploy healthcheck",
		"test", manifest.Healthcheck.Test,
		"interval", manifest.Healthcheck.Interval,
		"timeout", manifest.Healthcheck.Timeout,
		"retries", manifest.Healthcheck.Retries,
		"start_period", manifest.Healthcheck.StartPeriod,
	)

	a.logger.InfoContext(
		ctx,
		"deploy rollout",
		"order", manifest.Rollout.Order,
		"parallelism", manifest.Rollout.Parallelism,
		"delay", manifest.Rollout.Delay,
		"failure_action", manifest.Rollout.FailureAction,
		"restart_condition", manifest.Rollout.RestartCondition,
	)

	a.logger.InfoContext(
		ctx,
		"deploy inputs",
		"depends_on", manifest.DependsOn,
		"secrets", manifest.Secrets,
		"volumes", manifest.Volumes,
	)

	a.logger.InfoContext(
		ctx,
		"deploy env source",
		"env_file_path", result.EnvFilePath,
	)

	a.logger.InfoContext(
		ctx,
		"deploy artifact",
		"stack_path", result.StackPath,
	)

	a.logger.InfoContext(
		ctx,
		"deploy env artifact",
		"env_path", result.EnvPath,
	)

	return nil
}
