package config

type Config struct {
	AppName        string
	StateDir       string
	InstallVersion string
}

func Load() (Config, error) {
	return Config{
		AppName:        "noops",
		StateDir:       "/Users/odu/Documents/alien/code-innate/personal/no-oops-ops/.noops",
		InstallVersion: "prod",
	}, nil
}
