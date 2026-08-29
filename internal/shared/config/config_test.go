package config

import (
	"strings"
	"testing"
)

func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	for k, v := range kv {
		t.Setenv(k, v)
	}
}

func validEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL":    "postgres://zelo:zelo@localhost:5432/zelo?sslmode=disable",
		"SESSION_SECRET":  "0123456789abcdef0123456789abcdef",
		"PASSWORD_PEPPER": "0123456789abcdef0123456789abcdef",
	}
}

func TestLoadRedisURLDefault(t *testing.T) {
	setEnv(t, validEnv())
	t.Setenv("REDIS_URL", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.RedisURL != "redis://localhost:6379/0" {
		t.Fatalf("RedisURL = %q, want default redis://localhost:6379/0", cfg.RedisURL)
	}
}

func TestLoadRedisURLExplicit(t *testing.T) {
	setEnv(t, validEnv())
	t.Setenv("REDIS_URL", "rediss://user:pass@cache.example.com:6380/2")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.RedisURL != "rediss://user:pass@cache.example.com:6380/2" {
		t.Fatalf("RedisURL = %q, want explicit value", cfg.RedisURL)
	}
}

func TestLoadRedisURLInvalid(t *testing.T) {
	setEnv(t, validEnv())
	t.Setenv("REDIS_URL", "not-a-url")
	if _, err := Load(); err == nil {
		t.Fatal("Load() = nil error for invalid REDIS_URL")
	}
}

func TestLoadRedisURLBadScheme(t *testing.T) {
	setEnv(t, validEnv())
	t.Setenv("REDIS_URL", "http://localhost:6379/0")
	_, err := Load()
	if err == nil {
		t.Fatal("Load() = nil error for non-redis scheme")
	}
	if !strings.Contains(err.Error(), "REDIS_URL") {
		t.Fatalf("error = %v, want mention of REDIS_URL", err)
	}
}
