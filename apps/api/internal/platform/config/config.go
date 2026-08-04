package config

import (
	"fmt"
	"os"
	"strconv"
)

// Execution result size bounds for n8n Query/Execute payloads.
const (
	DefaultN8NExecutionResultMaxBytes = 4 << 20  // 4 MiB covers four-stage body-scale outputs
	MinN8NExecutionResultMaxBytes     = 64 << 10 // 64 KiB
	MaxN8NExecutionResultMaxBytes     = 32 << 20 // 32 MiB hard ceiling
)

type Config struct {
	Environment                      string
	APIAddress                       string
	DatabaseURL                      string
	RedisURL                         string
	ConfigurationEncryptionKey       string
	ChapterPlanIdempotencyHMACSecret string
	// N8NExecutionResultMaxBytes caps durable n8n execution result bodies.
	N8NExecutionResultMaxBytes int
}

// Load reads process configuration. Invalid ACF_N8N_EXECUTION_RESULT_MAX_BYTES
// fails closed via panic so the process never starts without a controlled limit.
func Load() Config {
	maxBytes, err := ParseN8NExecutionResultMaxBytes(os.Getenv("ACF_N8N_EXECUTION_RESULT_MAX_BYTES"))
	if err != nil {
		panic(err)
	}
	return Config{
		Environment:                      envOrDefault("APP_ENV", "development"),
		APIAddress:                       ":" + envOrDefault("API_PORT", "8080"),
		DatabaseURL:                      envOrDefault("DATABASE_URL", "postgres://acf:acf@localhost:5432/acf?sslmode=disable"),
		RedisURL:                         envOrDefault("REDIS_URL", "redis://localhost:6379/0"),
		ConfigurationEncryptionKey:       os.Getenv("CONFIGURATION_ENCRYPTION_KEY"),
		ChapterPlanIdempotencyHMACSecret: os.Getenv("CHAPTER_PLAN_IDEMPOTENCY_HMAC_SECRET"),
		N8NExecutionResultMaxBytes:       maxBytes,
	}
}

// ParseN8NExecutionResultMaxBytes returns the configured cap or the safe default.
// Empty input uses DefaultN8NExecutionResultMaxBytes; invalid values return error
// so callers never silently disable the limit.
func ParseN8NExecutionResultMaxBytes(raw string) (int, error) {
	if raw == "" {
		return DefaultN8NExecutionResultMaxBytes, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("ACF_N8N_EXECUTION_RESULT_MAX_BYTES must be an integer: %w", err)
	}
	if value < MinN8NExecutionResultMaxBytes || value > MaxN8NExecutionResultMaxBytes {
		return 0, fmt.Errorf("ACF_N8N_EXECUTION_RESULT_MAX_BYTES must be between %d and %d", MinN8NExecutionResultMaxBytes, MaxN8NExecutionResultMaxBytes)
	}
	return value, nil
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
