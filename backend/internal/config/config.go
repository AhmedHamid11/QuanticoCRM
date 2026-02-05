package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

// Config holds all application configuration loaded from environment variables.
// Validates required settings at startup to fail fast on misconfiguration.
type Config struct {
	// Environment is "development", "production", or "prod"
	Environment string

	// JWTSecret is the secret used to sign JWT tokens.
	// REQUIRED in production - app will not start without it.
	JWTSecret string

	// AllowedOrigins is a list of origins allowed for CORS.
	// In development, defaults to allowing all ("*").
	// In production, should be explicitly set.
	AllowedOrigins []string

	// AuthRateLimit is the max login attempts per minute per IP.
	// Default: 5
	AuthRateLimit int

	// TursoURL is the Turso database URL (indicates production mode).
	TursoURL string

	// TursoToken is the Turso authentication token.
	TursoToken string
}

// Load reads configuration from environment variables and validates required settings.
// Calls log.Fatal if required production settings are missing.
func Load() *Config {
	cfg := &Config{
		Environment:   getEnv("ENVIRONMENT", "development"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		TursoURL:      os.Getenv("TURSO_URL"),
		TursoToken:    os.Getenv("TURSO_AUTH_TOKEN"),
		AuthRateLimit: getEnvInt("AUTH_RATE_LIMIT", 20),
	}

	// Parse ALLOWED_ORIGINS into slice
	originsEnv := os.Getenv("ALLOWED_ORIGINS")
	if originsEnv == "" {
		if cfg.IsProduction() {
			log.Println("WARNING: ALLOWED_ORIGINS not set in production. Set this to restrict CORS.")
		}
		cfg.AllowedOrigins = []string{"*"}
	} else {
		cfg.AllowedOrigins = parseOrigins(originsEnv)
	}

	// Production validation: JWT_SECRET is required
	if cfg.IsProduction() && cfg.JWTSecret == "" {
		log.Fatal("FATAL: JWT_SECRET environment variable must be set in production")
	}

	// Warn about wildcard origins in production
	if cfg.IsProduction() && len(cfg.AllowedOrigins) == 1 && cfg.AllowedOrigins[0] == "*" {
		log.Println("WARNING: ALLOWED_ORIGINS is '*' in production. This allows any origin.")
	}

	return cfg
}

// IsDevelopment returns true if running in development environment.
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development" || c.Environment == ""
}

// IsProduction returns true if running in production environment.
// Production is indicated by ENVIRONMENT=production/prod OR presence of TURSO_URL.
func (c *Config) IsProduction() bool {
	env := c.Environment
	if env == "production" || env == "prod" {
		return true
	}
	// If TURSO_URL is set, we're connecting to production database
	return c.TursoURL != ""
}

// GetJWTSecret returns the JWT secret, using a default in development only.
func (c *Config) GetJWTSecret() string {
	if c.JWTSecret == "" {
		if c.IsProduction() {
			// Should never reach here due to Load() validation
			log.Fatal("FATAL: JWT_SECRET not set in production")
		}
		log.Println("WARNING: Using default JWT secret. Set JWT_SECRET in production!")
		return "dev-secret-change-in-production"
	}
	return c.JWTSecret
}

// parseOrigins splits a comma-separated origins string into a slice.
// Trims whitespace from each origin.
func parseOrigins(s string) []string {
	parts := strings.Split(s, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}

// getEnv gets an environment variable or returns a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets an environment variable as int or returns a default value.
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
