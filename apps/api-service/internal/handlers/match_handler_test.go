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

// Helper to set up ArenaHandler with real DB + services for match tests.
func setupMatchTestHandler(t *testing.T) (*ArenaHandler, func()) {
	t.Helper()
	tdb := testutil.SetupTestDB(t)
	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil, nil)
	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg, nil)
	return handler, func() { testutil.TeardownTestDB(t, tdb) }
}

// Helper to create a JWT claims context with a given user and chat.
func matchJWTContext(req *http.Request, userID int64, chatID int64) *http.Request {
	secretKey := "test-jwt-secret-key-for-testing"
	jwtAuth := middleware.NewJWTAuth(secretKey)
	token, _ := jwtAuth.CreateToken(userID, &chatID, nil, "Test User")
	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	return req.WithContext(ctx)
}

// =============================================================================
// HandleBotGetUserActiveMatch Tests
// =============================================================================

func TestHandleBotGetUserActiveMatch_Returns400OnMissingChatID(t *testing.T) {
	handler, teardown := setupMatchTestHandler(t)
	defer teardown()

	// Request without chat_id but with user_id
	req := httptest.NewRequest(http.MethodGet, "/api/v1/arena/matches/active?user_id=12345", nil)
	rr := httptest.NewRecorder()

	handler.HandleBotGetUserActiveMatch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "chat_id is required" {
		t.Errorf("expected error 'chat_id is required', got '%s'", resp["error"])
	}
}

func TestHandleBotGetUserActiveMatch_Returns400OnMissingUserID(t *testing.T) {
	handler, teardown := setupMatchTestHandler(t)
	defer teardown()

	// Request with chat_id but without user_id
	req := httptest.NewRequest(http.MethodGet, "/api/v1/arena/matches/active?chat_id=-1001234567890", nil)
	rr := httptest.NewRecorder()

	handler.HandleBotGetUserActiveMatch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "user_id is required" {
		t.Errorf("expected error 'user_id is required', got '%s'", resp["error"])
	}
}

func TestHandleBotGetUserActiveMatch_Returns404OnNoActiveMatch(t *testing.T) {
	handler, teardown := setupMatchTestHandler(t)
	defer teardown()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/arena/matches/active?chat_id=-1001234567890&user_id=12345", nil)
	rr := httptest.NewRecorder()

	handler.HandleBotGetUserActiveMatch(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "no active match found" {
		t.Errorf("expected error 'no active match found', got '%s'", resp["error"])
	}
}

// =============================================================================
// HandleGetChatOpenMatch Tests
// =============================================================================

func TestHandleGetChatOpenMatch_Returns400OnMissingChatID(t *testing.T) {
	handler, teardown := setupMatchTestHandler(t)
	defer teardown()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/arena/matches/open", nil)
	rr := httptest.NewRecorder()

	handler.HandleGetChatOpenMatch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "chat_id is required" {
		t.Errorf("expected error 'chat_id is required', got '%s'", resp["error"])
	}
}

func TestHandleGetChatOpenMatch_Returns404OnNoOpenMatch(t *testing.T) {
	handler, teardown := setupMatchTestHandler(t)
	defer teardown()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/arena/matches/open?chat_id=-1001234567890", nil)
	rr := httptest.NewRecorder()

	handler.HandleGetChatOpenMatch(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "no open match found" {
		t.Errorf("expected error 'no open match found', got '%s'", resp["error"])
	}
}

// =============================================================================
// HandleSetTelegramMessageID Tests
// =============================================================================

func TestHandleSetTelegramMessageID_Returns400OnInvalidBody(t *testing.T) {
	handler, teardown := setupMatchTestHandler(t)
	defer teardown()

	matchID := "00000000-0000-0000-0000-000000000001"
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/arena/match/"+matchID+"/message-id", bytes.NewReader([]byte(`{invalid json`)))
	req = mux.SetURLVars(req, map[string]string{"id": matchID})
	rr := httptest.NewRecorder()

	handler.HandleSetTelegramMessageID(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "invalid request body" {
		t.Errorf("expected error 'invalid request body', got '%s'", resp["error"])
	}
}

func TestHandleSetTelegramMessageID_Returns400OnMissingMessageID(t *testing.T) {
	handler, teardown := setupMatchTestHandler(t)
	defer teardown()

	matchID := "00000000-0000-0000-0000-000000000001"
	body, _ := json.Marshal(map[string]int64{"telegram_message_id": 0})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/arena/match/"+matchID+"/message-id", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": matchID})
	rr := httptest.NewRecorder()

	handler.HandleSetTelegramMessageID(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "telegram_message_id is required" {
		t.Errorf("expected error 'telegram_message_id is required', got '%s'", resp["error"])
	}
}

func TestHandleSetTelegramMessageID_Returns400OnMissingMatchID(t *testing.T) {
	handler, teardown := setupMatchTestHandler(t)
	defer teardown()

	body, _ := json.Marshal(map[string]int64{"telegram_message_id": 12345})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/arena/match//message-id", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": ""})
	rr := httptest.NewRecorder()

	handler.HandleSetTelegramMessageID(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "match ID is required" {
		t.Errorf("expected error 'match ID is required', got '%s'", resp["error"])
	}
}

// =============================================================================
// HandleGetUserActiveMatch (Mini-App JWT) Tests
// =============================================================================

func TestHandleGetUserActiveMatch_Returns401OnMissingJWT(t *testing.T) {
	handler, teardown := setupMatchTestHandler(t)
	defer teardown()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/matches/active?chat_id=-1001234567890&user_id=12345", nil)
	rr := httptest.NewRecorder()

	handler.HandleGetUserActiveMatch(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetUserActiveMatch_Returns400OnMissingChatID(t *testing.T) {
	handler, teardown := setupMatchTestHandler(t)
	defer teardown()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/matches/active?user_id=12345", nil)
	req = matchJWTContext(req, 12345, -1001234567890)
	rr := httptest.NewRecorder()

	handler.HandleGetUserActiveMatch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetUserActiveMatch_Returns403OnOtherUsersMatch(t *testing.T) {
	handler, teardown := setupMatchTestHandler(t)
	defer teardown()

	// JWT has user_id=12345 but we're querying user_id=99999
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/matches/active?chat_id=-1001234567890&user_id=99999", nil)
	req = matchJWTContext(req, 12345, -1001234567890)
	rr := httptest.NewRecorder()

	handler.HandleGetUserActiveMatch(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "can only query your own active match" {
		t.Errorf("expected error 'can only query your own active match', got '%s'", resp["error"])
	}
}

func TestHandleGetUserActiveMatch_Returns404OnNoActiveMatch(t *testing.T) {
	handler, teardown := setupMatchTestHandler(t)
	defer teardown()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/matches/active?chat_id=-1001234567890&user_id=12345", nil)
	req = matchJWTContext(req, 12345, -1001234567890)
	rr := httptest.NewRecorder()

	handler.HandleGetUserActiveMatch(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

// =============================================================================
// HandleGetRoundBattle Tests
// =============================================================================

func TestHandleGetRoundBattle_Returns401OnMissingJWT(t *testing.T) {
	handler, teardown := setupMatchTestHandler(t)
	defer teardown()

	matchID := "00000000-0000-0000-0000-000000000099"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/match/"+matchID+"/battle/round/1", nil)
	req = mux.SetURLVars(req, map[string]string{"id": matchID, "round_number": "1"})
	rr := httptest.NewRecorder()

	handler.HandleGetRoundBattle(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetRoundBattle_Returns400OnMissingRoundNumber(t *testing.T) {
	handler, teardown := setupMatchTestHandler(t)
	defer teardown()

	matchID := "00000000-0000-0000-0000-000000000099"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/match/"+matchID+"/battle/round/", nil)
	req = mux.SetURLVars(req, map[string]string{"id": matchID, "round_number": ""})
	req = matchJWTContext(req, 12345, -1001234567890)
	rr := httptest.NewRecorder()

	handler.HandleGetRoundBattle(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "round_number is required" {
		t.Errorf("expected error 'round_number is required', got '%s'", resp["error"])
	}
}

func TestHandleGetRoundBattle_Returns400OnInvalidRoundNumber(t *testing.T) {
	handler, teardown := setupMatchTestHandler(t)
	defer teardown()

	matchID := "00000000-0000-0000-0000-000000000099"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/match/"+matchID+"/battle/round/abc", nil)
	req = mux.SetURLVars(req, map[string]string{"id": matchID, "round_number": "abc"})
	req = matchJWTContext(req, 12345, -1001234567890)
	rr := httptest.NewRecorder()

	handler.HandleGetRoundBattle(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "round_number must be an integer" {
		t.Errorf("expected error 'round_number must be an integer', got '%s'", resp["error"])
	}
}

func TestHandleGetRoundBattle_Returns400OnMissingMatchID(t *testing.T) {
	handler, teardown := setupMatchTestHandler(t)
	defer teardown()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/match//battle/round/1", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "", "round_number": "1"})
	req = matchJWTContext(req, 12345, -1001234567890)
	rr := httptest.NewRecorder()

	handler.HandleGetRoundBattle(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetRoundBattle_Returns404OnNonExistentMatch(t *testing.T) {
	handler, teardown := setupMatchTestHandler(t)
	defer teardown()

	matchID := "00000000-0000-0000-0000-000000000099"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/match/"+matchID+"/battle/round/1", nil)
	req = mux.SetURLVars(req, map[string]string{"id": matchID, "round_number": "1"})
	req = matchJWTContext(req, 12345, -1001234567890)
	rr := httptest.NewRecorder()

	handler.HandleGetRoundBattle(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

// =============================================================================
// HandleListMatches Tests (additional - missing JWT already tested in arena_handler_test.go)
// =============================================================================

func TestHandleListMatches_Returns403OnWrongChat(t *testing.T) {
	handler, teardown := setupMatchTestHandler(t)
	defer teardown()

	// JWT is for chat -1001234567890 but query asks for a different chat
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/matches?chat_id=-1009999999999", nil)
	req = matchJWTContext(req, 12345, -1001234567890)
	rr := httptest.NewRecorder()

	handler.HandleListMatches(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

// =============================================================================
// HandleShareResult Tests (additional)
// =============================================================================

func TestHandleShareResult_Returns401OnMissingJWT(t *testing.T) {
	handler, teardown := setupMatchTestHandler(t)
	defer teardown()

	matchID := "00000000-0000-0000-0000-000000000099"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+matchID+"/share", nil)
	req = mux.SetURLVars(req, map[string]string{"id": matchID})
	rr := httptest.NewRecorder()

	handler.HandleShareResult(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}
