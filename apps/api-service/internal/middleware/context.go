package middleware

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	// AppNameContextKey is the context key for the authenticated app name.
	AppNameContextKey contextKey = "appName"

	// JWTContextKey is the context key for JWT claims.
	JWTContextKey contextKey = "jwtClaims"
)
