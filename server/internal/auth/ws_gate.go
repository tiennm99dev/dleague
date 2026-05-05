package auth

import (
	"context"

	"github.com/tiennm99/dleague/server/internal/store"
)

// Gate is the verifier handle used by the WS upgrade path. Phase 6 wires the
// AUTH frame protocol; this layer just verifies the token and (optionally)
// upserts the user record. Returns claims on success so the connection can
// stash them.
type Gate struct {
	v        Verifier
	upserter Upserter
}

// NewGate is a thin constructor. upserter may be nil in tests.
func NewGate(v Verifier, upserter Upserter) *Gate {
	return &Gate{v: v, upserter: upserter}
}

// Verify checks the token and runs the user upsert side effect. Returns the
// validated claims so the caller can attach them to the connection.
func (g *Gate) Verify(ctx context.Context, idToken string) (store.AuthClaims, error) {
	if g == nil || g.v == nil {
		return store.AuthClaims{}, ErrInvalidToken
	}
	if idToken == "" {
		return store.AuthClaims{}, ErrMissingToken
	}
	claims, err := g.v.Verify(ctx, idToken)
	if err != nil {
		return store.AuthClaims{}, err
	}
	if g.upserter != nil {
		if _, err := g.upserter.UpsertUserOnFirstAuth(ctx, claims); err != nil {
			_ = err // see Middleware: storage failure is non-fatal here.
		}
	}
	return claims, nil
}
