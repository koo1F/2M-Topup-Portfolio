package config

import (
	"os"
	"strconv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Port                string
	DatabaseURL         string
	RedisURL            string
	JWTSecret           string
	WebhookSecret       string
	PaymentRateLimit    int
	StripeSecretKey     string
	StripeWebhookSecret string
	StripeSuccessURL    string
	StripeCancelURL     string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	rateLimitStr := getEnv("PAYMENT_RATE_LIMIT", "200")
	rateLimit, err := strconv.Atoi(rateLimitStr)
	if err != nil {
		rateLimit = 200
	}

	return &Config{
		Port:                getEnv("PORT", "8080"),
		DatabaseURL:         getEnv("DATABASE_URL", "postgres://user:password@localhost:5432/payment_db?sslmode=disable"),
		RedisURL:            getEnv("REDIS_URL", "localhost:6379"),
		JWTSecret:           getEnv("JWT_SECRET", "change-me-in-production-use-a-long-random-string"),
		WebhookSecret:       getEnv("WEBHOOK_SECRET", "webhook-secret-change-in-production"),
		PaymentRateLimit:    rateLimit,
		StripeSecretKey:     getEnv("STRIPE_SECRET_KEY", ""),
		StripeWebhookSecret: getEnv("STRIPE_WEBHOOK_SECRET", ""),
		StripeSuccessURL:    getEnv("STRIPE_SUCCESS_URL", "http://localhost:3000/payment/success"),
		StripeCancelURL:     getEnv("STRIPE_CANCEL_URL", "http://localhost:3000/payment/cancel"),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
