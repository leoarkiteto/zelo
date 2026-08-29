// Package config loads and validates environment-based configuration.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
)

// Config holds all runtime configuration loaded from the environment.
type Config struct {
	DatabaseURL    string
	RedisURL       string
	PasswordPepper string
	AppEnv         string
	HTTPAddr       string
	UploadDir      string
	SessionSecret  []byte
}

// Load reads configuration from the environment and validates required values.
func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		RedisURL:       getEnv("REDIS_URL", "redis://localhost:6379/0"),
		SessionSecret:  []byte(os.Getenv("SESSION_SECRET")),
		PasswordPepper: os.Getenv("PASSWORD_PEPPER"),
		AppEnv:         getEnv("APP_ENV", "development"),
		HTTPAddr:       getEnv("HTTP_ADDR", ":8080"),
		UploadDir:      getEnv("UPLOAD_DIR", "uploads"),
	}

	var errs []error
	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if len(cfg.SessionSecret) == 0 {
		errs = append(errs, errors.New("SESSION_SECRET is required"))
	}
	if len(cfg.PasswordPepper) == 0 {
		errs = append(errs, errors.New("PASSWORD_PEPPER is required"))
	}
	if err := validateRedisURL(cfg.RedisURL); err != nil {
		errs = append(errs, err)
	}
	if cfg.AppEnv != "development" && cfg.AppEnv != "production" {
		errs = append(
			errs,
			fmt.Errorf("APP_ENV must be development or production, got %q", cfg.AppEnv),
		)
	}
	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}
	return cfg, nil
}

// validateRedisURL ensures REDIS_URL is parseable and uses the redis or
// rediss scheme so connection errors surface at startup, not on first login.
func validateRedisURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("REDIS_URL is invalid: %w", err)
	}
	if u.Scheme != "redis" && u.Scheme != "rediss" {
		return fmt.Errorf("REDIS_URL scheme must be redis or rediss, got %q", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("REDIS_URL must include a host")
	}
	return nil
}

// IsProduction reports whether the application runs in production mode.
func (c Config) IsProduction() bool {
	return c.AppEnv == "production"
}

func getEnv(key string, fallBack string) string {
	keyEnv := os.Getenv(key)
	if keyEnv == "" {
		return fallBack
	}

	return keyEnv
}
