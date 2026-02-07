package middleware

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestCreateToken_ContainsStandardClaims(t *testing.T) {
	auth := NewJWTAuth("test-secret-key")

	chatID := int64(-1001234567890)
	username := "testuser"
	tokenString, err := auth.CreateToken(12345, &chatID, &username, "TestUser")
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	// Parse without validation to inspect claims
	claims := &MiniAppClaims{}
	_, err = jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte("test-secret-key"), nil
	})
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}

	if claims.Issuer != "beef-briefing-api" {
		t.Errorf("expected issuer 'beef-briefing-api', got '%s'", claims.Issuer)
	}

	audiences, err := claims.GetAudience()
	if err != nil {
		t.Fatalf("failed to get audience: %v", err)
	}
	if len(audiences) != 1 || audiences[0] != "beef-briefing-mini-app" {
		t.Errorf("expected audience ['beef-briefing-mini-app'], got %v", audiences)
	}

	if claims.ID == "" {
		t.Error("expected non-empty jti claim")
	}
}

func TestValidateToken_RejectsWrongIssuer(t *testing.T) {
	auth := NewJWTAuth("test-secret-key")

	// Forge a token with wrong issuer
	claims := MiniAppClaims{
		UserID:    12345,
		FirstName: "TestUser",
		Type:      "mini_app",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "wrong-issuer",
			Audience:  jwt.ClaimStrings{"beef-briefing-mini-app"},
			ID:        "some-jti",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte("test-secret-key"))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	_, err = auth.ValidateToken(tokenString)
	if err == nil {
		t.Fatal("expected error for wrong issuer, got nil")
	}
}

func TestValidateToken_RejectsWrongAudience(t *testing.T) {
	auth := NewJWTAuth("test-secret-key")

	// Forge a token with wrong audience
	claims := MiniAppClaims{
		UserID:    12345,
		FirstName: "TestUser",
		Type:      "mini_app",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "beef-briefing-api",
			Audience:  jwt.ClaimStrings{"wrong-audience"},
			ID:        "some-jti",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte("test-secret-key"))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	_, err = auth.ValidateToken(tokenString)
	if err == nil {
		t.Fatal("expected error for wrong audience, got nil")
	}
}

func TestValidateToken_RejectsMissingIssuer(t *testing.T) {
	auth := NewJWTAuth("test-secret-key")

	// Forge a token without issuer (old-format token)
	claims := MiniAppClaims{
		UserID:    12345,
		FirstName: "TestUser",
		Type:      "mini_app",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte("test-secret-key"))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	_, err = auth.ValidateToken(tokenString)
	if err == nil {
		t.Fatal("expected error for missing issuer, got nil")
	}
}

func TestCreateToken_UniqueJTI(t *testing.T) {
	auth := NewJWTAuth("test-secret-key")

	token1, err := auth.CreateToken(12345, nil, nil, "User1")
	if err != nil {
		t.Fatalf("failed to create token 1: %v", err)
	}

	token2, err := auth.CreateToken(12345, nil, nil, "User1")
	if err != nil {
		t.Fatalf("failed to create token 2: %v", err)
	}

	// Parse both tokens to extract jti
	claims1 := &MiniAppClaims{}
	jwt.ParseWithClaims(token1, claims1, func(token *jwt.Token) (interface{}, error) {
		return []byte("test-secret-key"), nil
	})

	claims2 := &MiniAppClaims{}
	jwt.ParseWithClaims(token2, claims2, func(token *jwt.Token) (interface{}, error) {
		return []byte("test-secret-key"), nil
	})

	if claims1.ID == "" || claims2.ID == "" {
		t.Fatal("expected non-empty jti claims")
	}

	if claims1.ID == claims2.ID {
		t.Errorf("expected unique jti values, got same value: %s", claims1.ID)
	}
}
