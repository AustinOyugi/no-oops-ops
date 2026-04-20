package release

import "github.com/AustinOyugi/no-oops-ops/internal/manifest"

type Result struct {
	Environment  string
	ManifestPath string
	Image        string
	Built        bool
	Manifest     manifest.Manifest
}
