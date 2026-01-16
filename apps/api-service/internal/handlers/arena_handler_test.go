package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"beef-briefing/apps/api-service/internal/middleware"
	"beef-briefing/apps/api-service/internal/services"
	"beef-briefing/apps/api-service/internal/testutil"
	"beef-briefing/pkg/config"

	"github.com/gorilla/mux"
)

// =============================================================================
// Auth Middleware Tests
// =============================================================================

func TestAuthMiddleware_ValidAPIKey(t *testing.T) {
	// Setup multi-key auth middleware with a known test key
	appKeys := map[string]string{
		"test-app": "valid-test-api-key-12345",
	}
	auth := middleware.NewMultiAPIKeyAuth(appKeys)

	// Create a simple handler that returns 200 OK
	handler := auth.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the app name was set in context
		appName := middleware.GetAppNameFromContext(r.Context())
		if appName != "test-app" {
			t.Errorf("expected app name 'test-app', got '%s'", appName)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))

	// Create request with valid API key
	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer valid-test-api-key-12345")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	appKeys := map[string]string{
		"test-app": "valid-test-api-key-12345",
	}
	auth := middleware.NewMultiAPIKeyAuth(appKeys)

	handler := auth.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called when auth header is missing")
	}))

	// Create request WITHOUT Authorization header
	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	// Verify error message
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "missing authorization header" {
		t.Errorf("expected error message 'missing authorization header', got '%s'", resp["error"])
	}
}

func TestAuthMiddleware_InvalidKey(t *testing.T) {
	appKeys := map[string]string{
		"test-app": "valid-test-api-key-12345",
	}
	auth := middleware.NewMultiAPIKeyAuth(appKeys)

	handler := auth.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called when API key is invalid")
	}))

	// Create request with INVALID API key
	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer wrong-key-totally-invalid")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	// Verify error message
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "invalid api key" {
		t.Errorf("expected error message 'invalid api key', got '%s'", resp["error"])
	}
}

func TestAuthMiddleware_InvalidFormat(t *testing.T) {
	appKeys := map[string]string{
		"test-app": "valid-test-api-key-12345",
	}
	auth := middleware.NewMultiAPIKeyAuth(appKeys)

	handler := auth.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called when auth format is invalid")
	}))

	// Create request with invalid format (no "Bearer " prefix)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req.Header.Set("Authorization", "valid-test-api-key-12345")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	// Verify error message
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "invalid authorization header format" {
		t.Errorf("expected error message 'invalid authorization header format', got '%s'", resp["error"])
	}
}

// =============================================================================
// JWT Auth Middleware Tests
// =============================================================================

func TestJWTMiddleware_ValidToken(t *testing.T) {
	secretKey := "test-jwt-secret-key-for-testing"
	jwtAuth := middleware.NewJWTAuth(secretKey)

	// Create a valid token
	chatID := int64(-1001234567890)
	username := "testuser"
	token, err := jwtAuth.CreateToken(12345, &chatID, &username, "Test User")
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	handler := jwtAuth.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify claims are in context
		claims := middleware.GetClaimsFromContext(r.Context())
		if claims == nil {
			t.Fatal("expected claims in context, got nil")
		}
		if claims.UserID != 12345 {
			t.Errorf("expected user ID 12345, got %d", claims.UserID)
		}
		if claims.ChatID == nil || *claims.ChatID != chatID {
			t.Errorf("expected chat ID %d, got %v", chatID, claims.ChatID)
		}
		if claims.FirstName != "Test User" {
			t.Errorf("expected first name 'Test User', got '%s'", claims.FirstName)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestJWTMiddleware_MissingHeader(t *testing.T) {
	secretKey := "test-jwt-secret-key-for-testing"
	jwtAuth := middleware.NewJWTAuth(secretKey)

	handler := jwtAuth.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called when auth header is missing")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "missing authorization header" {
		t.Errorf("expected error 'missing authorization header', got '%s'", resp["error"])
	}
}

func TestJWTMiddleware_InvalidToken(t *testing.T) {
	secretKey := "test-jwt-secret-key-for-testing"
	jwtAuth := middleware.NewJWTAuth(secretKey)

	handler := jwtAuth.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called when token is invalid")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/test", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "invalid or expired token" {
		t.Errorf("expected error 'invalid or expired token', got '%s'", resp["error"])
	}
}

func TestJWTMiddleware_ExpiredToken(t *testing.T) {
	secretKey := "test-jwt-secret-key-for-testing"
	jwtAuth := middleware.NewJWTAuth(secretKey)

	// Create a token with a different secret (simulates tampering)
	otherAuth := middleware.NewJWTAuth("different-secret-key")
	chatID := int64(-1001234567890)
	token, _ := otherAuth.CreateToken(12345, &chatID, nil, "Test User")

	handler := jwtAuth.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called when token signature is invalid")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

// =============================================================================
// Arena Handler HTTP Status Tests
// =============================================================================

// Note: TestCreateMatch_Returns201OnSuccess is a complex integration test that
// requires proper database seeding. The arena service tests (arena_service_test.go)
// already cover this scenario comprehensively with TestCreateMatch_Success.
// Handler tests focus on HTTP-level concerns like auth, status codes, and JSON structure.

func TestCreateMatch_Returns400OnMissingChatID(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	secretKey := "test-jwt-secret-key-for-testing"
	jwtAuth := middleware.NewJWTAuth(secretKey)

	// Create token WITHOUT chat ID in claims
	token, _ := jwtAuth.CreateToken(12345, nil, nil, "Test User")

	// Request with empty body and no chat_id in claims
	reqBody := CreateMatchRequest{ChatID: 0} // No chat_id
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.HandleCreateMatch(rr, req)

	// Should return 400 because chat_id is required
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusForbidden {
		t.Errorf("expected status 400 or 403, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateMatch_Returns400OnBadRequest(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	secretKey := "test-jwt-secret-key-for-testing"
	jwtAuth := middleware.NewJWTAuth(secretKey)

	// Create token WITHOUT chat ID
	token, _ := jwtAuth.CreateToken(12345, nil, nil, "Test User")

	// Create request with invalid JSON body
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match", bytes.NewReader([]byte(`{invalid json`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.HandleCreateMatch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["error"] != "invalid request body" {
		t.Errorf("expected error 'invalid request body', got '%s'", resp["error"])
	}
}

func TestCreateMatch_Returns401OnUnauthorized(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Create request WITHOUT JWT claims in context (simulating no auth middleware)
	reqBody := CreateMatchRequest{ChatID: -1001234567890}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header and no claims in context

	rr := httptest.NewRecorder()

	handler.HandleCreateMatch(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["error"] != "unauthorized" {
		t.Errorf("expected error 'unauthorized', got '%s'", resp["error"])
	}
}

func TestCreateMatch_Returns403OnForbidden(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	secretKey := "test-jwt-secret-key-for-testing"
	jwtAuth := middleware.NewJWTAuth(secretKey)

	// Create token with one chat ID
	chatIDInToken := int64(-1001234567890)
	token, _ := jwtAuth.CreateToken(12345, &chatIDInToken, nil, "Test User")

	// Request for a DIFFERENT chat ID (access denied)
	differentChatID := int64(-1009999999999)
	reqBody := CreateMatchRequest{ChatID: differentChatID}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.HandleCreateMatch(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusForbidden, rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["error"] != "access denied to this chat" {
		t.Errorf("expected error 'access denied to this chat', got '%s'", resp["error"])
	}
}

func TestGetMatch_Returns404OnNotFound(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	secretKey := "test-jwt-secret-key-for-testing"
	jwtAuth := middleware.NewJWTAuth(secretKey)
	chatID := int64(-1001234567890)
	token, _ := jwtAuth.CreateToken(12345, &chatID, nil, "Test User")

	// Use a valid UUID format that doesn't exist
	nonExistentMatchID := "00000000-0000-0000-0000-000000000000"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/match/"+nonExistentMatchID, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	// Setup URL vars (gorilla/mux)
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.HandleGetMatch(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["error"] != "match not found" {
		t.Errorf("expected error 'match not found', got '%s'", resp["error"])
	}
}

// =============================================================================
// JSON Response Structure Tests
// =============================================================================

// Note: Full integration tests for JSON response structure are covered in
// arena_service_test.go. Handler tests verify error response structure.

func TestErrorResponse_JSONStructure(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	secretKey := "test-jwt-secret-key-for-testing"
	jwtAuth := middleware.NewJWTAuth(secretKey)
	chatID := int64(-1001234567890)
	token, _ := jwtAuth.CreateToken(12345, &chatID, nil, "Test User")

	// Request for a non-existent match (valid UUID format)
	nonExistentMatchID := "00000000-0000-0000-0000-000000000001"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/match/"+nonExistentMatchID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleGetMatch(rr, req)

	// Should return error status
	if rr.Code == http.StatusOK {
		t.Errorf("expected error status, got %d", rr.Code)
	}

	// Parse response as generic map to verify error field exists
	var respMap map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&respMap); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify error field exists
	if _, ok := respMap["error"]; !ok {
		t.Error("error response should contain 'error' field")
	}

	// Verify error is a string
	if _, ok := respMap["error"].(string); !ok {
		t.Error("'error' field should be a string")
	}
}
