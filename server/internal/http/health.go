package http

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/tiennm99/dleague/server/internal/store"
)

const healthPingTimeout = 2 * time.Second

// healthHandler returns a handler that reports liveness plus DB reachability.
//
// st may be nil (e.g. in tests where no DB is wired up); the handler then
// degrades to a plain "ok" without a DB check.
func healthHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		if st == nil {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), healthPingTimeout)
		defer cancel()

		if err := st.Ping(ctx); err != nil {
			log.Printf("health: db ping: %v", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}
