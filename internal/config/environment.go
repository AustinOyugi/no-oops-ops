package config

import (
	"fmt"
	"strings"
)

// RequireEnvironment validates an explicit flag or NOOPS_DEFAULT_ENV selection.
func RequireEnvironment(environment string) error {
	if environment == "" {
		return fmt.Errorf("environment is required; select one with -e / --environment or set NOOPS_DEFAULT_ENV")
	}
	if !environmentNamePattern.MatchString(environment) {
		return fmt.Errorf("invalid environment %q", environment)
	}
	return nil
}

// SelectEnvironment resolves an explicit flag, a legacy positional selection,
// and the optional process default. Conflicting explicit selections are rejected.
func SelectEnvironment(flag string, flagSet bool, legacy, fallback string) (string, error) {
	flag, legacy = strings.TrimSpace(flag), strings.TrimSpace(legacy)
	if flagSet {
		if legacy != "" && flag != legacy {
			return "", fmt.Errorf("--environment %q conflicts with positional environment %q", flag, legacy)
		}
		return flag, nil
	}
	if legacy != "" {
		return legacy, nil
	}
	return strings.TrimSpace(fallback), nil
}
