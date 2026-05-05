package store

import "errors"

// Sentinel errors. Concrete impls wrap these so callers can `errors.Is`
// without knowing which backend produced the failure.
var (
	// ErrNotFound — document/key not present.
	ErrNotFound = errors.New("store: not found")
	// ErrClosed — Close() already called on this store.
	ErrClosed = errors.New("store: closed")
)
