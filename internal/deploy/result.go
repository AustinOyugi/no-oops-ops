package deploy

import "github.com/AustinOyugi/no-oops-ops/internal/manifest"

type Result struct {
	ManifestPath string
	StackPath    string
	Manifest     manifest.Manifest
}
