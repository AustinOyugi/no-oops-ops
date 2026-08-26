package manifest

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	domainPattern     = regexp.MustCompile(`(?i)^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)*$`)
	pathPrefixPattern = regexp.MustCompile(`^/[A-Za-z0-9._~%/@:+-]*$`)
)

func (m Manifest) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("manifest name is required")
	}

	if m.Image.Repository == "" {
		return fmt.Errorf("image.repository is required")
	}

	if m.Expose.Enabled && m.Service.InternalPort == 0 {
		return fmt.Errorf("service.internal_port is required when ingress is enabled")
	}

	for name, value := range map[string]string{
		"healthcheck.interval":        m.Healthcheck.Interval,
		"healthcheck.timeout":         m.Healthcheck.Timeout,
		"healthcheck.start_period":    m.Healthcheck.StartPeriod,
		"rollout.delay":               m.Rollout.Delay,
		"rollout.monitor":             m.Rollout.Monitor,
		"rollout.convergence_timeout": m.Rollout.ConvergenceTimeout,
		"rollout.rollback.delay":      m.Rollout.Rollback.Delay,
		"rollout.rollback.monitor":    m.Rollout.Rollback.Monitor,
	} {
		if value == "" {
			continue
		}
		if _, err := time.ParseDuration(value); err != nil {
			return fmt.Errorf("%s must be a Go duration: %w", name, err)
		}
	}

	if m.Rollout.MaxFailureRatio < 0 || m.Rollout.MaxFailureRatio > 1 {
		return fmt.Errorf("rollout.max_failure_ratio must be between 0 and 1")
	}
	if m.Rollout.Rollback.MaxFailureRatio < 0 || m.Rollout.Rollback.MaxFailureRatio > 1 {
		return fmt.Errorf("rollout.rollback.max_failure_ratio must be between 0 and 1")
	}

	if m.Image.ShouldBuild() {
		if m.Source.Context == "" {
			return fmt.Errorf("source.context is required when image.build is true")
		}

		if m.Source.Dockerfile == "" {
			return fmt.Errorf("source.dockerfile is required when image.build is true")
		}
	}
	if err := m.Build.Validate(m.Image.ShouldBuild()); err != nil {
		return err
	}
	if len(m.Source.Build.Command) > 0 {
		return fmt.Errorf("x-noops.source.build.command is unsupported; move build steps into the Dockerfile")
	}

	if m.Env.Secrets != nil {
		if err := m.Env.Secrets.Validate(); err != nil {
			return err
		}
	}

	if m.Expose.Enabled {
		if m.Expose.Domain == "" {
			return fmt.Errorf("expose.domain is required when expose.enabled is true")
		}
		if !domainPattern.MatchString(m.Expose.Domain) {
			return fmt.Errorf("expose.domain must be a valid hostname")
		}
		if !pathPrefixPattern.MatchString(m.Expose.PathPrefix) {
			return fmt.Errorf("expose.path_prefix must be an absolute HTTP path without query or fragment")
		}
	}
	if m.Expose.BlueGreen != nil && *m.Expose.BlueGreen && !m.Expose.Enabled {
		return fmt.Errorf("expose.blue_green requires expose.enabled")
	}
	if err := m.validateComposeCompatibility(); err != nil {
		return err
	}

	return nil
}

func (b NoOpsBuild) Validate(imageBuild bool) error {
	if b.Timeout != "" {
		if _, err := time.ParseDuration(b.Timeout); err != nil {
			return fmt.Errorf("x-noops.build.timeout must be a Go duration: %w", err)
		}
	}
	if b.Resources.CPUs != "" {
		cpus, err := strconv.ParseFloat(b.Resources.CPUs, 64)
		if err != nil || cpus <= 0 {
			return fmt.Errorf("x-noops.build.resources.cpus must be a positive number")
		}
	}
	if b.Resources.Memory != "" && !regexp.MustCompile(`^[1-9][0-9]*[kKmMgGtT]i?[bB]?$`).MatchString(b.Resources.Memory) {
		return fmt.Errorf("x-noops.build.resources.memory must be a positive byte value such as 2Gi")
	}
	if b.Source.Git == nil {
		return nil
	}
	if !imageBuild {
		return fmt.Errorf("x-noops.build.source.git requires a Compose build")
	}
	git := b.Source.Git
	u, err := url.Parse(git.URL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return fmt.Errorf("x-noops.build.source.git.url must be an HTTPS repository URL without embedded credentials")
	}
	if len(git.Environments) == 0 {
		return fmt.Errorf("x-noops.build.source.git.environments is required")
	}
	for environment, settings := range git.Environments {
		if strings.TrimSpace(environment) == "" || strings.TrimSpace(settings.Ref) == "" {
			return fmt.Errorf("x-noops.build.source.git.environments entries require an environment and ref")
		}
	}
	return nil
}

func (m Manifest) validateComposeCompatibility() error {
	if m.Compose == nil {
		return nil
	}
	root := m.Compose
	if root.Kind == 1 && len(root.Content) > 0 {
		root = root.Content[0]
	}
	services := mapValue(root, "services")
	if services == nil || len(services.Content) != 2 {
		return nil
	}
	service := services.Content[1]
	if mapValue(service, "container_name") != nil {
		return fmt.Errorf("service %q: container_name is incompatible with Docker Swarm scheduling; remove it", m.Name)
	}
	if m.Expose.Enabled && mapValue(service, "ports") != nil {
		return fmt.Errorf("service %q: ports conflicts with No Oops-managed ingress; remove public HTTP ports", m.Name)
	}
	if env := mapValue(service, "environment"); env != nil && env.Kind == 4 {
		for i := 0; i+1 < len(env.Content); i += 2 {
			key, value := env.Content[i].Value, env.Content[i+1].Value
			lower := strings.ToLower(key)
			if (strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "token") || strings.Contains(lower, "api_key") || strings.Contains(lower, "private_key")) && value != "" && !strings.Contains(value, "${") {
				return fmt.Errorf("service %q: plaintext credential %q must move to a managed secret", m.Name, key)
			}
		}
	}
	return nil
}

func (s *EnvSecrets) Validate() error {
	switch s.Resolution {
	case "env", "file":
	default:
		return fmt.Errorf("env.secrets.resolution must be \"env\" or \"file\", got %q", s.Resolution)
	}
	return nil
}
