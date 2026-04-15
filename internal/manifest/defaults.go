package manifest

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
	defaultExposePathPrefix       = "/"
)

func (m *Manifest) applyDefaults() {
	if m.Image.Tag == "" {
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

	if m.Expose.PathPrefix == "" {
		m.Expose.PathPrefix = defaultExposePathPrefix
	}

	if m.DependsOn == nil {
		m.DependsOn = []string{}
	}

	if m.Secrets == nil {
		m.Secrets = []string{}
	}

	if m.Volumes == nil {
		m.Volumes = []string{}
	}
}
