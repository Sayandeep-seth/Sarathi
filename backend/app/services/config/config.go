// Package config loads all runtime configuration from environment variables.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config is the root configuration shared by the api, worker and migrate binaries.
type Config struct {
	Env        string
	HTTPAddr   string
	InstanceID string
	AIEndpoint string

	Postgres PostgresConfig
	Redis    RedisConfig
	Map      MapConfig
	Security SecurityConfig
	Notify   NotifyConfig
}

// NotifyConfig holds push / SMS / email provider settings (empty = disabled).
type NotifyConfig struct {
	FCMURL, FCMKey               string
	SMSURL, SMSKey, SMSSender    string
	SMTPHost, SMTPUser, SMTPPass string
	SMTPPort                     int
	MailFrom                     string
}

// Load reads configuration from the environment, falling back to sane local defaults.
func Load() *Config {
	host, _ := os.Hostname()
	return &Config{
		Env:        getEnv("APP_ENV", "development"),
		HTTPAddr:   getEnv("HTTP_ADDR", ":8080"),
		InstanceID: getEnv("INSTANCE_ID", host),
		AIEndpoint: getEnv("AI_ENDPOINT", ""),
		Postgres:   loadPostgres(),
		Redis:      loadRedis(),
		Map:        loadMap(),
		Security:   loadSecurity(),
		Notify: NotifyConfig{
			FCMURL: getEnv("FCM_URL", ""), FCMKey: getEnv("FCM_KEY", ""),
			SMSURL: getEnv("SMS_URL", ""), SMSKey: getEnv("SMS_KEY", ""), SMSSender: getEnv("SMS_SENDER", "SARATI"),
			SMTPHost: getEnv("SMTP_HOST", ""), SMTPPort: getInt("SMTP_PORT", 0),
			SMTPUser: getEnv("SMTP_USER", ""), SMTPPass: getEnv("SMTP_PASS", ""),
			MailFrom: getEnv("MAIL_FROM", "no-reply@sarathi.app"),
		},
	}
}

// IsProduction reports whether we run in production mode.
func (c *Config) IsProduction() bool { return c.Env == "production" }

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return v
	}
	return def
}

func getFloat(key string, def float64) float64 {
	if v, err := strconv.ParseFloat(os.Getenv(key), 64); err == nil {
		return v
	}
	return def
}

func getDuration(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(os.Getenv(key)); err == nil {
		return v
	}
	return def
}
