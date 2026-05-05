package http

import (
	"context"
	"net/http"
	"time"

	"github.com/tiennm99/dleague/server/internal/store"
)

const healthPingTimeout = 2 * time.Second

// healthHandler reports liveness plus storage reachability. Nil store →
// plain "ok" (the Phase-2 fallback).
func healthHandler(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if s == nil {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), healthPingTimeout)
		defer cancel()
		if err := s.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("degraded: store unreachable"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}
