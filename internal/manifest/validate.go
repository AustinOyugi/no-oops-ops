package manifest

import "fmt"

func (m Manifest) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("manifest name is required")
	}

	if m.Image.Repository == "" {
		return fmt.Errorf("image.repository is required")
	}

	if m.Service.InternalPort == 0 {
		return fmt.Errorf("service.internal_port is required")
	}

	if len(m.Healthcheck.Test) == 0 {
		return fmt.Errorf("healthcheck.test is required")
	}

	return nil
}
