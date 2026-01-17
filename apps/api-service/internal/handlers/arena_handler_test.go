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

// =============================================================================
// List Matches Tests
// =============================================================================

func TestListMatches_Returns200OnSuccess(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/matches?chat_id=-1001234567890", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleListMatches(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify matches field exists
	if _, ok := resp["matches"]; !ok {
		t.Error("response should contain 'matches' field")
	}
}

func TestListMatches_Returns401OnUnauthorized(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Request without JWT claims
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/matches?chat_id=-1001234567890", nil)

	rr := httptest.NewRecorder()
	handler.HandleListMatches(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

func TestListMatches_Returns400OnMissingChatID(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	secretKey := "test-jwt-secret-key-for-testing"
	jwtAuth := middleware.NewJWTAuth(secretKey)
	// Token without chat_id
	token, _ := jwtAuth.CreateToken(12345, nil, nil, "Test User")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/matches", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleListMatches(rr, req)

	// Should return 400 or 403 based on missing chat context
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusForbidden {
		t.Errorf("expected status 400 or 403, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

// =============================================================================
// Join Match Tests
// =============================================================================

func TestJoinMatch_Returns401OnUnauthorized(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Request without JWT claims
	matchID := "00000000-0000-0000-0000-000000000001"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+matchID+"/join", nil)
	req = mux.SetURLVars(req, map[string]string{"id": matchID})

	rr := httptest.NewRecorder()
	handler.HandleJoinMatch(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

func TestJoinMatch_Returns400OnMissingMatchID(t *testing.T) {
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

	// Request without match ID in URL
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match//join", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"id": ""})

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleJoinMatch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestJoinMatch_Returns404OnMatchNotFound(t *testing.T) {
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

	nonExistentMatchID := "00000000-0000-0000-0000-000000000002"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+nonExistentMatchID+"/join", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleJoinMatch(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

// =============================================================================
// Leave Match Tests
// =============================================================================

func TestLeaveMatch_Returns401OnUnauthorized(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Request without JWT claims
	matchID := "00000000-0000-0000-0000-000000000001"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+matchID+"/leave", nil)
	req = mux.SetURLVars(req, map[string]string{"id": matchID})

	rr := httptest.NewRecorder()
	handler.HandleLeaveMatch(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

func TestLeaveMatch_Returns404OnMatchNotFound(t *testing.T) {
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

	nonExistentMatchID := "00000000-0000-0000-0000-000000000003"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+nonExistentMatchID+"/leave", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleLeaveMatch(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

// =============================================================================
// Start Match Tests
// =============================================================================

func TestStartMatch_Returns401OnUnauthorized(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Request without JWT claims
	matchID := "00000000-0000-0000-0000-000000000001"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+matchID+"/start", nil)
	req = mux.SetURLVars(req, map[string]string{"id": matchID})

	rr := httptest.NewRecorder()
	handler.HandleStartMatch(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

func TestStartMatch_Returns404OnMatchNotFound(t *testing.T) {
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

	nonExistentMatchID := "00000000-0000-0000-0000-000000000004"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+nonExistentMatchID+"/start", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleStartMatch(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

// =============================================================================
// Get Shop Tests
// =============================================================================

func TestGetShop_Returns401OnUnauthorized(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Request without JWT claims
	matchID := "00000000-0000-0000-0000-000000000001"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/match/"+matchID+"/shop", nil)
	req = mux.SetURLVars(req, map[string]string{"id": matchID})

	rr := httptest.NewRecorder()
	handler.HandleGetShop(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

func TestGetShop_Returns400OnMissingMatchID(t *testing.T) {
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

	// Request without match ID in URL
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/match//shop", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"id": ""})

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleGetShop(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestGetShop_Returns404OnMatchNotFound(t *testing.T) {
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

	nonExistentMatchID := "00000000-0000-0000-0000-000000000005"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/match/"+nonExistentMatchID+"/shop", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleGetShop(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

// =============================================================================
// Buy Card Tests
// =============================================================================

func TestBuyCard_Returns401OnUnauthorized(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Request without JWT claims
	matchID := "00000000-0000-0000-0000-000000000001"
	reqBody := BuyCardRequest{CardIndex: 0}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+matchID+"/buy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": matchID})

	rr := httptest.NewRecorder()
	handler.HandleBuyCard(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

func TestBuyCard_Returns400OnInvalidJSON(t *testing.T) {
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

	matchID := "00000000-0000-0000-0000-000000000001"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+matchID+"/buy", bytes.NewReader([]byte(`{invalid json`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"id": matchID})

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleBuyCard(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestBuyCard_Returns404OnMatchNotFound(t *testing.T) {
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

	nonExistentMatchID := "00000000-0000-0000-0000-000000000006"
	reqBody := BuyCardRequest{CardIndex: 0}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+nonExistentMatchID+"/buy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleBuyCard(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

// =============================================================================
// Reroll Tests
// =============================================================================

func TestReroll_Returns401OnUnauthorized(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Request without JWT claims
	matchID := "00000000-0000-0000-0000-000000000001"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+matchID+"/reroll", nil)
	req = mux.SetURLVars(req, map[string]string{"id": matchID})

	rr := httptest.NewRecorder()
	handler.HandleReroll(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

func TestReroll_Returns404OnMatchNotFound(t *testing.T) {
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

	nonExistentMatchID := "00000000-0000-0000-0000-000000000007"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+nonExistentMatchID+"/reroll", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleReroll(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

// =============================================================================
// Upgrade Tests
// =============================================================================

func TestUpgrade_Returns401OnUnauthorized(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Request without JWT claims
	matchID := "00000000-0000-0000-0000-000000000001"
	reqBody := UpgradeRequest{TeamSlot: 0, UpgradeType: "atk"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+matchID+"/upgrade", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": matchID})

	rr := httptest.NewRecorder()
	handler.HandleUpgrade(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

func TestUpgrade_Returns400OnInvalidJSON(t *testing.T) {
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

	matchID := "00000000-0000-0000-0000-000000000001"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+matchID+"/upgrade", bytes.NewReader([]byte(`{invalid json`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"id": matchID})

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleUpgrade(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestUpgrade_Returns404OnMatchNotFound(t *testing.T) {
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

	nonExistentMatchID := "00000000-0000-0000-0000-000000000008"
	reqBody := UpgradeRequest{TeamSlot: 0, UpgradeType: "atk"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+nonExistentMatchID+"/upgrade", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleUpgrade(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

// =============================================================================
// Set Order Tests
// =============================================================================

func TestSetOrder_Returns401OnUnauthorized(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Request without JWT claims
	matchID := "00000000-0000-0000-0000-000000000001"
	reqBody := SetOrderRequest{Order: []int{0, 1, 2}}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+matchID+"/order", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": matchID})

	rr := httptest.NewRecorder()
	handler.HandleSetOrder(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

func TestSetOrder_Returns400OnInvalidJSON(t *testing.T) {
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

	matchID := "00000000-0000-0000-0000-000000000001"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+matchID+"/order", bytes.NewReader([]byte(`{invalid json`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"id": matchID})

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleSetOrder(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

// =============================================================================
// Submit Team Tests
// =============================================================================

func TestSubmitTeam_Returns401OnUnauthorized(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Request without JWT claims
	matchID := "00000000-0000-0000-0000-000000000001"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+matchID+"/team", nil)
	req = mux.SetURLVars(req, map[string]string{"id": matchID})

	rr := httptest.NewRecorder()
	handler.HandleSubmitTeam(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

func TestSubmitTeam_Returns404OnMatchNotFound(t *testing.T) {
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

	nonExistentMatchID := "00000000-0000-0000-0000-000000000009"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+nonExistentMatchID+"/team", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleSubmitTeam(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

// =============================================================================
// Get Battle Tests
// =============================================================================

func TestGetBattle_Returns401OnUnauthorized(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Request without JWT claims
	matchID := "00000000-0000-0000-0000-000000000001"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/match/"+matchID+"/battle", nil)
	req = mux.SetURLVars(req, map[string]string{"id": matchID})

	rr := httptest.NewRecorder()
	handler.HandleGetBattle(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

func TestGetBattle_Returns404OnMatchNotFound(t *testing.T) {
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

	nonExistentMatchID := "00000000-0000-0000-0000-000000000010"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/match/"+nonExistentMatchID+"/battle", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleGetBattle(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

// =============================================================================
// Get Leaderboard Tests
// =============================================================================

func TestGetLeaderboard_Returns200OnSuccess(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/leaderboard?chat_id=-1001234567890", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleGetLeaderboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify entries field exists
	if _, ok := resp["entries"]; !ok {
		t.Error("response should contain 'entries' field")
	}
	// Verify type field exists
	if _, ok := resp["type"]; !ok {
		t.Error("response should contain 'type' field")
	}
}

func TestGetLeaderboard_Returns401OnUnauthorized(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Request without JWT claims
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/leaderboard?chat_id=-1001234567890", nil)

	rr := httptest.NewRecorder()
	handler.HandleGetLeaderboard(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

func TestGetLeaderboard_UsesDefaultType(t *testing.T) {
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

	// Request without type parameter - should default to "ranked"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/leaderboard?chat_id=-1001234567890", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleGetLeaderboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify default type is "ranked"
	if resp["type"] != "ranked" {
		t.Errorf("expected type 'ranked', got '%s'", resp["type"])
	}
}

// =============================================================================
// Get Constants Tests
// =============================================================================

func TestGetConstants_Returns200OnSuccess(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/constants", nil)

	rr := httptest.NewRecorder()
	handler.HandleGetConstants(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify response structure
	if _, ok := resp["costs"]; !ok {
		t.Error("response should contain 'costs' field")
	}
	if _, ok := resp["sizes"]; !ok {
		t.Error("response should contain 'sizes' field")
	}
	if _, ok := resp["upgrades"]; !ok {
		t.Error("response should contain 'upgrades' field")
	}
	if _, ok := resp["timings"]; !ok {
		t.Error("response should contain 'timings' field")
	}

	// Verify specific values
	costs := resp["costs"].(map[string]interface{})
	if costs["card"] != float64(3) {
		t.Errorf("expected card cost 3, got %v", costs["card"])
	}
	if costs["reroll"] != float64(1) {
		t.Errorf("expected reroll cost 1, got %v", costs["reroll"])
	}
	if costs["upgrade"] != float64(1) {
		t.Errorf("expected upgrade cost 1, got %v", costs["upgrade"])
	}
}

// =============================================================================
// Get History Tests
// =============================================================================

func TestGetHistory_Returns200OnSuccess(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/history?chat_id=-1001234567890", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleGetHistory(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify response structure
	if _, ok := resp["matches"]; !ok {
		t.Error("response should contain 'matches' field")
	}
	if _, ok := resp["total"]; !ok {
		t.Error("response should contain 'total' field")
	}
	if _, ok := resp["has_more"]; !ok {
		t.Error("response should contain 'has_more' field")
	}
}

func TestGetHistory_Returns401OnUnauthorized(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Request without JWT claims
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/history?chat_id=-1001234567890", nil)

	rr := httptest.NewRecorder()
	handler.HandleGetHistory(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

// =============================================================================
// Get Profile Tests
// =============================================================================

func TestGetProfile_Returns200OnSuccess(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/profile?chat_id=-1001234567890", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleGetProfile(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify response structure
	if _, ok := resp["recent_matches"]; !ok {
		t.Error("response should contain 'recent_matches' field")
	}
	// profile can be null for users without arena data
}

func TestGetProfile_Returns401OnUnauthorized(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Request without JWT claims
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/profile?chat_id=-1001234567890", nil)

	rr := httptest.NewRecorder()
	handler.HandleGetProfile(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

// =============================================================================
// Get H2H Tests
// =============================================================================

func TestGetH2H_Returns200OnSuccess(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/h2h?chat_id=-1001234567890&opponent_id=67890", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleGetH2H(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify response structure
	if _, ok := resp["recent_matches"]; !ok {
		t.Error("response should contain 'recent_matches' field")
	}
	// record can be null if no matches played
}

func TestGetH2H_Returns401OnUnauthorized(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Request without JWT claims
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/h2h?chat_id=-1001234567890&opponent_id=67890", nil)

	rr := httptest.NewRecorder()
	handler.HandleGetH2H(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

func TestGetH2H_Returns400OnMissingOpponentID(t *testing.T) {
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

	// Missing opponent_id parameter
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/h2h?chat_id=-1001234567890", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleGetH2H(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

// =============================================================================
// Share Result Tests
// =============================================================================

func TestShareResult_Returns401OnUnauthorized(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Request without JWT claims
	matchID := "00000000-0000-0000-0000-000000000001"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+matchID+"/share", nil)
	req = mux.SetURLVars(req, map[string]string{"id": matchID})

	rr := httptest.NewRecorder()
	handler.HandleShareResult(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

func TestShareResult_Returns400OnMissingMatchID(t *testing.T) {
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

	// Request without match ID in URL
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match//share", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"id": ""})

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleShareResult(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestShareResult_Returns404OnMatchNotFound(t *testing.T) {
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

	nonExistentMatchID := "00000000-0000-0000-0000-000000000011"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+nonExistentMatchID+"/share", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})

	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.HandleShareResult(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

// =============================================================================
// Bot Endpoint Tests
// =============================================================================

func TestBotCreateMatch_Returns400OnMissingFields(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Request with missing fields
	reqBody := BotCreateMatchRequest{ChatID: 0, CreatorUserID: 0}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/arena/match", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.HandleBotCreateMatch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestBotCreateMatch_Returns400OnInvalidJSON(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/arena/match", bytes.NewReader([]byte(`{invalid json`)))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.HandleBotCreateMatch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestBotGetMatch_Returns400OnMissingMatchID(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/arena/match/", nil)
	req = mux.SetURLVars(req, map[string]string{"id": ""})

	rr := httptest.NewRecorder()
	handler.HandleBotGetMatch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestBotGetMatch_Returns404OnMatchNotFound(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	nonExistentMatchID := "00000000-0000-0000-0000-000000000012"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/arena/match/"+nonExistentMatchID, nil)
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})

	rr := httptest.NewRecorder()
	handler.HandleBotGetMatch(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

func TestBotJoinMatch_Returns400OnMissingUserID(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	matchID := "00000000-0000-0000-0000-000000000001"
	reqBody := BotJoinMatchRequest{UserID: 0}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/arena/match/"+matchID+"/join", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": matchID})

	rr := httptest.NewRecorder()
	handler.HandleBotJoinMatch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestBotJoinMatch_Returns400OnInvalidJSON(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	matchID := "00000000-0000-0000-0000-000000000001"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/arena/match/"+matchID+"/join", bytes.NewReader([]byte(`{invalid json`)))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": matchID})

	rr := httptest.NewRecorder()
	handler.HandleBotJoinMatch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestBotLeaveMatch_Returns400OnMissingUserID(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	matchID := "00000000-0000-0000-0000-000000000001"
	reqBody := BotJoinMatchRequest{UserID: 0}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/arena/match/"+matchID+"/leave", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": matchID})

	rr := httptest.NewRecorder()
	handler.HandleBotLeaveMatch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestBotStartMatch_Returns400OnMissingUserID(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	matchID := "00000000-0000-0000-0000-000000000001"
	reqBody := BotJoinMatchRequest{UserID: 0}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/arena/match/"+matchID+"/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = mux.SetURLVars(req, map[string]string{"id": matchID})

	rr := httptest.NewRecorder()
	handler.HandleBotStartMatch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestBotGetPendingMatches_Returns200OnSuccess(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/arena/matches/pending", nil)

	rr := httptest.NewRecorder()
	handler.HandleBotGetPendingMatches(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify matches field exists
	if _, ok := resp["matches"]; !ok {
		t.Error("response should contain 'matches' field")
	}
}

func TestBotAutoStartMatch_Returns404OnMatchNotFound(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	nonExistentMatchID := "00000000-0000-0000-0000-000000000013"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/arena/match/"+nonExistentMatchID+"/auto-start", nil)
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})

	rr := httptest.NewRecorder()
	handler.HandleBotAutoStartMatch(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

func TestBotForceSubmitTeams_Returns404OnMatchNotFound(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	nonExistentMatchID := "00000000-0000-0000-0000-000000000014"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/arena/match/"+nonExistentMatchID+"/force-submit", nil)
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})

	rr := httptest.NewRecorder()
	handler.HandleBotForceSubmitTeams(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

func TestBotGetShareData_Returns400OnMissingMatchID(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/arena/match//share-data", nil)
	req = mux.SetURLVars(req, map[string]string{"id": ""})

	rr := httptest.NewRecorder()
	handler.HandleBotGetShareData(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestBotGetShareData_Returns404OnMatchNotFound(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	nonExistentMatchID := "00000000-0000-0000-0000-000000000015"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/arena/match/"+nonExistentMatchID+"/share-data", nil)
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})

	rr := httptest.NewRecorder()
	handler.HandleBotGetShareData(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

// =============================================================================
// Tournament Handler Tests
// =============================================================================

func TestBotGetTodayTournament_Returns400OnMissingChatID(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Request without chat_id
	req := httptest.NewRequest(http.MethodGet, "/api/v1/arena/tournament/today?date=2026-01-17", nil)

	rr := httptest.NewRecorder()
	handler.HandleBotGetTodayTournament(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "chat_id is required" {
		t.Errorf("expected error message 'chat_id is required', got '%s'", resp["error"])
	}
}

func TestBotGetTodayTournament_Returns400OnMissingDate(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Request without date
	req := httptest.NewRequest(http.MethodGet, "/api/v1/arena/tournament/today?chat_id=-1001234567890", nil)

	rr := httptest.NewRecorder()
	handler.HandleBotGetTodayTournament(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "date is required (YYYY-MM-DD)" {
		t.Errorf("expected error message 'date is required (YYYY-MM-DD)', got '%s'", resp["error"])
	}
}

func TestBotGetTodayTournament_Returns200OnNoTournament(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Request for a chat with no tournament
	req := httptest.NewRequest(http.MethodGet, "/api/v1/arena/tournament/today?chat_id=-1001234567890&date=2026-01-17", nil)

	rr := httptest.NewRecorder()
	handler.HandleBotGetTodayTournament(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["tournament"] != nil {
		t.Errorf("expected tournament to be nil, got %v", resp["tournament"])
	}
}

func TestBotGetTournament_Returns400OnInvalidID(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/arena/tournament/invalid", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "invalid"})

	rr := httptest.NewRecorder()
	handler.HandleBotGetTournament(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestBotGetTournament_Returns404OnNotFound(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/arena/tournament/99999", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "99999"})

	rr := httptest.NewRecorder()
	handler.HandleBotGetTournament(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

func TestBotGetPendingAnnouncements_Returns200(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/arena/tournaments/pending-announcements", nil)

	rr := httptest.NewRecorder()
	handler.HandleBotGetPendingAnnouncements(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if _, ok := resp["tournaments"]; !ok {
		t.Errorf("expected 'tournaments' key in response")
	}
}

func TestBotAnnounceTournament_Returns400OnMissingFields(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Request with missing fields
	reqBody := TournamentAnnounceRequest{ChatID: 0, Date: ""}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/arena/tournament/announce", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.HandleBotAnnounceTournament(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestBotAnnounceTournament_Returns400OnInvalidJSON(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/arena/tournament/announce", bytes.NewReader([]byte(`{invalid json`)))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.HandleBotAnnounceTournament(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestBotJoinTournament_Returns400OnMissingFields(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Request with missing fields
	reqBody := TournamentJoinRequest{ChatID: 0, UserID: 0}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/arena/tournament/join", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.HandleBotJoinTournament(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestBotJoinTournament_Returns400OnInvalidJSON(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/arena/tournament/join", bytes.NewReader([]byte(`{invalid json`)))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.HandleBotJoinTournament(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestBotJoinTournament_Returns404OnNoTournament(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Request for a chat with no tournament
	reqBody := TournamentJoinRequest{ChatID: -1001234567890, UserID: 12345}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/arena/tournament/join", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.HandleBotJoinTournament(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

func TestBotLeaveTournament_Returns400OnMissingFields(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Request with missing fields
	reqBody := TournamentJoinRequest{ChatID: 0, UserID: 0}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/arena/tournament/leave", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.HandleBotLeaveTournament(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestBotLeaveTournament_Returns400OnInvalidJSON(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/arena/tournament/leave", bytes.NewReader([]byte(`{invalid json`)))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.HandleBotLeaveTournament(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestBotLeaveTournament_Returns404OnNoTournament(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	// Request for a chat with no tournament
	reqBody := TournamentJoinRequest{ChatID: -1001234567890, UserID: 12345}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/arena/tournament/leave", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.HandleBotLeaveTournament(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

func TestBotGetPendingClose_Returns200(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/arena/tournaments/pending-close", nil)

	rr := httptest.NewRecorder()
	handler.HandleBotGetPendingClose(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if _, ok := resp["tournaments"]; !ok {
		t.Errorf("expected 'tournaments' key in response")
	}
}

func TestBotCloseTournament_Returns400OnInvalidID(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/arena/tournament/invalid/close", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "invalid"})

	rr := httptest.NewRecorder()
	handler.HandleBotCloseTournament(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestBotCloseTournament_Returns404OnNotFound(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/arena/tournament/99999/close", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "99999"})

	rr := httptest.NewRecorder()
	handler.HandleBotCloseTournament(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusNotFound, rr.Code, rr.Body.String())
	}
}

func TestBotGetPendingRounds_Returns200(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil)

	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/arena/tournaments/pending-rounds", nil)

	rr := httptest.NewRecorder()
	handler.HandleBotGetPendingRounds(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if _, ok := resp["tournaments"]; !ok {
		t.Errorf("expected 'tournaments' key in response")
	}
}
