package release

import "time"

type Metadata struct {
	App           string    `json:"app"`
	CreateAt      time.Time `json:"create_at"`
	Environment   string    `json:"environment"`
	Image         string    `json:"image"`
	RegistryImage string    `json:"registry_image"`
	Tag           string    `json:"tag"`
}

type ActiveRelease struct {
	Tag string `json:"tag"`
}
