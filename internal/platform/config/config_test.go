package config

import "testing"

func TestLoad_RequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")

	if _, err := Load(); err == nil {
		t.Fatal("expected error when DATABASE_URL is missing, got nil")
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("JWT_SECRET", "test-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr default = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.Env != "development" {
		t.Errorf("Env default = %q, want development", cfg.Env)
	}
	if cfg.IsProduction() {
		t.Error("IsProduction() = true, want false for development")
	}
	if cfg.AccessTokenTTL == 0 || cfg.RefreshTokenTTL == 0 {
		t.Error("token TTL defaults must be non-zero")
	}
}
