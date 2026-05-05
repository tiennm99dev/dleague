package store

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestStorePingLive opens a real connection if MYSQL_TEST_DSN is set.
// Skipped otherwise — CI without Docker stays green.
func TestStorePingLive(t *testing.T) {
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("MYSQL_TEST_DSN not set; skipping live MySQL integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s, err := New(ctx, dsn)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	if err := s.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestStoreNewRejectsEmptyDSN(t *testing.T) {
	_, err := New(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty DSN, got nil")
	}
}

func TestStoreNilSafe(t *testing.T) {
	var s *Store
	if err := s.Close(); err != nil {
		t.Fatalf("nil Close: %v", err)
	}
	if err := s.Ping(context.Background()); err == nil {
		t.Fatal("nil Ping should return an error")
	}
}

func TestSplitStatementsBasic(t *testing.T) {
	body := `-- comment line
CREATE TABLE a (id INT);

CREATE TABLE b (id INT);
`
	stmts := splitStatements(body)
	if got, want := len(stmts), 2; got != want {
		t.Fatalf("statements = %d, want %d (stmts=%q)", got, want, stmts)
	}
}
