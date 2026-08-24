package manifest

type Manifest struct {
	Name        string      `yaml:"name"`
	Source      Source      `yaml:"source"`
	Image       Image       `yaml:"image"`
	Service     Service     `yaml:"service"`
	Healthcheck Healthcheck `yaml:"healthcheck"`
	Rollout     Rollout     `yaml:"rollout"`
	Expose      Expose      `yaml:"expose"`
	Env         Env         `yaml:"env"`
	DependsOn   []string    `yaml:"depends_on"`
	Volumes     []string    `yaml:"volumes"`
}

type Source struct {
	Context    string      `yaml:"context"`
	Dockerfile string      `yaml:"dockerfile"`
	Build      SourceBuild `yaml:"build"`
}

type SourceBuild struct {
	Command []string `yaml:"command"`
}

type Image struct {
	Repository string `yaml:"repository"`
	Tag        string `yaml:"tag"`
	Build      *bool  `yaml:"build"`
}

func (i Image) ShouldBuild() bool {
	return i.Build == nil || *i.Build
}

type Service struct {
	InternalPort int      `yaml:"internal_port"`
	ExternalPort int      `yaml:"external_port"`
	Replicas     int      `yaml:"replicas"`
	Network      string   `yaml:"network"`
	Command      []string `yaml:"command"`
}

type Healthcheck struct {
	Test        []string `yaml:"test"`
	Interval    string   `yaml:"interval"`
	Timeout     string   `yaml:"timeout"`
	Retries     int      `yaml:"retries"`
	StartPeriod string   `yaml:"start_period"`
}

type Rollout struct {
	Order              string   `yaml:"order"`
	Parallelism        int      `yaml:"parallelism"`
	Delay              string   `yaml:"delay"`
	Monitor            string   `yaml:"monitor"`
	MaxFailureRatio    float64  `yaml:"max_failure_ratio"`
	FailureAction      string   `yaml:"failure_action"`
	RestartCondition   string   `yaml:"restart_condition"`
	RestartDelay       string   `yaml:"restart_delay"`
	RestartMaxAttempts int      `yaml:"restart_max_attempts"`
	RestartWindow      string   `yaml:"restart_window"`
	ConvergenceTimeout string   `yaml:"convergence_timeout"`
	Rollback           Rollback `yaml:"rollback"`
}

type Rollback struct {
	Order           string  `yaml:"order"`
	Parallelism     int     `yaml:"parallelism"`
	Delay           string  `yaml:"delay"`
	Monitor         string  `yaml:"monitor"`
	MaxFailureRatio float64 `yaml:"max_failure_ratio"`
	FailureAction   string  `yaml:"failure_action"`
}

type Expose struct {
	Domain     string `yaml:"domain"`
	PathPrefix string `yaml:"path_prefix"`
	Enabled    bool   `yaml:"enabled"`
	TLS        bool   `yaml:"tls"`
}

type Env struct {
	File    string      `yaml:"file"`
	Secrets *EnvSecrets `yaml:"secrets"`
}

type EnvSecrets struct {
	Resolution string   `yaml:"resolution"`
	Resolvable []string `yaml:"resolvable"`
}
