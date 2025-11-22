package configs

import (
	"os"
)

type Config struct {
	Env string

	PostgresURL string
	Port        string
}

func Load() *Config {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "test"
	}

	cfg := &Config{
		Env:  env,
		Port: getEnv("PORT", "8080"),
	}

	switch env {
	case "prod":
		cfg.PostgresURL = getEnv("DATABASE_URL",
			"postgres://postgres:postgres@postgres:5432/avito?sslmode=disable") // В проде будем работать с сертификатом
	case "test":
		cfg.PostgresURL = getEnv("DATABASE_URL",
			"postgres://postgres:postgres@localhost:5432/avito?sslmode=disable")
	}

	return cfg
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
