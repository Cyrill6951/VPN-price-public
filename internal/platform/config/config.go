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

	// Auth
	JWTSecret        string        // HMAC secret for access tokens
	AccessTokenTTL   time.Duration // access token lifetime
	RefreshTokenTTL  time.Duration // refresh token lifetime
	TelegramBotToken string        // used to verify Telegram Login signatures

	// Object storage (MinIO/S3) for VPN configs and QR codes.
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	MinioUseSSL    bool

	// VPN
	VPNConfigKeyHex       string // hex-encoded 32-byte AES key for config encryption
	ProvisionerMode       string // noop | agent
	ProvisionerAgentToken string // bearer token for the node agent (agent mode)

	// Billing
	PublicBaseURL     string        // externally reachable base URL (webhook callbacks)
	CryptomusMerchant string        // Cryptomus merchant id (optional)
	CryptomusAPIKey   string        // Cryptomus API key (optional)
	GracePeriod       time.Duration // subscription grace period after expiry
	StarsPerUSD       float64       // Telegram Stars per USD for invoice pricing
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

		JWTSecret:        os.Getenv("JWT_SECRET"),
		AccessTokenTTL:   getdur("JWT_ACCESS_TTL", 15*time.Minute),
		RefreshTokenTTL:  getdur("JWT_REFRESH_TTL", 30*24*time.Hour),
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),

		MinioEndpoint:  getenv("MINIO_ENDPOINT", "minio:9000"),
		MinioAccessKey: getenv("MINIO_ACCESS_KEY", getenv("MINIO_ROOT_USER", "minioadmin")),
		MinioSecretKey: getenv("MINIO_SECRET_KEY", getenv("MINIO_ROOT_PASSWORD", "minioadmin")),
		MinioBucket:    getenv("MINIO_BUCKET", "vpn-configs"),
		MinioUseSSL:    getbool("MINIO_USE_SSL", false),

		VPNConfigKeyHex: getenv("VPN_CONFIG_KEY",
			"00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"),
		ProvisionerMode:       getenv("PROVISIONER_MODE", "noop"),
		ProvisionerAgentToken: os.Getenv("PROVISIONER_AGENT_TOKEN"),

		PublicBaseURL:     getenv("PUBLIC_BASE_URL", "http://localhost:8080"),
		CryptomusMerchant: os.Getenv("CRYPTOMUS_MERCHANT"),
		CryptomusAPIKey:   os.Getenv("CRYPTOMUS_API_KEY"),
		GracePeriod:       getdur("GRACE_PERIOD", 72*time.Hour),
		StarsPerUSD:       getfloat("STARS_PER_USD", 1),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("config: DATABASE_URL is required")
	}
	if cfg.RedisURL == "" {
		return nil, fmt.Errorf("config: REDIS_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("config: JWT_SECRET is required")
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

func getfloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getbool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
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
