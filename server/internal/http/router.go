// Package http wires HTTP routes. The server only does two things over HTTP:
// serve static assets (web/ directory + WASM bundle) and upgrade /ws to a
// WebSocket. All gameplay messages travel over /ws as binary protobuf.
package http

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/tiennm99/dleague/server/internal/ws"
)

// NewRouter constructs the top-level chi.Router.
//
// webRoot must be an existing directory containing index.html. It is resolved
// to an absolute path before being mounted; relative or non-existent paths
// return an error rather than panicking on first request.
//
// wsOpts is optional and forwarded to the WebSocket upgrade handler.
//
// Note: the storage layer is intentionally absent from this signature during
// the pivot. Phase 3 reintroduces a `store.Store` interface dependency once
// Couchbase + Redis are wired.
func NewRouter(webRoot string, hub *ws.Hub, wsOpts ws.UpgradeOptions) (http.Handler, error) {
	abs, err := filepath.Abs(webRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve webRoot: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("webRoot %q: %w", abs, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("webRoot %q is not a directory", abs)
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/health", healthHandler())
	r.Get("/ws", ws.UpgradeHandler(hub, wsOpts))

	fs := http.FileServer(http.Dir(abs))
	r.Handle("/*", fs)

	return r, nil
}
