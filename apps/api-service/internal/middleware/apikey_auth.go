package middleware

import (
	"log/slog"
	"net/http"
	"strings"
)

// APIKeyAuth validates API key from Authorization header
type APIKeyAuth struct {
	apiKey string
}

// NewAPIKeyAuth creates a new API key authentication middleware
func NewAPIKeyAuth(apiKey string) *APIKeyAuth {
	return &APIKeyAuth{
		apiKey: apiKey,
	}
}

// Authenticate middleware checks for valid API key
func (a *APIKeyAuth) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")

		// Expected format: "Bearer <api-key>"
		if authHeader == "" {
			slog.Warn("missing authorization header", "path", r.URL.Path, "ip", r.RemoteAddr)
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error": "missing authorization header"}`, http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			slog.Warn("invalid authorization header format", "path", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error": "invalid authorization header format"}`, http.StatusUnauthorized)
			return
		}

		if parts[1] != a.apiKey {
			slog.Warn("invalid API key", "path", r.URL.Path, "ip", r.RemoteAddr)
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error": "invalid API key"}`, http.StatusUnauthorized)
			return
		}

		// API key is valid, proceed
		next.ServeHTTP(w, r)
	})
}
