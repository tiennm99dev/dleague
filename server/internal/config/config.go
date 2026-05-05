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

	// Couchbase 8.0 connection details. Required.
	// Conn string typically `couchbase://couchbase` (docker service name) in
	// prod, `couchbase://127.0.0.1` for local dev.
	CouchbaseConnString string
	CouchbaseUsername   string
	CouchbasePassword   string
	CouchbaseBucket     string

	// Redis 8.4 connection details. Required.
	RedisAddr     string
	RedisPassword string
}

// Required-env errors. Sentinels rather than dynamic strings so callers can
// `errors.Is` if they ever need to distinguish missing-key from malformed.
var (
	ErrMissingFirebaseCredentials = errors.New("FIREBASE_CREDENTIALS_JSON is required")
	ErrMalformedFirebaseJSON      = errors.New("FIREBASE_CREDENTIALS_JSON must be valid JSON")
	ErrMissingFirebaseProject     = errors.New("FIREBASE_PROJECT_ID is required")
	ErrMissingCouchbaseConn       = errors.New("COUCHBASE_CONN_STRING is required")
	ErrMissingCouchbaseUser       = errors.New("COUCHBASE_USERNAME is required")
	ErrMissingCouchbasePassword   = errors.New("COUCHBASE_PASSWORD is required")
	ErrMissingCouchbaseBucket     = errors.New("COUCHBASE_BUCKET is required")
	ErrMissingRedisAddr           = errors.New("REDIS_ADDR is required")
	ErrMissingRedisPassword       = errors.New("REDIS_PASSWORD is required")
)

// Load reads configuration from the process environment.
func Load() (Config, error) {
	cfg := Config{
		Addr:                    envOr("DLEAGUE_ADDR", ":8080"),
		WebRoot:                 envOr("DLEAGUE_WEB", "./web"),
		AllowedOrigins:          splitCSV(os.Getenv("DLEAGUE_WS_ORIGINS")),
		FirebaseCredentialsJSON: os.Getenv("FIREBASE_CREDENTIALS_JSON"),
		FirebaseProjectID:       os.Getenv("FIREBASE_PROJECT_ID"),
		CouchbaseConnString:     os.Getenv("COUCHBASE_CONN_STRING"),
		CouchbaseUsername:       os.Getenv("COUCHBASE_USERNAME"),
		CouchbasePassword:       os.Getenv("COUCHBASE_PASSWORD"),
		CouchbaseBucket:         os.Getenv("COUCHBASE_BUCKET"),
		RedisAddr:               os.Getenv("REDIS_ADDR"),
		RedisPassword:           os.Getenv("REDIS_PASSWORD"),
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
	// Fail fast on malformed service-account JSON — Phase 5 wants a sane
	// shape before constructing the Admin SDK client.
	if !json.Valid([]byte(cfg.FirebaseCredentialsJSON)) {
		return Config{}, ErrMalformedFirebaseJSON
	}
	if cfg.FirebaseProjectID == "" {
		return Config{}, ErrMissingFirebaseProject
	}
	if cfg.CouchbaseConnString == "" {
		return Config{}, ErrMissingCouchbaseConn
	}
	if cfg.CouchbaseUsername == "" {
		return Config{}, ErrMissingCouchbaseUser
	}
	if cfg.CouchbasePassword == "" {
		return Config{}, ErrMissingCouchbasePassword
	}
	if cfg.CouchbaseBucket == "" {
		return Config{}, ErrMissingCouchbaseBucket
	}
	if cfg.RedisAddr == "" {
		return Config{}, ErrMissingRedisAddr
	}
	if cfg.RedisPassword == "" {
		return Config{}, ErrMissingRedisPassword
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
