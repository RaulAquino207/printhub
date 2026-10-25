package httpapi

import (
	"net/http"
	"strings"
)

// Methods and request headers that browsers may use in cross-origin requests.
// Keep them limited to what the API actually serves (ADR 0003).
var (
	corsAllowedMethods = strings.Join([]string{http.MethodGet, http.MethodPost}, ", ")
	corsAllowedHeaders = strings.Join([]string{"Content-Type", "X-Requested-With"}, ", ")
)

// allowOrigins lets browsers on the given origins make credentialed
// cross-origin requests. Requests from other origins get no CORS headers, so
// the browser refuses to expose the response to the calling page.
//
// CORS is a browser policy, not authorization: it does not stop a request from
// reaching the API, only what a web page from another origin may read.
func allowOrigins(origins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(origins))
	for _, o := range origins {
		allowed[o] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			// The response depends on the Origin header, so caches must not
			// reuse it for a request with a different origin.
			h.Add("Vary", "Origin")

			origin := r.Header.Get("Origin")
			if !allowed[origin] {
				next.ServeHTTP(w, r)
				return
			}

			// Credentialed requests require the exact origin, never "*".
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")

			if isPreflight(r) {
				h.Add("Vary", "Access-Control-Request-Method")
				h.Add("Vary", "Access-Control-Request-Headers")
				h.Set("Access-Control-Allow-Methods", corsAllowedMethods)
				h.Set("Access-Control-Allow-Headers", corsAllowedHeaders)
				h.Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isPreflight reports whether r is a CORS preflight: an OPTIONS request the
// browser sends before the real one to ask which methods and headers are allowed.
func isPreflight(r *http.Request) bool {
	return r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
}
