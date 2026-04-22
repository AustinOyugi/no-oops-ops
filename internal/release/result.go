package release

import "github.com/AustinOyugi/no-oops-ops/internal/manifest"

type Result struct {
	Environment   string
	MetadataPath  string
	ManifestPath  string
	Image         string
	RegistryImage string
	Tag           string
	Built         bool
	Pushed        bool
	Manifest      manifest.Manifest
}
