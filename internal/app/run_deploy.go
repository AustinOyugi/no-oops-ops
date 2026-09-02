package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/AustinOyugi/no-oops-ops/internal/deploy"
	"github.com/AustinOyugi/no-oops-ops/internal/doctor"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

func (a *App) runDeploy(ctx context.Context, args []string) error {
	environment, manifestPath, services, quick, err := parseDeployArgs(args, a.resolveApp)
	if err != nil {
		return err
	}

	if err := a.runDeployPreflight(ctx); err != nil {
		return err
	}
	for _, service := range services {
		if err := a.runDeployService(ctx, environment, manifest.WithService(manifestPath, service), "", quick); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) runDeployService(ctx context.Context, environment, manifestPath, optionalReleaseVersion string, quick bool) error {
	loadedManifest, err := manifest.Load(manifestPath)
	if err != nil {
		return err
	}
	if err := validateIngressTLS(loadedManifest, a.config.NginxCloudflare); err != nil {
		return err
	}
	if requiresACME(loadedManifest, a.config.NginxCloudflare) {
		if err := a.config.RequireACMEEmail(bufio.NewReader(os.Stdin), os.Stderr); err != nil {
			return err
		}
		if deployer, ok := a.deployer.(interface{ SetACMEEmail(string) }); ok {
			deployer.SetACMEEmail(a.config.ACMEEmail)
		}
	}

	options := deploy.RunOptions{Quick: quick}
	result, err := a.deployer.RunWithOptions(ctx, environment, manifestPath, optionalReleaseVersion, options)
	if err != nil {
		a.logger.ErrorContext(ctx, "deploy failed", "environment", environment, "manifest_path", manifestPath, "reason", err.Error())
		return err
	}

	if result.Verified == false && result.Executed == false {
		_, err := a.releaser.Run(ctx, environment, manifestPath)
		if err != nil {
			return err
		}

		result, err := a.deployer.RunWithOptions(ctx, environment, manifestPath, optionalReleaseVersion, options)
		if err != nil {
			a.logger.ErrorContext(ctx, "deploy failed", "environment", environment, "manifest_path", manifestPath, "reason", err.Error())
			return err
		}

		if result.Verified == false && result.Executed == false {
			return fmt.Errorf("failed to run deploy")
		}
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
		"swarm_outcome", result.SwarmOutcome,
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
		"monitor", manifest.Rollout.Monitor,
		"failure_action", manifest.Rollout.FailureAction,
		"restart_condition", manifest.Rollout.RestartCondition,
	)

	a.logger.InfoContext(
		ctx,
		"deploy inputs",
		"depends_on", manifest.DependsOn,
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
		"deploy history",
		"deployment_path", result.DeploymentPath,
	)

	a.logger.InfoContext(
		ctx,
		"deploy env artifact",
		"env_path", result.EnvPath,
	)

	return nil
}

func requiresACME(m manifest.Manifest, cloudflare bool) bool {
	return m.Expose.TLS && m.Expose.TLSCertificate == "" && !cloudflare
}

func validateIngressTLS(m manifest.Manifest, cloudflare bool) error {
	if cloudflare && m.Expose.TLS && m.Expose.TLSCertificate == "" {
		return errors.New("Cloudflare ingress requires x-noops.ingress.tls_certificate for an HTTPS route; import a Cloudflare Origin certificate with `noops certificate import`")
	}
	return nil
}

func parseDeployArgs(args []string, resolveApp func(string) (string, error)) (environment, manifestPath string, services []string, quick bool, err error) {
	if len(args) > 0 && args[0] == "--quick" {
		quick = true
		args = args[1:]
	}
	if len(args) < 2 {
		return "", "", nil, false, errors.New("deploy requires an environment and app name")
	}
	environment = args[0]
	manifestPath, err = resolveApp(args[1])
	if err != nil {
		return "", "", nil, false, err
	}
	if len(args) == 3 && args[2] == "--all" {
		names, e := manifest.DeploymentOrder(manifestPath)
		return environment, manifestPath, names, quick, e
	}
	if len(args) == 4 && args[2] == "--service" {
		return environment, manifestPath, []string{args[3]}, quick, nil
	}
	if len(args) == 2 {
		names, e := manifest.Services(manifestPath)
		if e != nil {
			return "", "", nil, false, e
		}
		if len(names) == 1 {
			return environment, manifestPath, names, quick, nil
		}
		return "", "", nil, false, errors.New("deploy requires --service <name> or --all when the manifest contains multiple services")
	}
	return "", "", nil, false, errors.New("deploy accepts only --service <name> or --all")
}

func (a *App) runDeployPreflight(ctx context.Context) error {
	result, err := a.doctor.RunProfile(ctx, doctor.ProfileDeployReadiness)
	if err != nil {
		return fmt.Errorf("run deploy preflight: %w", err)
	}

	for _, check := range result.Checks {
		switch check.Status {
		case doctor.StatusFail:
			a.logger.ErrorContext(ctx, "deploy preflight check",
				"name", check.Name,
				"status", check.Status,
				"message", check.Message,
				"remediation", check.Remediation,
			)
		case doctor.StatusSkip:
			a.logger.WarnContext(ctx, "deploy preflight check",
				"name", check.Name,
				"status", check.Status,
				"message", check.Message,
				"remediation", check.Remediation,
			)
		}
	}

	if result.Failed() {
		return fmt.Errorf("deploy preflight failed: %s", result.FirstRemediation())
	}

	return nil
}
