package http

import "net/http"

// healthHandler reports plain liveness. Storage-dependent health checks come
// back in Phase 3 once the `store.Store` interface lands; for now the server
// is considered healthy if the goroutine can serve.
func healthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}
