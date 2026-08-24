package manifest

import (
	"strings"
	"time"
)

const (
	defaultImageTag               = "latest"
	defaultServiceReplicas        = 1
	defaultServiceNetwork         = "noops-net"
	defaultHealthcheckInterval    = "10s"
	defaultHealthcheckTimeout     = "10s"
	defaultHealthcheckRetries     = 3
	defaultHealthcheckStartPeriod = "60s"
	defaultRolloutOrder           = "start-first"
	defaultRolloutParallelism     = 1
	defaultRolloutDelay           = "10s"
	defaultRolloutFailureAction   = "rollback"
	defaultRestartCondition       = "on-failure"
	defaultRestartDelay           = "10s"
	defaultRestartMaxAttempts     = 5
	defaultRestartWindow          = "70s"
	defaultConvergenceTimeout     = "5m"
	defaultRollbackOrder          = "start-first"
	defaultRollbackParallelism    = 1
	defaultRollbackDelay          = "0s"
	defaultRollbackFailureAction  = "pause"
	defaultMonitorSafetyBuffer    = 10 * time.Second
	defaultExposePathPrefix       = "/"
)

func (m *Manifest) applyDefaults() {
	if m.Image.Tag == "" && !strings.Contains(m.Image.SourceReference, "@") {
		m.Image.Tag = defaultImageTag
	}

	if m.Service.Replicas == 0 {
		m.Service.Replicas = defaultServiceReplicas
	}

	if m.Service.Network == "" {
		m.Service.Network = defaultServiceNetwork
	}

	if m.Healthcheck.Interval == "" {
		m.Healthcheck.Interval = defaultHealthcheckInterval
	}

	if m.Healthcheck.Timeout == "" {
		m.Healthcheck.Timeout = defaultHealthcheckTimeout
	}

	if m.Healthcheck.Retries == 0 {
		m.Healthcheck.Retries = defaultHealthcheckRetries
	}

	if m.Healthcheck.StartPeriod == "" {
		m.Healthcheck.StartPeriod = defaultHealthcheckStartPeriod
	}

	if m.Rollout.Order == "" {
		m.Rollout.Order = defaultRolloutOrder
	}

	if m.Rollout.Parallelism == 0 {
		m.Rollout.Parallelism = defaultRolloutParallelism
	}

	if m.Rollout.Delay == "" {
		m.Rollout.Delay = defaultRolloutDelay
	}

	if m.Rollout.Monitor == "" {
		m.Rollout.Monitor = rolloutMonitor(m.Healthcheck)
	}

	if m.Rollout.FailureAction == "" {
		m.Rollout.FailureAction = defaultRolloutFailureAction
	}

	if m.Rollout.RestartCondition == "" {
		m.Rollout.RestartCondition = defaultRestartCondition
	}

	if m.Rollout.RestartDelay == "" {
		m.Rollout.RestartDelay = defaultRestartDelay
	}

	if m.Rollout.RestartMaxAttempts == 0 {
		m.Rollout.RestartMaxAttempts = defaultRestartMaxAttempts
	}

	if m.Rollout.RestartWindow == "" {
		m.Rollout.RestartWindow = defaultRestartWindow
	}

	if m.Rollout.ConvergenceTimeout == "" {
		m.Rollout.ConvergenceTimeout = defaultConvergenceTimeout
	}

	if m.Rollout.Rollback.Order == "" {
		m.Rollout.Rollback.Order = defaultRollbackOrder
	}

	if m.Rollout.Rollback.Parallelism == 0 {
		m.Rollout.Rollback.Parallelism = defaultRollbackParallelism
	}

	if m.Rollout.Rollback.Delay == "" {
		m.Rollout.Rollback.Delay = defaultRollbackDelay
	}

	if m.Rollout.Rollback.Monitor == "" {
		m.Rollout.Rollback.Monitor = m.Rollout.Monitor
	}

	if m.Rollout.Rollback.FailureAction == "" {
		m.Rollout.Rollback.FailureAction = defaultRollbackFailureAction
	}

	if m.Expose.PathPrefix == "" {
		m.Expose.PathPrefix = defaultExposePathPrefix
	}

	if m.DependsOn == nil {
		m.DependsOn = []string{}
	}

	if m.Volumes == nil {
		m.Volumes = []string{}
	}
}

func rolloutMonitor(healthcheck Healthcheck) string {
	startPeriod, startErr := time.ParseDuration(healthcheck.StartPeriod)
	interval, intervalErr := time.ParseDuration(healthcheck.Interval)
	timeout, timeoutErr := time.ParseDuration(healthcheck.Timeout)
	if startErr != nil || intervalErr != nil || timeoutErr != nil {
		return "2m"
	}

	monitor := startPeriod + time.Duration(healthcheck.Retries)*interval + timeout + defaultMonitorSafetyBuffer
	return monitor.String()
}
