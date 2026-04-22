package deploy

import "github.com/AustinOyugi/no-oops-ops/internal/manifest"

type Result struct {
	Environment  string
	ServiceName  string
	StackName    string
	Executed     bool
	Verified     bool
	RunningTasks int
	ReleaseImage string
	ReleaseTag   string
	ManifestPath string
	EnvFilePath  string
	StackPath    string
	EnvPath      string
	Manifest     manifest.Manifest
}
