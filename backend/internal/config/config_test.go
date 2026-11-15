package config

import (
	"testing"
	"time"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestLoadDefaults(t *testing.T) {
	c, err := Load(env(map[string]string{"SARATHI_DATABASE_URL": "postgres://x"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Env != "local" || c.HTTPAddr != ":8080" || c.MaxBodyBytes != 1<<20 || c.ShutdownTimeout != 15*time.Second {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestLoadRejects(t *testing.T) {
	cases := map[string]map[string]string{
		"missing db":         {},
		"bad env":            {"SARATHI_DATABASE_URL": "x", "SARATHI_ENV": "dev"},
		"bad log level":      {"SARATHI_DATABASE_URL": "x", "SARATHI_LOG_LEVEL": "loud"},
		"bad body limit":     {"SARATHI_DATABASE_URL": "x", "SARATHI_MAX_BODY_BYTES": "-1"},
		"bad timeout":        {"SARATHI_DATABASE_URL": "x", "SARATHI_REQUEST_TIMEOUT": "soon"},
		"prod wildcard cors": {"SARATHI_DATABASE_URL": "x", "SARATHI_ENV": "production", "SARATHI_CORS_ALLOWED_ORIGINS": "https://a.example, *"},
		"prod no jwt secret": {"SARATHI_DATABASE_URL": "x", "SARATHI_ENV": "production"},
		"prod log otp":       prod(map[string]string{"SARATHI_OTP_SENDER": "log"}),
		"prod mock payments": prod(map[string]string{"SARATHI_PAYMENT_PROVIDER": "mock"}),
		"prod http webhook":  prod(map[string]string{"SARATHI_OTP_WEBHOOK_URL": "http://sms.example"}),
		"prod short gateway": prod(map[string]string{"SARATHI_GATEWAY_TOKEN": "short"}),
	}
	for name, m := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(env(m)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestCORSParsing(t *testing.T) {
	c, err := Load(env(map[string]string{"SARATHI_DATABASE_URL": "x", "SARATHI_CORS_ALLOWED_ORIGINS": " https://a.example ,https://b.example,"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.CORSAllowedOrigins) != 2 || c.CORSAllowedOrigins[1] != "https://b.example" {
		t.Fatalf("got %v", c.CORSAllowedOrigins)
	}
}

// prod returns a valid production environment with the given overrides applied.
func prod(over map[string]string) map[string]string {
	m := map[string]string{
		"SARATHI_DATABASE_URL":    "x",
		"SARATHI_ENV":             "production",
		"SARATHI_JWT_SECRET":      "0123456789abcdef0123456789abcdef",
		"SARATHI_GATEWAY_TOKEN":   "0123456789abcdef0123456789abcdef",
		"SARATHI_OTP_WEBHOOK_URL": "https://sms.example/send",
	}
	for k, v := range over {
		m[k] = v
	}
	return m
}

func TestProductionConfig(t *testing.T) {
	c, err := Load(env(prod(nil)))
	if err != nil {
		t.Fatal(err)
	}
	if c.Simulator || c.OTPDevEcho() || c.PaymentProvider != "none" || c.OTPSender != "webhook" {
		t.Fatalf("unsafe production defaults: %+v", c)
	}
}

func TestLocalDefaultsEnableDemoHelpers(t *testing.T) {
	c, err := Load(env(map[string]string{"SARATHI_DATABASE_URL": "x"}))
	if err != nil {
		t.Fatal(err)
	}
	if !c.Simulator || !c.OTPDevEcho() || c.PaymentProvider != "mock" {
		t.Fatalf("unexpected local defaults: %+v", c)
	}
}
