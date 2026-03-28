package manifest

type Manifest struct {
	Name        string      `yaml:"name"`
	Image       Image       `yaml:"image"`
	Service     Service     `yaml:"service"`
	Healthcheck Healthcheck `yaml:"healthcheck"`
	Rollout     Rollout     `yaml:"rollout"`
	Expose      Expose      `yaml:"expose"`
	Env         Env         `yaml:"env"`
	DependsOn   []string    `yaml:"depends_on"`
	Secrets     []string    `yaml:"secrets"`
	Volumes     []string    `yaml:"volumes"`
}

type Image struct {
	Repository string `yaml:"repository"`
	Tag        string `yaml:"tag"`
}

type Service struct {
	InternalPort int    `yaml:"internal_port"`
	Replicas     int    `yaml:"replicas"`
	Network      string `yaml:"network"`
}

type Healthcheck struct {
	Type        string `yaml:"type"`
	Path        string `yaml:"path"`
	Port        int    `yaml:"port"`
	Interval    string `yaml:"interval"`
	Timeout     string `yaml:"timeout"`
	Retries     int    `yaml:"retries"`
	StartPeriod string `yaml:"start_period"`
}

type Rollout struct {
	Order              string `yaml:"order"`
	Parallelism        int    `yaml:"parallelism"`
	Delay              string `yaml:"delay"`
	FailureAction      string `yaml:"failure_action"`
	RestartCondition   string `yaml:"restart_condition"`
	RestartDelay       string `yaml:"restart_delay"`
	RestartMaxAttempts int    `yaml:"restart_max_attempts"`
	RestartWindow      string `yaml:"restart_window"`
}

type Expose struct {
	Domain     string `yaml:"domain"`
	PathPrefix string `yaml:"path_prefix"`
	Enabled    bool   `yaml:"enabled"`
}

type Env struct {
	Shared       map[string]string            `yaml:"shared"`
	Environments map[string]map[string]string `yaml:"environments"`
}
