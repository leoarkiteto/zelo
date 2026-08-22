// Package config loads and validates environment-based configuration.
package config

import (
	"errors"
	"fmt"
	"os"
)

// Config holds all runtime configuration loaded from the environment.
type Config struct {
	DatabaseURL   string
	SessionSecret []byte
	AppEnv        string
	HTTPAddr      string
}

// Load reads configuration from the environment and validates required values.
func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		SessionSecret: []byte(os.Getenv("SESSION_SECRET")),
		AppEnv:        os.Getenv("APP_ENV"),
		HTTPAddr:      os.Getenv("HTTP_ADDR"),
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
	}
	if cfg.AppEnv == "" {
		cfg.AppEnv = "development"
	}

	var errs []error
	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if len(cfg.SessionSecret) == 0 {
		errs = append(errs, errors.New("SESSION_SECRET is required"))
	}
	if cfg.AppEnv != "development" && cfg.AppEnv != "production" {
		errs = append(errs, fmt.Errorf("APP_ENV must be development or production, got %q", cfg.AppEnv))
	}
	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}
	return cfg, nil
}

// IsProduction reports whether the application runs in production mode.
func (c Config) IsProduction() bool {
	return c.AppEnv == "production"
}
