package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/tiennm99/dleague/server/internal/store"
)

// ctxKey is private so callers must use ClaimsFromContext rather than poking
// the value out by key.
type ctxKey struct{}

// ClaimsFromContext returns the claims attached by the middleware, or zero
// if the request was unauthenticated.
func ClaimsFromContext(ctx context.Context) (store.AuthClaims, bool) {
	c, ok := ctx.Value(ctxKey{}).(store.AuthClaims)
	return c, ok
}

// Upserter is the side-effect surface — middleware calls
// `UpsertUserOnFirstAuth` after a successful verify so the local user record
// (and beta-tester ledger) stays in sync. `store.Store` satisfies it.
type Upserter interface {
	UpsertUserOnFirstAuth(ctx context.Context, claims store.AuthClaims) (store.User, error)
}

// Middleware verifies Bearer tokens and attaches claims to the request
// context. On verify failure it returns 401 with a WWW-Authenticate hint.
//
// upserter may be nil (handy in tests that don't care about the side effect).
func Middleware(v Verifier, upserter Upserter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok, err := bearerFromHeader(r.Header.Get("Authorization"))
			if err != nil {
				unauthorized(w, "invalid_request")
				return
			}
			claims, err := v.Verify(r.Context(), tok)
			if err != nil {
				unauthorized(w, "invalid_token")
				return
			}
			if upserter != nil {
				if _, err := upserter.UpsertUserOnFirstAuth(r.Context(), claims); err != nil {
					// Storage failure shouldn't block authenticated traffic
					// — the next request will retry. Log path lives at the
					// router layer once a logger is wired in.
					_ = err
				}
			}
			ctx := context.WithValue(r.Context(), ctxKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bearerFromHeader(h string) (string, error) {
	if h == "" {
		return "", ErrMissingToken
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return "", ErrInvalidToken
	}
	tok := strings.TrimSpace(h[len(prefix):])
	if tok == "" {
		return "", ErrMissingToken
	}
	return tok, nil
}

func unauthorized(w http.ResponseWriter, reason string) {
	w.Header().Set("WWW-Authenticate", `Bearer error="`+reason+`"`)
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte("unauthorized"))
}
