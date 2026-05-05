package couchbase_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/tiennm99/dleague/server/internal/store"
	"github.com/tiennm99/dleague/server/internal/store/couchbase"
)

// Live integration test — only runs when COUCHBASE_TEST_CONN is set.
// Set the four COUCHBASE_TEST_* envs to point at a disposable bucket.
//
// Example:
//   COUCHBASE_TEST_CONN=couchbase://127.0.0.1 \
//   COUCHBASE_TEST_USER=Administrator \
//   COUCHBASE_TEST_PASS=password \
//   COUCHBASE_TEST_BUCKET=dleague \
//   go test ./server/internal/store/couchbase/...
func TestCouchbaseRoundTrip(t *testing.T) {
	conn := os.Getenv("COUCHBASE_TEST_CONN")
	if conn == "" {
		t.Skip("COUCHBASE_TEST_CONN unset; skipping live integration")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := couchbase.New(ctx, couchbase.Config{
		ConnString: conn,
		Username:   os.Getenv("COUCHBASE_TEST_USER"),
		Password:   os.Getenv("COUCHBASE_TEST_PASS"),
		Bucket:     os.Getenv("COUCHBASE_TEST_BUCKET"),
	})
	if err != nil {
		t.Fatalf("couchbase.New: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	if err := c.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	uid := "test-uid-" + time.Now().UTC().Format("20060102150405")
	u, err := c.UpsertUserOnFirstAuth(ctx, store.AuthClaims{UID: uid, Email: "x@y", Provider: "password"})
	if err != nil {
		t.Fatalf("UpsertUserOnFirstAuth: %v", err)
	}
	if !u.IsBetaTester || u.BetaSignupAt.IsZero() {
		t.Fatalf("beta fields not stamped: %+v", u)
	}
	first := u.BetaSignupAt

	// Re-auth must not move BetaSignupAt.
	u2, err := c.UpsertUserOnFirstAuth(ctx, store.AuthClaims{UID: uid, Email: "x2@y", Provider: "google.com"})
	if err != nil {
		t.Fatalf("re-auth upsert: %v", err)
	}
	if !u2.BetaSignupAt.Equal(first) {
		t.Errorf("BetaSignupAt drifted: was %v, now %v", first, u2.BetaSignupAt)
	}

	got, err := c.GetUser(ctx, uid)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.Email != "x2@y" {
		t.Errorf("Email refresh failed: %q", got.Email)
	}

	if _, err := c.GetUser(ctx, "definitely-missing-"+uid); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
