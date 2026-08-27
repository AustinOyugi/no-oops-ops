package deploy

import "github.com/AustinOyugi/no-oops-ops/internal/environment"

type ResolvedEnv = environment.Resolved
type EnvSecretRef = environment.SecretRef

func ResolveEnvFile(envFile EnvFile, target string, resolvable []string) ResolvedEnv {
	return environment.Resolve(envFile, target, resolvable)
}
