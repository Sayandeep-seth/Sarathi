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

	MongoURL      string // optional: telemetry and trip events are dropped when empty
	MongoDatabase string

	JWTSecret       []byte
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	OTPSender       string // log | webhook
	OTPWebhookURL   string
	OTPWebhookToken string

	PaymentProvider string // mock | none
	GatewayToken    string // shared secret presented by cab gateways as X-Gateway-Token
	AdminPhones     []string
	TrustProxy      bool // honour X-Real-IP from the ingress for rate limiting

	Simulator     bool
	SimulatorCabs int
	SimulatorRate float64 // simulated seconds per real second
}

// localJWTSecret is used only when SARATHI_ENV=local and no secret is configured.
const localJWTSecret = "local-development-only-secret-do-not-use-anywhere-else"

// OTPDevEcho reports whether OTP codes may be returned in API responses.
func (c Config) OTPDevEcho() bool { return c.Env == "local" && c.OTPSender == "log" }

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

	c.MongoURL = getenv("SARATHI_MONGO_URL")
	c.MongoDatabase = def(getenv("SARATHI_MONGO_DATABASE"), "sarathi")
	c.OTPWebhookURL = getenv("SARATHI_OTP_WEBHOOK_URL")
	c.OTPWebhookToken = getenv("SARATHI_OTP_WEBHOOK_TOKEN")
	c.GatewayToken = getenv("SARATHI_GATEWAY_TOKEN")
	c.TrustProxy = getenv("SARATHI_TRUST_PROXY") == "true"
	for _, p := range strings.Split(getenv("SARATHI_ADMIN_PHONES"), ",") {
		if p = strings.TrimSpace(p); p != "" {
			c.AdminPhones = append(c.AdminPhones, p)
		}
	}
	if c.AccessTokenTTL, err = duration(getenv, "SARATHI_ACCESS_TOKEN_TTL", 15*time.Minute); err != nil {
		return c, err
	}
	if c.RefreshTokenTTL, err = duration(getenv, "SARATHI_REFRESH_TOKEN_TTL", 30*24*time.Hour); err != nil {
		return c, err
	}

	local := c.Env == "local"
	secret := getenv("SARATHI_JWT_SECRET")
	if secret == "" && local {
		secret = localJWTSecret
	}
	if len(secret) < 32 {
		return c, fmt.Errorf("SARATHI_JWT_SECRET must be at least 32 characters")
	}
	c.JWTSecret = []byte(secret)

	c.OTPSender = def(getenv("SARATHI_OTP_SENDER"), map[bool]string{true: "log", false: "webhook"}[local])
	switch c.OTPSender {
	case "log":
		if c.Env == "production" {
			return c, fmt.Errorf("SARATHI_OTP_SENDER=log is not allowed in production")
		}
	case "webhook":
		if !strings.HasPrefix(c.OTPWebhookURL, "https://") && !(local && strings.HasPrefix(c.OTPWebhookURL, "http://")) {
			return c, fmt.Errorf("SARATHI_OTP_WEBHOOK_URL must be an https URL when SARATHI_OTP_SENDER=webhook")
		}
	default:
		return c, fmt.Errorf("SARATHI_OTP_SENDER must be log or webhook, got %q", c.OTPSender)
	}

	c.PaymentProvider = def(getenv("SARATHI_PAYMENT_PROVIDER"), map[bool]string{true: "mock", false: "none"}[local])
	switch c.PaymentProvider {
	case "none":
	case "mock":
		if c.Env == "production" {
			return c, fmt.Errorf("SARATHI_PAYMENT_PROVIDER=mock is not allowed in production")
		}
	default:
		return c, fmt.Errorf("SARATHI_PAYMENT_PROVIDER must be mock or none, got %q", c.PaymentProvider)
	}
	if !local && len(c.GatewayToken) < 32 {
		return c, fmt.Errorf("SARATHI_GATEWAY_TOKEN must be at least 32 characters outside local")
	}

	c.Simulator = local
	if v := getenv("SARATHI_SIMULATOR"); v != "" {
		c.Simulator = v == "true"
	}
	c.SimulatorCabs, c.SimulatorRate = 14, 4
	if v := getenv("SARATHI_SIMULATOR_CABS"); v != "" {
		n, perr := strconv.Atoi(v)
		if perr != nil || n < 1 || n > 200 {
			return c, fmt.Errorf("SARATHI_SIMULATOR_CABS must be 1-200, got %q", v)
		}
		c.SimulatorCabs = n
	}
	if v := getenv("SARATHI_SIMULATOR_RATE"); v != "" {
		f, perr := strconv.ParseFloat(v, 64)
		if perr != nil || f < 0.1 || f > 60 {
			return c, fmt.Errorf("SARATHI_SIMULATOR_RATE must be 0.1-60, got %q", v)
		}
		c.SimulatorRate = f
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
