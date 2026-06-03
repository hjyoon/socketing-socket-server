package app

import (
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Setenv("PORT", "9999")
	t.Setenv("CACHE_HOST", "redis")
	t.Setenv("CACHE_PORT", "bad")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://a, http://b")
	t.Setenv("DB_URL", "postgres://u:p@h:5432/db")
	cfg := LoadConfig()
	if cfg.Port != "9999" || cfg.RedisAddr != "redis:6379" {
		t.Fatalf("config mismatch: %#v", cfg)
	}
	if !strings.Contains(cfg.DBURL, "sslmode=disable") {
		t.Fatalf("sslmode missing: %s", cfg.DBURL)
	}
	if len(cfg.CORSOrigins) != 2 || splitList(" , ")[0] != "*" {
		t.Fatalf("origin split failed")
	}
	if env("MISSING", "fallback") != "fallback" {
		t.Fatalf("env fallback failed")
	}
	t.Setenv("DB_URL", "postgres://u:p@h:5432/db?sslmode=require")
	if dbURL() != "postgres://u:p@h:5432/db?sslmode=require" {
		t.Fatalf("existing sslmode changed")
	}
	t.Setenv("DB_URL", "host=postgres user=postgres")
	if dbURL() != "host=postgres user=postgres" {
		t.Fatalf("non-url dsn changed")
	}
}
