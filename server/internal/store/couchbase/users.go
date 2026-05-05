package couchbase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/couchbase/gocb/v2"

	"github.com/tiennm99/dleague/server/internal/store"
)

// UpsertUserOnFirstAuth stamps beta-tester provenance only on first write.
//
// Strategy: try `Insert` with the full beta-stamped doc. On a 'doc exists'
// error, fall through to a CAS-based read-modify-write that refreshes
// mutable fields (Email/DisplayName/Provider/LastSeen) without touching
// IsBetaTester / BetaSignupAt / CreatedAt.
func (c *Client) UpsertUserOnFirstAuth(ctx context.Context, claims store.AuthClaims) (store.User, error) {
	if c == nil || c.users == nil {
		return store.User{}, store.ErrClosed
	}
	now := time.Now().UTC()
	fresh := store.User{
		UID:          claims.UID,
		Email:        claims.Email,
		DisplayName:  claims.DisplayName,
		Provider:     claims.Provider,
		IsBetaTester: true,
		BetaSignupAt: now,
		CreatedAt:    now,
		LastSeen:     now,
	}

	_, err := c.users.Insert(claims.UID, fresh, &gocb.InsertOptions{
		Context: ctx,
		Timeout: defaultOpTimeout,
	})
	if err == nil {
		return fresh, nil
	}
	if !errors.Is(err, gocb.ErrDocumentExists) {
		return store.User{}, fmt.Errorf("couchbase: user insert: %w", err)
	}

	// Existing user — CAS-based update of mutable fields only.
	for attempt := 0; attempt < 3; attempt++ {
		var existing store.User
		res, err := c.users.Get(claims.UID, &gocb.GetOptions{Context: ctx, Timeout: defaultOpTimeout})
		if err != nil {
			return store.User{}, fmt.Errorf("couchbase: user get: %w", err)
		}
		if err := res.Content(&existing); err != nil {
			return store.User{}, fmt.Errorf("couchbase: user decode: %w", err)
		}
		existing.Email = claims.Email
		existing.DisplayName = claims.DisplayName
		existing.Provider = claims.Provider
		existing.LastSeen = now

		_, err = c.users.Replace(claims.UID, existing, &gocb.ReplaceOptions{
			Cas:     res.Cas(),
			Context: ctx,
			Timeout: defaultOpTimeout,
		})
		if err == nil {
			return existing, nil
		}
		if !errors.Is(err, gocb.ErrCasMismatch) {
			return store.User{}, fmt.Errorf("couchbase: user replace: %w", err)
		}
	}
	return store.User{}, fmt.Errorf("couchbase: user replace: cas mismatch after retries")
}

// GetUser returns ErrNotFound when the doc is absent.
func (c *Client) GetUser(ctx context.Context, uid string) (store.User, error) {
	if c == nil || c.users == nil {
		return store.User{}, store.ErrClosed
	}
	res, err := c.users.Get(uid, &gocb.GetOptions{Context: ctx, Timeout: defaultOpTimeout})
	if err != nil {
		if errors.Is(err, gocb.ErrDocumentNotFound) {
			return store.User{}, store.ErrNotFound
		}
		return store.User{}, fmt.Errorf("couchbase: user get: %w", err)
	}
	var u store.User
	if err := res.Content(&u); err != nil {
		return store.User{}, fmt.Errorf("couchbase: user decode: %w", err)
	}
	return u, nil
}

// TouchLastSeen updates only the LastSeen field via subdocument mutation —
// avoids re-marshaling the whole document.
func (c *Client) TouchLastSeen(ctx context.Context, uid string, at time.Time) error {
	if c == nil || c.users == nil {
		return store.ErrClosed
	}
	_, err := c.users.MutateIn(uid,
		[]gocb.MutateInSpec{
			gocb.UpsertSpec("lastSeen", at.UTC(), nil),
		},
		&gocb.MutateInOptions{Context: ctx, Timeout: defaultOpTimeout},
	)
	if err != nil {
		if errors.Is(err, gocb.ErrDocumentNotFound) {
			return store.ErrNotFound
		}
		return fmt.Errorf("couchbase: touch last seen: %w", err)
	}
	return nil
}
