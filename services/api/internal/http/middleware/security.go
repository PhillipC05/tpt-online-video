package middleware

import (
	"net/http"
	"strings"
)

// CORSMiddleware returns a middleware that sets CORS headers using an explicit
// origin allowlist. Wildcard + credentials is intentionally avoided because it
// allows any site to issue credentialed cross-origin requests.
//
// CSRF note: all state-changing endpoints require an Authorization: Bearer
// token in the request header, which browsers cannot set in cross-origin
// form/navigation requests, so bearer-token authentication is itself CSRF-safe.
// The optional cookie fallback path (access_token cookie) must use SameSite=Strict.
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	originSet := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originSet[strings.TrimRight(o, "/")] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if _, ok := originSet[strings.TrimRight(origin, "/")]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Credentials", "true")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
					w.Header().Set("Vary", "Origin")
				}
				// Unknown origins get no CORS headers — browser will block them.
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders adds standard defensive HTTP response headers.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// Permissive CSP for a JSON API; tighten if the API ever serves HTML.
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		next.ServeHTTP(w, r)
	})
}
