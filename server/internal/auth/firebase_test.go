package auth

import (
	"context"
	"errors"
	"os"
	"testing"
)

// TestNewVerifier_MissingProjectID ensures New returns ErrMissingProjectID
// when no project ID is provided and the emulator is not active.
func TestNewVerifier_MissingProjectID(t *testing.T) {
	// Guarantee the emulator env var is absent for this test.
	orig := os.Getenv("FIREBASE_AUTH_EMULATOR_HOST")
	if err := os.Unsetenv("FIREBASE_AUTH_EMULATOR_HOST"); err != nil {
		t.Fatalf("unsetenv: %v", err)
	}
	defer func() {
		if orig != "" {
			if err := os.Setenv("FIREBASE_AUTH_EMULATOR_HOST", orig); err != nil {
				t.Logf("restore env: %v", err)
			}
		}
	}()

	_, err := New(context.Background(), "", "")
	if err == nil {
		t.Fatal("expected ErrMissingProjectID, got nil")
	}
	if !errors.Is(err, ErrMissingProjectID) {
		t.Fatalf("expected ErrMissingProjectID, got: %v", err)
	}
}

// TestNewVerifier_Emulator verifies that New succeeds when the emulator host is
// set (no credentials required). This test is skipped if
// FIREBASE_AUTH_EMULATOR_HOST is not set in the environment, since the emulator
// binary must actually be running for the SDK to initialise the auth client.
func TestNewVerifier_Emulator(t *testing.T) {
	if os.Getenv("FIREBASE_AUTH_EMULATOR_HOST") == "" {
		t.Skip("FIREBASE_AUTH_EMULATOR_HOST not set — skipping emulator test")
	}

	ctx := context.Background()
	v, err := New(ctx, "dleague-dev", "")
	if err != nil {
		t.Fatalf("New with emulator: %v", err)
	}
	if v == nil {
		t.Fatal("New returned nil verifier")
	}
}

// TestVerifyIDToken_InvalidToken verifies that VerifyIDToken rejects a
// syntactically invalid (non-JWT) token string. Emulator must be running.
func TestVerifyIDToken_InvalidToken(t *testing.T) {
	if os.Getenv("FIREBASE_AUTH_EMULATOR_HOST") == "" {
		t.Skip("FIREBASE_AUTH_EMULATOR_HOST not set — skipping emulator test")
	}

	ctx := context.Background()
	v, err := New(ctx, "dleague-dev", "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = v.VerifyIDToken(ctx, "not.a.valid.jwt")
	if err == nil {
		t.Fatal("expected error for invalid token, got nil")
	}
}
