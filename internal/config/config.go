package config

type Config struct {
	AppName  string
	StateDir string
}

func Load() (Config, error) {
	return Config{
		AppName:  "noops",
		StateDir: "/var/lib/noops",
	}, nil
}
