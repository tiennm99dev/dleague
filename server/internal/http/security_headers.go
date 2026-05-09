package http

import "net/http"

// securityHeaders returns a chi middleware that adds standard security headers
// to static-asset responses. Applied only to the "/*" FileServer route, NOT to
// /ws or /health, so the headers don't interfere with protocol upgrades.
//
// CSP note: wasm-unsafe-eval is present for the WASM runtime. Remove it in
// Phase 06 once the WASM bundle is replaced by native JS.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'wasm-unsafe-eval'")
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}
