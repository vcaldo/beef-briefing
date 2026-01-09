package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"beef-briefing/apps/api-service/internal/httputil"

	"github.com/golang-jwt/jwt/v5"
)

// MiniAppClaims represents the JWT claims for Mini App authentication
type MiniAppClaims struct {
	UserID    int64   `json:"user_id"`
	ChatID    *int64  `json:"chat_id,omitempty"`
	Username  *string `json:"username,omitempty"`
	FirstName string  `json:"first_name"`
	Type      string  `json:"type"`
	jwt.RegisteredClaims
}

// JWTAuth handles JWT authentication for Mini App endpoints
type JWTAuth struct {
	secretKey []byte
}

// NewJWTAuth creates a new JWT authentication middleware
func NewJWTAuth(secretKey string) *JWTAuth {
	return &JWTAuth{
		secretKey: []byte(secretKey),
	}
}

// CreateToken creates a new JWT token for a Mini App user
func (a *JWTAuth) CreateToken(userID int64, chatID *int64, username *string, firstName string) (string, error) {
	claims := MiniAppClaims{
		UserID:    userID,
		ChatID:    chatID,
		Username:  username,
		FirstName: firstName,
		Type:      "mini_app",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.secretKey)
}

// ValidateToken validates a JWT token and returns the claims
func (a *JWTAuth) ValidateToken(tokenString string) (*MiniAppClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &MiniAppClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return a.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*MiniAppClaims); ok && token.Valid {
		if claims.Type != "mini_app" {
			return nil, jwt.ErrTokenInvalidClaims
		}
		return claims, nil
	}

	return nil, jwt.ErrTokenInvalidClaims
}

// Authenticate middleware validates JWT tokens for Mini App endpoints
func (a *JWTAuth) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")

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

		claims, err := a.ValidateToken(parts[1])
		if err != nil {
			slog.Warn("invalid JWT token", "path", r.URL.Path, "error", err)
			httputil.RespondError(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Store claims in context
		ctx := context.WithValue(r.Context(), JWTContextKey, claims)
		slog.Debug("JWT authenticated", "user_id", claims.UserID, "path", r.URL.Path)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetClaimsFromContext retrieves JWT claims from the request context
func GetClaimsFromContext(ctx context.Context) *MiniAppClaims {
	if claims, ok := ctx.Value(JWTContextKey).(*MiniAppClaims); ok {
		return claims
	}
	return nil
}
