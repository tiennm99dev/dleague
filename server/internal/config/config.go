// Package config centralizes server runtime configuration.
//
// All values are sourced from environment variables (Coolify-injected in
// production, manual in dev). Load() returns an error if any required key is
// missing or invalid; main() should fail-fast on that error.
package config

import (
	"encoding/json"
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

	// Firebase Admin SDK service-account JSON (single-line stringified) and
	// project ID. Required.
	FirebaseCredentialsJSON string
	FirebaseProjectID       string

	// MongoDB Atlas connection. SRV URI from Atlas → Connect → Drivers → Go.
	// Required. See docs/atlas-setup.md.
	MongoURI string

	// MongoDB database name. Defaults to "dleague".
	MongoDB string
}

// Required-env errors. Sentinels rather than dynamic strings so callers can
// `errors.Is` if they ever need to distinguish missing-key from malformed.
var (
	ErrMissingFirebaseCredentials = errors.New("FIREBASE_CREDENTIALS_JSON is required")
	ErrMalformedFirebaseJSON      = errors.New("FIREBASE_CREDENTIALS_JSON must be valid JSON")
	ErrMissingFirebaseProject     = errors.New("FIREBASE_PROJECT_ID is required")
	ErrMissingMongoURI            = errors.New("MONGODB_URI is required")
)

// Load reads configuration from the process environment.
func Load() (Config, error) {
	cfg := Config{
		Addr:                    envOr("DLEAGUE_ADDR", ":8080"),
		WebRoot:                 envOr("DLEAGUE_WEB", "./web"),
		AllowedOrigins:          splitCSV(os.Getenv("DLEAGUE_WS_ORIGINS")),
		FirebaseCredentialsJSON: os.Getenv("FIREBASE_CREDENTIALS_JSON"),
		FirebaseProjectID:       os.Getenv("FIREBASE_PROJECT_ID"),
		MongoURI:                os.Getenv("MONGODB_URI"),
		MongoDB:                 envOr("MONGODB_DB", "dleague"),
	}

	if cfg.Addr == "" {
		return Config{}, fmt.Errorf("DLEAGUE_ADDR must not be empty")
	}
	if cfg.WebRoot == "" {
		return Config{}, fmt.Errorf("DLEAGUE_WEB must not be empty")
	}
	if cfg.FirebaseCredentialsJSON == "" {
		return Config{}, ErrMissingFirebaseCredentials
	}
	if !json.Valid([]byte(cfg.FirebaseCredentialsJSON)) {
		return Config{}, ErrMalformedFirebaseJSON
	}
	if cfg.FirebaseProjectID == "" {
		return Config{}, ErrMissingFirebaseProject
	}
	if cfg.MongoURI == "" {
		return Config{}, ErrMissingMongoURI
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
