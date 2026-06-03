package app

import "testing"

func TestLoadConfig(t *testing.T) {
	t.Setenv("PORT", "9999")
	t.Setenv("CACHE_HOST", "redis")
	t.Setenv("CACHE_PORT", "bad")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://a, http://b")
	cfg := LoadConfig()
	if cfg.Port != "9999" || cfg.RedisAddr != "redis:6379" {
		t.Fatalf("config mismatch: %#v", cfg)
	}
	if len(cfg.CORSOrigins) != 2 || splitList(" , ")[0] != "*" {
		t.Fatalf("origin split failed")
	}
	if env("MISSING", "fallback") != "fallback" {
		t.Fatalf("env fallback failed")
	}
}
