package release

import (
	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"time"
)

type Metadata struct {
	App           string       `json:"app"`
	Build         bool         `json:"build"`
	CreateAt      time.Time    `json:"create_at"`
	Environment   string       `json:"environment"`
	Image         string       `json:"image"`
	RegistryImage string       `json:"registry_image"`
	Git           *GitMetadata `json:"git,omitempty"`
	SourceTag     string       `json:"source_tag,omitempty"`
	Tag           string       `json:"tag"`
}

type ActiveRelease struct {
	Tag         string `json:"tag"`
	IsAvailable bool   `json:"is_available"`
}

type Store interface {
	Find(cfg config.Config, name string, environment string, tag string) (Metadata, error)
	Latest(cfg config.Config, name string, environment string) (ActiveRelease, error)
	SetLatest(cfg config.Config, appName string, metadata ActiveRelease, environment string) error
}
