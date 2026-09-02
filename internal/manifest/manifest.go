package manifest

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type Manifest struct {
	Name        string      `yaml:"name"`
	Source      Source      `yaml:"source"`
	Image       Image       `yaml:"image"`
	Service     Service     `yaml:"service"`
	Healthcheck Healthcheck `yaml:"healthcheck"`
	Rollout     Rollout     `yaml:"rollout"`
	Expose      Expose      `yaml:"expose"`
	Env         Env         `yaml:"env"`
	Build       NoOpsBuild  `yaml:"build"`
	DependsOn   []string    `yaml:"depends_on"`
	Volumes     []string    `yaml:"volumes"`
	// Compose is the selected, Compose-shaped document. It is deliberately a
	// yaml.Node rather than a Go struct: Compose adds fields over time and No
	// Oops must not discard fields it does not own.
	Compose *yaml.Node `yaml:"-"`
	Path    string     `yaml:"-"`
}

// ComposeFile is the initial Compose-shaped input supported by No Oops Ops.
// Until service selection is introduced, it must contain exactly one service.
type ComposeFile struct {
	Services map[string]ComposeService `yaml:"services"`
}

type ComposeService struct {
	Image       string        `yaml:"image"`
	Build       ComposeBuild  `yaml:"build"`
	Command     []string      `yaml:"command"`
	Healthcheck Healthcheck   `yaml:"healthcheck"`
	Deploy      ComposeDeploy `yaml:"deploy"`
	Networks    []string      `yaml:"networks"`
	Volumes     []string      `yaml:"volumes"`
	NoOps       ComposeNoOps  `yaml:"x-noops"`
}

type ComposeBuild struct {
	Context    string `yaml:"context"`
	Dockerfile string `yaml:"dockerfile"`
}

func (b *ComposeBuild) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		b.Context = value.Value
		return nil
	}
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("build must be a path or mapping")
	}
	type composeBuild ComposeBuild
	return value.Decode((*composeBuild)(b))
}

type ComposeDeploy struct {
	Replicas int `yaml:"replicas"`
}

type ComposeNoOps struct {
	Env   Env        `yaml:"env"`
	Build NoOpsBuild `yaml:"build"`
	// Ingress is the Compose-preserving public-routing metadata. Expose remains
	// accepted as a backwards-compatible alias for existing manifests.
	Ingress   Expose   `yaml:"ingress"`
	Expose    Expose   `yaml:"expose"`
	Rollout   Rollout  `yaml:"rollout"`
	DependsOn []string `yaml:"depends_on"`
	Source    Source   `yaml:"source"`
	Service   Service  `yaml:"service"`
}

// NoOpsBuild describes how No Oops obtains an isolated build context. Compose
// retains ownership of build.context and build.dockerfile; these settings are
// specific to the No Oops release workflow.
type NoOpsBuild struct {
	Source    BuildSource    `yaml:"source"`
	Resources BuildResources `yaml:"resources"`
	Timeout   string         `yaml:"timeout"`
	NoCache   bool           `yaml:"no-cache"`
}

type BuildSource struct {
	Git *GitSource `yaml:"git"`
}

type GitSource struct {
	URL          string                    `yaml:"url"`
	Environments map[string]GitEnvironment `yaml:"environments"`
}

type GitEnvironment struct {
	Ref    string `yaml:"ref"`
	Secret string `yaml:"secret"`
}

type BuildResources struct {
	CPUs   string `yaml:"cpus"`
	Memory string `yaml:"memory"`
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
	Repository      string `yaml:"repository"`
	Tag             string `yaml:"tag"`
	SourceReference string `yaml:"-"` // original Compose reference, including a digest when supplied
	Build           *bool  `yaml:"build"`
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
	Entrypoint   []string `yaml:"entrypoint"`
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
	Domain         string   `yaml:"domain"`
	Domains        []string `yaml:"domains"`
	PathPrefix     string   `yaml:"path_prefix"`
	Enabled        bool     `yaml:"enabled"`
	BlueGreen      *bool    `yaml:"blue_green"`
	TLS            bool     `yaml:"tls"`
	TLSCertificate string   `yaml:"tls_certificate"`
	Proxy          Proxy    `yaml:"proxy"`
}

type Proxy struct {
	Websocket         bool   `yaml:"websocket"`
	ClientMaxBodySize string `yaml:"client_max_body_size"`
}

// BlueGreenEnabled reports whether exposed releases should use blue/green
// promotion. It defaults to true; an explicit false opts into in-place Swarm
// updates instead.
func (e Expose) BlueGreenEnabled() bool {
	return e.BlueGreen == nil || *e.BlueGreen
}

type Env struct {
	File    string      `yaml:"file"`
	Build   *EnvBuild   `yaml:"build"`
	Secrets *EnvSecrets `yaml:"secrets"`
}

// EnvBuild declares an ephemeral dotenv file that No Oops materializes in the
// isolated build context. It is useful for tools such as Next.js that already
// discover dotenv files without any Dockerfile-specific integration.
type EnvBuild struct {
	File    string   `yaml:"file"`
	Secrets []string `yaml:"secrets"`
}

type EnvSecrets struct {
	Resolution string   `yaml:"resolution"`
	Resolvable []string `yaml:"resolvable"`
}
