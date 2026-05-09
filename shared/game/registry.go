// Reserved scaffold; not currently used. See phase-04-pluggability-decision.md.
//
// Pattern adapted from ratel-online/server/state/state.go (MIT License).
// See NOTICE for the full attribution.

package game

import (
	"fmt"
	"sync"
)

// Factory constructs a fresh Game instance. Implementations must be cheap —
// callers may invoke a Factory per match.
type Factory func() Game

var (
	registryMu sync.RWMutex
	registry   = map[string]Factory{}
)

// Register associates a stable game ID (e.g. "wordle") with a Factory.
// Calling Register twice with the same id panics — registration is intended
// to happen once per binary, in package init.
func Register(id string, f Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := registry[id]; dup {
		panic(fmt.Sprintf("game.Register: duplicate id %q", id))
	}
	registry[id] = f
}

// New constructs a Game by id. Returns an error if no factory is registered.
func New(id string) (Game, error) {
	registryMu.RLock()
	f, ok := registry[id]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("game.New: unknown id %q", id)
	}
	return f(), nil
}

// IDs returns all registered game IDs (unordered). Useful for diagnostics.
func IDs() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, 0, len(registry))
	for id := range registry {
		out = append(out, id)
	}
	return out
}
