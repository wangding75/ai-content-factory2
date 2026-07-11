package config

import (
	"os"
)

type Config struct {
	Environment string
	APIAddress  string
	DatabaseURL string
	RedisURL    string
}

func Load() Config {
	return Config{
		Environment: envOrDefault("APP_ENV", "development"),
		APIAddress:  ":" + envOrDefault("API_PORT", "8080"),
		DatabaseURL: envOrDefault("DATABASE_URL", "postgres://acf:acf@localhost:5432/acf?sslmode=disable"),
		RedisURL:    envOrDefault("REDIS_URL", "redis://localhost:6379/0"),
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
