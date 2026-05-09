package http

import "net/http"

// securityHeaders returns a chi middleware that adds standard security headers
// to static-asset responses. Applied only to the "/*" FileServer route, NOT to
// /ws or /health, so the headers don't interfere with protocol upgrades.
//
// CSP note: no eval or unsafe directives needed — the client is pure SvelteKit +
// Phaser JS with no runtime code evaluation.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self'")
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}
