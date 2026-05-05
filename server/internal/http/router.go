// Package http wires HTTP routes. The server only does two things over HTTP
// at the transport layer: serve static assets (web/ directory) and upgrade
// /ws to a WebSocket. The /api/v1 surface lives in `internal/api` and is
// mounted here.
package http

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/tiennm99/dleague/server/internal/api"
	"github.com/tiennm99/dleague/server/internal/auth"
	"github.com/tiennm99/dleague/server/internal/store"
	"github.com/tiennm99/dleague/server/internal/ws"
)

// NewRouter constructs the top-level chi.Router.
//
// webRoot must be an existing directory. wsOpts is forwarded to the WS
// upgrade handler — set wsOpts.Verifier to enable the AUTH handshake.
//
// `s` and `verifier` may be nil (e.g. minimal Phase-1-style boot for tests
// or a degraded server). When non-nil they wire `/api/v1` and the auth
// middleware on protected routes.
func NewRouter(webRoot string, hub *ws.Hub, wsOpts ws.UpgradeOptions, s store.Store, verifier auth.Verifier) (http.Handler, error) {
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

	r.Get("/health", healthHandler(s))
	r.Get("/ws", ws.UpgradeHandler(hub, wsOpts))

	if s != nil {
		// Auth's Upserter shape is satisfied by store.Store directly.
		api.Mount(r, s, verifier, asUpserter(s))
	}

	fs := http.FileServer(http.Dir(abs))
	r.Handle("/*", fs)

	return r, nil
}

// asUpserter returns nil if s is nil so api.Mount can short-circuit upsert
// wiring even when a store is present but the operator wants to disable
// the side effect — currently never used in production but useful for tests.
func asUpserter(s store.Store) auth.Upserter {
	if s == nil {
		return nil
	}
	return s
}
