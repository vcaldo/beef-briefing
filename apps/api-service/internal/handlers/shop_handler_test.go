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

// Helper to set up ArenaHandler with real DB + services for shop tests.
func setupShopTestHandler(t *testing.T) (*ArenaHandler, func()) {
	t.Helper()
	tdb := testutil.SetupTestDB(t)
	mockMinIO := testutil.NewMockMinIOClient()
	cardService := services.NewCardService(tdb.DB, mockMinIO, nil, nil)
	arenaService := services.NewArenaService(tdb.DB, mockMinIO, cardService, nil, nil, nil, nil)
	cfg := &config.Config{}
	handler := NewArenaHandler(arenaService, cfg, nil)
	return handler, func() { testutil.TeardownTestDB(t, tdb) }
}

// Helper to create a JWT claims context with a given user and chat.
func shopJWTContext(req *http.Request, userID int64, chatID int64) *http.Request {
	secretKey := "test-jwt-secret-key-for-testing"
	jwtAuth := middleware.NewJWTAuth(secretKey)
	token, _ := jwtAuth.CreateToken(userID, &chatID, nil, "Test User")
	claims, _ := jwtAuth.ValidateToken(token)
	ctx := context.WithValue(req.Context(), middleware.JWTContextKey, claims)
	return req.WithContext(ctx)
}

// nonExistentMatchID is a valid UUID that doesn't exist in the DB.
const nonExistentMatchID = "00000000-0000-0000-0000-000000000099"

// =============================================================================
// HandleGetShop Tests
// =============================================================================

func TestHandleGetShop_Returns401WithoutJWT(t *testing.T) {
	handler, teardown := setupShopTestHandler(t)
	defer teardown()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/match/"+nonExistentMatchID+"/shop", nil)
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})
	rr := httptest.NewRecorder()

	handler.HandleGetShop(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "unauthorized" {
		t.Errorf("expected error 'unauthorized', got '%s'", resp["error"])
	}
}

func TestHandleGetShop_Returns400OnMissingMatchID(t *testing.T) {
	handler, teardown := setupShopTestHandler(t)
	defer teardown()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/match//shop", nil)
	// No mux vars set — simulates missing match ID
	req = shopJWTContext(req, 12345, -1001234567890)
	rr := httptest.NewRecorder()

	handler.HandleGetShop(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleGetShop_Returns404OnNonExistentMatch(t *testing.T) {
	handler, teardown := setupShopTestHandler(t)
	defer teardown()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mini-app/arena/match/"+nonExistentMatchID+"/shop", nil)
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})
	req = shopJWTContext(req, 12345, -1001234567890)
	rr := httptest.NewRecorder()

	handler.HandleGetShop(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d. Body: %s", rr.Code, rr.Body.String())
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
// HandleBuyCard Tests
// =============================================================================

func TestHandleBuyCard_Returns401WithoutJWT(t *testing.T) {
	handler, teardown := setupShopTestHandler(t)
	defer teardown()

	body, _ := json.Marshal(BuyCardRequest{CardIndex: 0})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+nonExistentMatchID+"/buy", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})
	rr := httptest.NewRecorder()

	handler.HandleBuyCard(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleBuyCard_Returns400OnInvalidBody(t *testing.T) {
	handler, teardown := setupShopTestHandler(t)
	defer teardown()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+nonExistentMatchID+"/buy", bytes.NewReader([]byte(`{invalid json`)))
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})
	req = shopJWTContext(req, 12345, -1001234567890)
	rr := httptest.NewRecorder()

	handler.HandleBuyCard(rr, req)

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

func TestHandleBuyCard_Returns404OnNonExistentMatch(t *testing.T) {
	handler, teardown := setupShopTestHandler(t)
	defer teardown()

	body, _ := json.Marshal(BuyCardRequest{CardIndex: 0})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+nonExistentMatchID+"/buy", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})
	req = shopJWTContext(req, 12345, -1001234567890)
	rr := httptest.NewRecorder()

	handler.HandleBuyCard(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

// =============================================================================
// HandleReroll Tests
// =============================================================================

func TestHandleReroll_Returns401WithoutJWT(t *testing.T) {
	handler, teardown := setupShopTestHandler(t)
	defer teardown()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+nonExistentMatchID+"/reroll", nil)
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})
	rr := httptest.NewRecorder()

	handler.HandleReroll(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleReroll_Returns404OnNonExistentMatch(t *testing.T) {
	handler, teardown := setupShopTestHandler(t)
	defer teardown()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+nonExistentMatchID+"/reroll", nil)
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})
	req = shopJWTContext(req, 12345, -1001234567890)
	rr := httptest.NewRecorder()

	handler.HandleReroll(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

// =============================================================================
// HandleUpgrade Tests
// =============================================================================

func TestHandleUpgrade_Returns401WithoutJWT(t *testing.T) {
	handler, teardown := setupShopTestHandler(t)
	defer teardown()

	body, _ := json.Marshal(UpgradeRequest{TeamSlot: 0, UpgradeType: "atk"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+nonExistentMatchID+"/upgrade", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})
	rr := httptest.NewRecorder()

	handler.HandleUpgrade(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleUpgrade_Returns400OnInvalidBody(t *testing.T) {
	handler, teardown := setupShopTestHandler(t)
	defer teardown()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+nonExistentMatchID+"/upgrade", bytes.NewReader([]byte(`not json`)))
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})
	req = shopJWTContext(req, 12345, -1001234567890)
	rr := httptest.NewRecorder()

	handler.HandleUpgrade(rr, req)

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

func TestHandleUpgrade_Returns404OnNonExistentMatch(t *testing.T) {
	handler, teardown := setupShopTestHandler(t)
	defer teardown()

	body, _ := json.Marshal(UpgradeRequest{TeamSlot: 0, UpgradeType: "atk"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+nonExistentMatchID+"/upgrade", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})
	req = shopJWTContext(req, 12345, -1001234567890)
	rr := httptest.NewRecorder()

	handler.HandleUpgrade(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

// =============================================================================
// HandleSetOrder Tests
// =============================================================================

func TestHandleSetOrder_Returns401WithoutJWT(t *testing.T) {
	handler, teardown := setupShopTestHandler(t)
	defer teardown()

	body, _ := json.Marshal(SetOrderRequest{Order: []int{0, 1, 2}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+nonExistentMatchID+"/order", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})
	rr := httptest.NewRecorder()

	handler.HandleSetOrder(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSetOrder_Returns400OnInvalidBody(t *testing.T) {
	handler, teardown := setupShopTestHandler(t)
	defer teardown()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+nonExistentMatchID+"/order", bytes.NewReader([]byte(`{bad`)))
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})
	req = shopJWTContext(req, 12345, -1001234567890)
	rr := httptest.NewRecorder()

	handler.HandleSetOrder(rr, req)

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

func TestHandleSetOrder_Returns404OnNonExistentMatch(t *testing.T) {
	handler, teardown := setupShopTestHandler(t)
	defer teardown()

	body, _ := json.Marshal(SetOrderRequest{Order: []int{0, 1, 2}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+nonExistentMatchID+"/order", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})
	req = shopJWTContext(req, 12345, -1001234567890)
	rr := httptest.NewRecorder()

	handler.HandleSetOrder(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

// =============================================================================
// HandleSubmitTeam Tests
// =============================================================================

func TestHandleSubmitTeam_Returns401WithoutJWT(t *testing.T) {
	handler, teardown := setupShopTestHandler(t)
	defer teardown()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+nonExistentMatchID+"/team", nil)
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})
	rr := httptest.NewRecorder()

	handler.HandleSubmitTeam(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSubmitTeam_Returns404OnNonExistentMatch(t *testing.T) {
	handler, teardown := setupShopTestHandler(t)
	defer teardown()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mini-app/arena/match/"+nonExistentMatchID+"/team", nil)
	req = mux.SetURLVars(req, map[string]string{"id": nonExistentMatchID})
	req = shopJWTContext(req, 12345, -1001234567890)
	rr := httptest.NewRecorder()

	handler.HandleSubmitTeam(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d. Body: %s", rr.Code, rr.Body.String())
	}
}
