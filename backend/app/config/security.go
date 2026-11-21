package config

import "time"

// SecurityConfig holds secrets and abuse-protection settings.
type SecurityConfig struct {
	JWTSecret     string
	QRSigningKey  string
	WebhookSecret string
	// PaymentKeySecret signs/verifies checkout signatures (gateway key secret).
	PaymentKeySecret string

	RateLimitPerMinute int
	QRTTL              time.Duration
}

func loadSecurity() SecurityConfig {
	return SecurityConfig{
		JWTSecret:          getEnv("JWT_SECRET", "dev-jwt-secret-change-me"),
		QRSigningKey:       getEnv("QR_SIGNING_KEY", "dev-qr-key-change-me"),
		WebhookSecret:      getEnv("PAYMENT_WEBHOOK_SECRET", "dev-webhook-secret"),
		PaymentKeySecret:   getEnv("PAYMENT_KEY_SECRET", "dev-payment-key-secret"),
		RateLimitPerMinute: getInt("RATE_LIMIT_PER_MINUTE", 120),
		QRTTL:              getDuration("QR_TTL", 4*time.Hour),
	}
}
