package middleware

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"

	"beef-briefing/apps/api-service/internal/httputil"
)

// MultiAPIKeyAuth validates API keys from multiple authorized applications
type MultiAPIKeyAuth struct {
	appKeys map[string]string // appName -> apiKey
}

// NewMultiAPIKeyAuth creates a new multi-key API authentication middleware
func NewMultiAPIKeyAuth(appKeys map[string]string) *MultiAPIKeyAuth {
	return &MultiAPIKeyAuth{
		appKeys: appKeys,
	}
}

// Authenticate middleware checks for valid API key from any authorized app
func (a *MultiAPIKeyAuth) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")

		// Expected format: "Bearer <api-key>"
		if authHeader == "" {
			slog.Warn("missing authorization header", "path", r.URL.Path, "ip", r.RemoteAddr)
			httputil.RespondError(w, "missing authorization header", http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			slog.Warn("invalid authorization header format", "path", r.URL.Path)
			httputil.RespondError(w, "invalid authorization header format", http.StatusUnauthorized)
			return
		}

		providedKey := parts[1]

		// Check against all registered app keys using constant-time comparison
		var authenticatedApp string
		for appName, appKey := range a.appKeys {
			if subtle.ConstantTimeCompare([]byte(providedKey), []byte(appKey)) == 1 {
				authenticatedApp = appName
				break
			}
		}

		if authenticatedApp == "" {
			slog.Warn("invalid API key", "path", r.URL.Path, "ip", r.RemoteAddr)
			httputil.RespondError(w, "invalid api key", http.StatusUnauthorized)
			return
		}

		// Store the authenticated app name in context for logging/auditing
		ctx := context.WithValue(r.Context(), AppNameContextKey, authenticatedApp)
		slog.Debug("request authenticated", "app", authenticatedApp, "path", r.URL.Path)

		// API key is valid, proceed with the authenticated app in context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetAppNameFromContext retrieves the authenticated app name from the request context
func GetAppNameFromContext(ctx context.Context) string {
	if appName, ok := ctx.Value(AppNameContextKey).(string); ok {
		return appName
	}
	return ""
}
