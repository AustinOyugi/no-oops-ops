package deploy

type ResolvedEnv struct {
	SecretRefs []EnvSecretRef
	Values     map[string]string
}

type EnvSecretRef struct {
	Key        string
	SecretName string
}

func ResolveEnvFile(envFile EnvFile, environment string, resolvable []string) ResolvedEnv {
	resolved := ResolvedEnv{Values: map[string]string{}}

	allowset := make(map[string]struct{}, len(resolvable))
	for _, key := range resolvable {
		allowset[key] = struct{}{}
	}
	hasAllowlist := resolvable != nil

	for _, section := range envFile.Sections {
		for _, item := range section.Items {
			if item.Key == "" {
				continue
			}
			if item.FromSecret != "" {
				if !hasAllowlist {
					resolved.SecretRefs = append(resolved.SecretRefs, EnvSecretRef{Key: item.Key, SecretName: item.FromSecret})
				} else if _, ok := allowset[item.Key]; ok {
					resolved.SecretRefs = append(resolved.SecretRefs, EnvSecretRef{Key: item.Key, SecretName: item.FromSecret})
				}
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
