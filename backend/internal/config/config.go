// Package config loads application configuration from environment variables.
// Every setting has a sensible default so the app works out-of-the-box for
// local development without any configuration.
package config

import (
	"os"
	"strings"
)

// Config holds all runtime configuration for the TimeSync backend.
type Config struct {
	// Server
	Port        string // HTTP/HTTPS listen port (default: 8080)
	TLSEnabled  bool   // Enable TLS (default: false in dev)
	TLSCertFile string // Path to TLS certificate PEM file
	TLSKeyFile  string // Path to TLS private key PEM file

	// Security
	// AllowedOrigins is the comma-separated list of permitted CORS origins.
	// Use "*" only for local development; set explicit origins in production.
	AllowedOrigins []string

	// Maconomy defaults (can be overridden per-session)
	MaconomyBaseURL          string
	MaconomyCompany          string
	MaconomyAPIBasePath      string
	MaconomyOAUTHClientId    string
	MaconomyOAUTHRedirectURI string
}

// Load reads configuration from environment variables, applying defaults
// where variables are absent.
func Load() *Config {
	c := &Config{
		Port:        getEnv("PORT", "8080"),
		TLSEnabled:  getEnv("TLS_ENABLED", "false") == "true",
		TLSCertFile: getEnv("TLS_CERT_FILE", ""),
		TLSKeyFile:  getEnv("TLS_KEY_FILE", ""),

		MaconomyBaseURL:          getEnv("MACONOMY_BASE_URL", ""),
		MaconomyCompany:          getEnv("MACONOMY_COMPANY", ""),
		MaconomyAPIBasePath:      getEnv("MACONOMY_API_BASEPATH", ""),
		MaconomyOAUTHClientId:    getEnv("MACONOMY_OAUTH_CLIENT_ID", ""),
		MaconomyOAUTHRedirectURI: getEnv("MACONOMY_OAUTH_REDIRECT_URI", ""),
	}

	// Parse CORS origins.
	originsRaw := getEnv("ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:3000")
	for _, o := range strings.Split(originsRaw, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			c.AllowedOrigins = append(c.AllowedOrigins, o)
		}
	}

	return c
}

// getEnv returns the value of the environment variable named by key, or the
// fallback string if the variable is not set or is empty.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
