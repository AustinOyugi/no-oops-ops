package release

type Metadata struct {
	Environment   string `json:"environment"`
	Image         string `json:"image"`
	RegistryImage string `json:"registry_image"`
	Tag           string `json:"tag"`
}
