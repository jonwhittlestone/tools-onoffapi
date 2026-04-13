package handlers

import (
	"net/http"
)

// RequireAPIKey is middleware that checks every request (except /health) carries
// a valid X-API-Key header. Returns 401 if missing or wrong.
//
// In FastAPI this would be: Depends(api_key_header)
// In Go, middleware wraps an http.Handler and returns a new http.Handler.
func RequireAPIKey(apiKey string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// /health is public — skip auth
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		key := r.Header.Get("X-API-Key")
		if key == "" || key != apiKey {
			writeError(w, http.StatusUnauthorized, "missing or invalid API key")
			return
		}

		next.ServeHTTP(w, r)
	})
}
