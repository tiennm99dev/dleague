// Package store is the dleague data layer over MySQL HeatWave.
//
// Store wraps a *sql.DB with conservative pool defaults sized for the OCI
// Always-Free shape (max_connections empirically observed in Phase B).
// Concrete table-touching methods live alongside this file (users.go etc.).
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql" // driver registration
)

// Pool defaults. Tuned in Phase C; revisit after Phase B confirms
// max_connections on the live HeatWave instance.
const (
	defaultMaxOpen     = 25
	defaultMaxIdle     = 5
	defaultMaxLifetime = 30 * time.Minute
)

// Store owns the database handle and exposes lifecycle + query methods.
type Store struct {
	db *sql.DB
}

// New opens a connection pool against dsn (MySQL driver DSN format) and
// pings the server to fail fast on misconfiguration.
func New(ctx context.Context, dsn string) (*Store, error) {
	if dsn == "" {
		return nil, fmt.Errorf("store: empty DSN")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	db.SetMaxOpenConns(defaultMaxOpen)
	db.SetMaxIdleConns(defaultMaxIdle)
	db.SetConnMaxLifetime(defaultMaxLifetime)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return &Store{db: db}, nil
}

// DB exposes the underlying handle for migrator use. Callers in the same
// module are trusted to use placeholders only.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Ping checks connectivity. Returns nil error if the server responds.
func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store: nil handle")
	}
	return s.db.PingContext(ctx)
}

// Close releases pooled connections. Safe to call on a nil receiver.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}
