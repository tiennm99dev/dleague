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
	"strconv"
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

	// MongoURI is the MongoDB connection string. Required.
	// Local:  mongodb://user:pass@localhost:27017/?authSource=admin
	// Atlas:  mongodb+srv://user:pass@cluster.mongodb.net/?retryWrites=true&w=majority
	MongoURI string

	// MaxConns is the maximum number of concurrent WebSocket connections.
	// Sourced from DLEAGUE_MAX_CONNS. Defaults to 1000.
	MaxConns int

	// TrustedProxies is an optional list of trusted proxy IP/CIDR strings.
	// When non-empty, middleware.RealIP is registered to honour X-Forwarded-For.
	// When empty, RealIP is skipped to prevent IP spoofing on direct-access deploys.
	// Sourced from DLEAGUE_TRUSTED_PROXIES (comma-separated).
	TrustedProxies []string

	// Env identifies the runtime environment ("development", "production", etc.).
	// Sourced from DLEAGUE_ENV. Defaults to "development". Use IsProduction()
	// for hardening checks rather than string comparisons.
	Env string
}

// IsProduction reports whether Env names a production environment.
// Accepts "production" and "prod" case-insensitively so misconfigured envs
// (e.g. "PROD", "Production") still trigger production hardening.
func (c Config) IsProduction() bool {
	switch strings.ToLower(strings.TrimSpace(c.Env)) {
	case "production", "prod":
		return true
	}
	return false
}

// ErrMissingMongoURI signals that MONGO_URI is unset or empty.
var ErrMissingMongoURI = errors.New("MONGO_URI is required")

// Load reads configuration from the process environment.
func Load() (Config, error) {
	maxConns, err := parseIntOr("DLEAGUE_MAX_CONNS", 1000)
	if err != nil {
		return Config{}, fmt.Errorf("DLEAGUE_MAX_CONNS: %w", err)
	}

	cfg := Config{
		Addr:           envOr("DLEAGUE_ADDR", ":8080"),
		WebRoot:        envOr("DLEAGUE_WEB", "./web"),
		AllowedOrigins: splitCSV(os.Getenv("DLEAGUE_WS_ORIGINS")),
		MongoURI:       os.Getenv("MONGO_URI"),
		MaxConns:       maxConns,
		TrustedProxies: splitCSV(os.Getenv("DLEAGUE_TRUSTED_PROXIES")),
		Env:            envOr("DLEAGUE_ENV", "development"),
	}

	if cfg.MongoURI == "" {
		return Config{}, ErrMissingMongoURI
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

func parseIntOr(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("must be an integer, got %q", v)
	}
	if n <= 0 {
		return 0, fmt.Errorf("must be positive, got %d", n)
	}
	return n, nil
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
