package release

import "github.com/AustinOyugi/no-oops-ops/internal/manifest"

type Result struct {
	Environment   string
	ManifestPath  string
	Image         string
	RegistryImage string
	Built         bool
	Pushed        bool
	Manifest      manifest.Manifest
}
