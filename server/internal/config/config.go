// Package config centralizes server runtime configuration.
//
// All values are sourced from environment variables (Coolify-injected in
// production, manual in dev). Load() returns an error if any required key is
// missing or invalid; main() should fail-fast on that error.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Config holds resolved runtime settings.
type Config struct {
	// Addr is the HTTP listen address (e.g. ":8080"). Defaults to ":8080".
	Addr string

	// WebRoot is the directory served as static assets. Defaults to "./web".
	WebRoot string

	// AllowedOrigins limits Origin headers accepted on the WebSocket upgrade.
	// Empty slice means same-origin only.
	AllowedOrigins []string

	// DatabaseURL is the MySQL driver DSN. Required.
	// Format: user:pass@tcp(host:port)/dbname?tls=true&parseTime=true&loc=UTC
	DatabaseURL string
}

// ErrMissingDatabaseURL signals that DATABASE_URL is unset or empty.
var ErrMissingDatabaseURL = errors.New("DATABASE_URL is required")

// Load reads configuration from the process environment.
func Load() (Config, error) {
	cfg := Config{
		Addr:           envOr("DLEAGUE_ADDR", ":8080"),
		WebRoot:        envOr("DLEAGUE_WEB", "./web"),
		AllowedOrigins: splitCSV(os.Getenv("DLEAGUE_WS_ORIGINS")),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, ErrMissingDatabaseURL
	}
	if cfg.Addr == "" {
		return Config{}, fmt.Errorf("DLEAGUE_ADDR must not be empty")
	}
	if cfg.WebRoot == "" {
		return Config{}, fmt.Errorf("DLEAGUE_WEB must not be empty")
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
