package deploy

type ResolvedEnv struct {
	SecretRefs []EnvSecretRef
	Values     map[string]string
}

type EnvSecretRef struct {
	Key        string
	SecretName string
}

func ResolveEnvFile(envFile EnvFile, environment string) ResolvedEnv {
	resolved := ResolvedEnv{Values: map[string]string{}}

	for _, section := range envFile.Sections {
		for _, item := range section.Items {
			if item.Key == "" {
				continue
			}
			if item.FromSecret != "" {
				resolved.SecretRefs = append(resolved.SecretRefs, EnvSecretRef{Key: item.Key, SecretName: item.FromSecret})
				continue
			}

			if value, ok := item.Values[environment]; ok {
				resolved.Values[item.Key] = value
				continue
			}

			if item.Value != "" {
				resolved.Values[item.Key] = item.Value
			}
		}
	}

	return resolved
}
