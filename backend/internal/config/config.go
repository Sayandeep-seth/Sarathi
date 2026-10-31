// Package config loads and validates SARATHI API configuration from the environment.
package config

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime settings for the API process.
type Config struct {
	Env                string // local | staging | production
	HTTPAddr           string
	DatabaseURL        string
	RedisAddr          string
	RedisPassword      string
	LogLevel           slog.Level
	ShutdownTimeout    time.Duration
	RequestTimeout     time.Duration
	MaxBodyBytes       int64
	CORSAllowedOrigins []string
	Version            string
}

// Load reads configuration using the supplied getenv function (os.Getenv in production).
func Load(getenv func(string) string) (Config, error) {
	c := Config{
		Env:           def(getenv("SARATHI_ENV"), "local"),
		HTTPAddr:      def(getenv("SARATHI_HTTP_ADDR"), ":8080"),
		DatabaseURL:   getenv("SARATHI_DATABASE_URL"),
		RedisAddr:     getenv("SARATHI_REDIS_ADDR"),
		RedisPassword: getenv("SARATHI_REDIS_PASSWORD"),
		Version:       def(getenv("SARATHI_VERSION"), "dev"),
	}

	switch c.Env {
	case "local", "staging", "production":
	default:
		return c, fmt.Errorf("SARATHI_ENV must be local, staging or production, got %q", c.Env)
	}

	if err := c.LogLevel.UnmarshalText([]byte(def(getenv("SARATHI_LOG_LEVEL"), "info"))); err != nil {
		return c, fmt.Errorf("SARATHI_LOG_LEVEL: %w", err)
	}

	var err error
	if c.ShutdownTimeout, err = duration(getenv, "SARATHI_SHUTDOWN_TIMEOUT", 15*time.Second); err != nil {
		return c, err
	}
	if c.RequestTimeout, err = duration(getenv, "SARATHI_REQUEST_TIMEOUT", 10*time.Second); err != nil {
		return c, err
	}

	c.MaxBodyBytes = 1 << 20 // 1 MiB
	if v := getenv("SARATHI_MAX_BODY_BYTES"); v != "" {
		n, perr := strconv.ParseInt(v, 10, 64)
		if perr != nil || n <= 0 {
			return c, fmt.Errorf("SARATHI_MAX_BODY_BYTES must be a positive integer, got %q", v)
		}
		c.MaxBodyBytes = n
	}

	for _, o := range strings.Split(getenv("SARATHI_CORS_ALLOWED_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.CORSAllowedOrigins = append(c.CORSAllowedOrigins, o)
		}
	}

	if c.DatabaseURL == "" {
		return c, fmt.Errorf("SARATHI_DATABASE_URL is required")
	}
	if c.Env == "production" {
		for _, o := range c.CORSAllowedOrigins {
			if o == "*" {
				return c, fmt.Errorf("wildcard CORS origin is not allowed in production")
			}
		}
	}
	return c, nil
}

func def(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func duration(getenv func(string) string, key string, fallback time.Duration) (time.Duration, error) {
	v := getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration, got %q", key, v)
	}
	return d, nil
}
