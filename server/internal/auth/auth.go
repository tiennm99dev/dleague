// Package auth verifies Firebase ID tokens and upserts user records on first
// auth. The HTTP middleware and WS gate both consume the same Verifier
// interface — the concrete Firebase impl is one swap away from a stub for
// tests or a different IdP later.
package auth

import (
	"context"
	"errors"

	"github.com/tiennm99/dleague/server/internal/store"
)

// Verifier exchanges a JWT for AuthClaims. The seam: production uses the
// Firebase impl, tests use a stub.
type Verifier interface {
	Verify(ctx context.Context, idToken string) (store.AuthClaims, error)
}

// Errors surfaced by the middleware and WS gate.
var (
	ErrMissingToken = errors.New("auth: missing bearer token")
	ErrInvalidToken = errors.New("auth: invalid token")
)
