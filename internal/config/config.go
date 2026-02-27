package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port           string
	DatabaseURL    string
	AllowedOrigins []string
	LogLevel       slog.Level

	// Tiered rate limiting: auth endpoints are stricter than general API.
	// Default: /auth/* = 30 req/min (0.5 rps, burst 5)
	//          /api/*  = 120 req/min (2 rps, burst 20)
	AuthRateLimitRPS   float64
	AuthRateLimitBurst int
	APIRateLimitRPS    float64
	APIRateLimitBurst  int

	// Deprecated single-tier fields kept for backward compatibility.
	RateLimitRPS   float64
	RateLimitBurst int

	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	MaxBodySize  int64
	Environment  string

	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	JWTSecret          string
	JWTExpiry          time.Duration
	AdminEmails        map[string]bool

	// CookieSecure controls the Secure flag on session cookies.
	// Defaults to true in production, false otherwise.
	// Override with COOKIE_SECURE=false for local HTTP development.
	CookieSecure bool
}

func Load() *Config {
	cfg := &Config{
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        requireEnv("DATABASE_URL"),
		AllowedOrigins:     parseOrigins(getEnv("ALLOWED_ORIGINS", "")),
		LogLevel:           parseLogLevel(getEnv("LOG_LEVEL", "info")),
		AuthRateLimitRPS:   parseFloat(getEnv("AUTH_RATE_LIMIT_RPS", "0.5")),
		AuthRateLimitBurst: parseInt(getEnv("AUTH_RATE_LIMIT_BURST", "5")),
		APIRateLimitRPS:    parseFloat(getEnv("API_RATE_LIMIT_RPS", "2")),
		APIRateLimitBurst:  parseInt(getEnv("API_RATE_LIMIT_BURST", "20")),
		// Legacy single-tier fields (ignored when tiered limits are set).
		RateLimitRPS:   parseFloat(getEnv("RATE_LIMIT_RPS", "2")),
		RateLimitBurst: parseInt(getEnv("RATE_LIMIT_BURST", "20")),
		ReadTimeout:    parseDuration(getEnv("READ_TIMEOUT", "5s")),
		WriteTimeout:   parseDuration(getEnv("WRITE_TIMEOUT", "10s")),
		IdleTimeout:    parseDuration(getEnv("IDLE_TIMEOUT", "120s")),
		MaxBodySize:    parseInt64(getEnv("MAX_BODY_SIZE", "1048576")),
		Environment:    getEnv("ENVIRONMENT", "development"),

		GoogleClientID:     requireEnv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: requireEnv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  requireEnv("GOOGLE_REDIRECT_URL"),
		JWTSecret:          requireEnv("JWT_SECRET"),
		JWTExpiry:          parseDuration(getEnv("JWT_EXPIRY", "24h")),
		AdminEmails:        parseEmailSet(getEnv("ADMIN_EMAILS", "")),
		CookieSecure:       parseCookieSecure(getEnv("COOKIE_SECURE", ""), getEnv("ENVIRONMENT", "development")),
	}
	validateConfig(cfg)
	return cfg
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func requireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		slog.Error("required environment variable not set", "key", key)
		os.Exit(1)
	}
	return val
}

// validateConfig performs post-load security checks and exits on failure,
// preventing the server from starting with insecure configuration.
func validateConfig(cfg *Config) {
	if len(cfg.JWTSecret) < 32 {
		slog.Error("JWT_SECRET is too short — minimum 32 characters required",
			"got", len(cfg.JWTSecret),
			"tip", "generate a secure secret with: openssl rand -base64 48")
		os.Exit(1)
	}
}

func parseOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			origins = append(origins, v)
		}
	}
	return origins
}

func parseLogLevel(raw string) slog.Level {
	switch strings.ToLower(raw) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func parseFloat(raw string) float64 {
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 10
	}
	return v
}

func parseInt(raw string) int {
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 20
	}
	return v
}

func parseInt64(raw string) int64 {
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 1 << 20
	}
	return v
}

func parseDuration(raw string) time.Duration {
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 5 * time.Second
	}
	return d
}

func parseCookieSecure(raw, environment string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	default:
		return environment == "production"
	}
}

func parseEmailSet(raw string) map[string]bool {
	m := make(map[string]bool)
	if raw == "" {
		return m
	}
	for _, e := range strings.Split(raw, ",") {
		if v := strings.TrimSpace(strings.ToLower(e)); v != "" {
			m[v] = true
		}
	}
	return m
}
