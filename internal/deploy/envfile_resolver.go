package deploy

func ResolveEnvFile(envFile EnvFile, environment string) map[string]string {
	resolved := map[string]string{}

	for _, section := range envFile.Sections {
		for _, item := range section.Items {
			if item.Key == "" {
				continue
			}

			if value, ok := item.Values[environment]; ok {
				resolved[item.Key] = value
				continue
			}

			if item.Value != "" {
				resolved[item.Key] = item.Value
			}
		}
	}

	return resolved
}
