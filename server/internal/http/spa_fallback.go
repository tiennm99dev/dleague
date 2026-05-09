package http

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// staticAssetExts is the set of file extensions that must 404 cleanly when
// missing rather than falling back to index.html.  These are all
// non-navigable assets; serving index.html for a missing .js file would
// break the app rather than help it.
var staticAssetExts = map[string]bool{
	".js":    true,
	".css":   true,
	".svg":   true,
	".png":   true,
	".jpg":   true,
	".jpeg":  true,
	".ico":   true,
	".wasm":  true,
	".json":  true,
	".woff":  true,
	".woff2": true,
	".ttf":   true,
	".map":   true,
}

// spaFallback wraps a standard http.FileServer and serves webRoot/index.html
// for any GET request that:
//   - is not a WebSocket upgrade path (/ws)
//   - is not the health endpoint (/health)
//   - does not resolve to an existing file in webRoot
//   - does not have a static-asset extension (those 404 cleanly)
//
// This enables SvelteKit client-side routing: navigating directly to
// /match/<token> or /play returns index.html so the JS router takes over.
func spaFallback(webRoot string, fs http.Handler) http.Handler {
	indexPath := filepath.Join(webRoot, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only intercept GETs; let everything else pass to the FileServer.
		if r.Method != http.MethodGet {
			fs.ServeHTTP(w, r)
			return
		}

		path := r.URL.Path

		// Never intercept API/WS paths.
		if path == "/ws" || path == "/health" {
			fs.ServeHTTP(w, r)
			return
		}

		// Static asset extensions must 404 cleanly on miss.
		ext := strings.ToLower(filepath.Ext(path))
		if ext != "" && staticAssetExts[ext] {
			fs.ServeHTTP(w, r)
			return
		}

		// Check whether the exact file exists in webRoot.
		fullPath := filepath.Join(webRoot, filepath.FromSlash(path))
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			// File exists — serve it normally.
			fs.ServeHTTP(w, r)
			return
		}

		// Attempt to serve index.html as SPA entry point.
		// If index.html itself is missing (empty webRoot in tests), fall through
		// to the FileServer which will return a clean 404.
		if _, err := os.Stat(indexPath); err != nil {
			fs.ServeHTTP(w, r)
			return
		}

		http.ServeFile(w, r, indexPath)
	})
}
