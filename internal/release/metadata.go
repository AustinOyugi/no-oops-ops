package release

import (
	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"strings"
	"time"
)

type Metadata struct {
	App           string       `json:"app"`
	Build         bool         `json:"build"`
	CreateAt      time.Time    `json:"create_at"`
	Environment   string       `json:"environment"`
	Image         string       `json:"image"`
	RegistryImage string       `json:"registry_image"`
	Digest        string       `json:"digest,omitempty"`
	Git           *GitMetadata `json:"git,omitempty"`
	SourceTag     string       `json:"source_tag,omitempty"`
	Tag           string       `json:"tag"`
}

// ImmutableImage is the registry digest reference recorded immediately after
// push. Older history without a digest remains deployable for compatibility.
func (m Metadata) ImmutableImage() string {
	if m.Digest == "" {
		return m.RegistryImage
	}
	if _, digest, found := strings.Cut(m.Digest, "@"); found {
		return m.RegistryImage + "@" + digest
	}
	return m.Digest
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
