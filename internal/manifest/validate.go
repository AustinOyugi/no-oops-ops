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

	if m.Source.Context == "" {
		return fmt.Errorf("source.context is required")
	}

	if m.Source.Dockerfile == "" {
		return fmt.Errorf("source.dockerfile is required")
	}

	if m.Env.Secrets != nil {
		if err := m.Env.Secrets.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (s *EnvSecrets) Validate() error {
	switch s.Resolution {
	case "env", "file":
	default:
		return fmt.Errorf("env.secrets.resolution must be \"env\" or \"file\", got %q", s.Resolution)
	}
	return nil
}
