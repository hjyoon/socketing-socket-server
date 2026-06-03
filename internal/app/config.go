package app

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port              string
	JWTSecret         string
	EntranceJWTSecret string
	RedisAddr         string
	DBURL             string
	SchedulingURL     string
	CORSOrigins       []string
}

func LoadConfig() Config {
	return Config{
		Port:              env("PORT", "3000"),
		JWTSecret:         env("JWT_SECRET", "my-jwt-secret"),
		EntranceJWTSecret: env("JWT_SECRET_FOR_ENTRANCE", "my-jwt-secret"),
		RedisAddr:         redisAddr(),
		DBURL:             env("DB_URL", "postgres://postgres:password@localhost:5432/socketing?sslmode=disable"),
		SchedulingURL:     env("SCHEDULING_SERVER_URL", "http://localhost:3001/"),
		CORSOrigins:       splitList(env("CORS_ALLOWED_ORIGINS", "*")),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func redisAddr() string {
	host := env("CACHE_HOST", "localhost")
	port := env("CACHE_PORT", "6379")
	if _, err := strconv.Atoi(port); err != nil {
		port = "6379"
	}
	return host + ":" + port
}

func splitList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if item := strings.TrimSpace(part); item != "" {
			out = append(out, item)
		}
	}
	if len(out) == 0 {
		return []string{"*"}
	}
	return out
}
