// Package config loads application configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration for the API service.
type Config struct {
	Env             string        // development | staging | production
	HTTPAddr        string        // host:port to listen on
	ShutdownTimeout time.Duration // graceful shutdown grace period

	DatabaseURL string // postgres connection string (pgx)
	RedisURL    string // redis connection string

	LogLevel string // debug | info | warn | error
}

// Load reads configuration from the environment, applying sane defaults.
// It returns an error only when a required value is missing or malformed.
func Load() (*Config, error) {
	cfg := &Config{
		Env:             getenv("APP_ENV", "development"),
		HTTPAddr:        getenv("HTTP_ADDR", ":8080"),
		ShutdownTimeout: getdur("SHUTDOWN_TIMEOUT", 15*time.Second),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		RedisURL:        os.Getenv("REDIS_URL"),
		LogLevel:        getenv("LOG_LEVEL", "info"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("config: DATABASE_URL is required")
	}
	if cfg.RedisURL == "" {
		return nil, fmt.Errorf("config: REDIS_URL is required")
	}
	return cfg, nil
}

// IsProduction reports whether the service runs in a production environment.
func (c *Config) IsProduction() bool { return c.Env == "production" }

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getdur(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

// getint is kept for forthcoming numeric settings (pool sizes, limits).
func getint(key string, fallback int) int { //nolint:unused // used as config grows
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
