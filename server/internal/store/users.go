package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotImplemented marks Phase C stubs that Phase 3 of the parent plan
// (260505-0947-dleague-pvp-game/phase-03-backend-auth.md) will fill in.
var ErrNotImplemented = errors.New("store: not implemented")

// User mirrors the row layout in the users table. UUIDv7 PK is generated
// by the app, stored as BINARY(16). Email comparisons are case-insensitive
// via a functional unique index on LOWER(email).
type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	DisplayName  string
	CreatedAt    time.Time
}

// CreateUser inserts a new user. Caller is responsible for hashing the
// password before passing it in. Returns ErrNotImplemented until Phase 3.
func (s *Store) CreateUser(ctx context.Context, u User) error {
	_ = ctx
	_ = u
	return ErrNotImplemented
}

// GetUserByEmailLower fetches a user by case-insensitive email match.
// Returns ErrNotImplemented until Phase 3.
func (s *Store) GetUserByEmailLower(ctx context.Context, email string) (User, error) {
	_ = ctx
	_ = email
	return User{}, ErrNotImplemented
}
