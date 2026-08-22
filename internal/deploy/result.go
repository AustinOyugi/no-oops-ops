package deploy

import "github.com/AustinOyugi/no-oops-ops/internal/manifest"

type Result struct {
	DeploymentPath string
	Environment    string
	ServiceName    string
	StackName      string
	Executed       bool
	Verified       bool
	RunningTasks   int
	SwarmOutcome   SwarmOutcome
	ReleaseImage   string
	ReleaseTag     string
	ManifestPath   string
	EnvFilePath    string
	StackPath      string
	EnvPath        string
	Manifest       manifest.Manifest
}
