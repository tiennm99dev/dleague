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

	"github.com/tiennm99/dleague/server/internal/store"
	"github.com/tiennm99/dleague/server/internal/ws"
)

// RouterOptions holds optional cross-cutting settings for NewRouter.
type RouterOptions struct {
	// TrustedProxies enables middleware.RealIP when non-empty.
	// Each entry is an IP or CIDR string. When empty, RealIP is skipped to
	// prevent IP spoofing on direct-access deployments.
	TrustedProxies []string
}

// NewRouter constructs the top-level chi.Router.
//
// webRoot must be an existing directory containing index.html. It is resolved
// to an absolute path before being mounted; relative or non-existent paths
// return an error rather than panicking on first request.
//
// wsOpts is optional and forwarded to the WebSocket upgrade handler.
//
// st may be nil — /health then skips the DB ping and reports plain "ok".
// In production main() wires a real Store; tests pass nil to keep setup
// minimal until they explicitly cover the DB-degraded path.
func NewRouter(webRoot string, hub *ws.Hub, wsOpts ws.UpgradeOptions, st *store.Client, rOpts RouterOptions) (http.Handler, error) {
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
	// RealIP is only safe when requests arrive via a trusted proxy. Without an
	// allowlist any client could spoof X-Forwarded-For.
	if len(rOpts.TrustedProxies) > 0 {
		r.Use(middleware.RealIP)
	}
	r.Use(middleware.Recoverer)

	r.Get("/health", healthHandler(st))
	r.Get("/ws", ws.UpgradeHandler(hub, wsOpts))

	// Static file server: apply security headers scoped to this route group only.
	fs := http.FileServer(http.Dir(abs))
	r.Group(func(r chi.Router) {
		r.Use(securityHeaders)
		r.Handle("/*", fs)
	})

	return r, nil
}
