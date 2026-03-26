package config

type Config struct {
	AppName string
}

func Load() (Config, error) {
	return Config{
		AppName: "noops",
	}, nil
}
