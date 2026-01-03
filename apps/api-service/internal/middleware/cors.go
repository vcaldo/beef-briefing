package middleware

import (
	"net/http"
	"strings"
)

// CORS handles Cross-Origin Resource Sharing for Mini App endpoints
type CORS struct {
	allowedOrigins map[string]bool
	allowAll       bool
}

// NewCORS creates a new CORS middleware
func NewCORS(origins []string) *CORS {
	c := &CORS{
		allowedOrigins: make(map[string]bool),
	}

	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin == "*" {
			c.allowAll = true
		} else if origin != "" {
			c.allowedOrigins[origin] = true
		}
	}

	return c
}

// Handler wraps an http.Handler with CORS support
func (c *CORS) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Check if origin is allowed
		if c.allowAll {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if origin != "" && c.allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}

		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
