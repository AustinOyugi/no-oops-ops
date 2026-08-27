package deploy

import "github.com/AustinOyugi/no-oops-ops/internal/environment"

type EnvFile = environment.File
type EnvSection = environment.Section
type EnvItem = environment.Item

func LoadEnvFile(path string) (EnvFile, error) {
	return environment.Load(path)
}

// LoadOptionalEnvFile returns an empty environment when a manifest does not
// declare x-noops.env.file. Static applications commonly need no deployment
// environment or secret bindings.
func LoadOptionalEnvFile(path string) (EnvFile, error) {
	return environment.LoadOptional(path)
}
