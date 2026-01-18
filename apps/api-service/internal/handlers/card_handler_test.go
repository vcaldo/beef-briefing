package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"beef-briefing/apps/api-service/internal/apperror"
	"beef-briefing/apps/api-service/internal/repository"
	"beef-briefing/apps/api-service/internal/services"
	"beef-briefing/pkg/config"

	"github.com/gorilla/mux"
)

// mockCardService implements services.CardServiceInterface for testing.
type mockCardService struct {
	getUserCardFunc             func(ctx context.Context, userID int64, chatID int64, weekStart *time.Time) (*repository.UserCard, *repository.CardUser, error)
	getChatCardsFunc            func(ctx context.Context, req services.GetChatCardsRequest) (*services.GetChatCardsResponse, error)
	getUserHistoryFunc          func(ctx context.Context, userID int64, chatID int64, limit int) (*services.UserHistoryResponse, error)
	getAvailableWeeksFunc       func(ctx context.Context, chatID int64) (*services.AvailableWeeksResponse, error)
	getCardImageURLFunc         func(ctx context.Context, userID int64, chatID int64, weekStart *time.Time, theme string, expirySeconds int) (*services.CardImageURLResponse, error)
	getCardImageURLStringFunc   func(ctx context.Context, userID int64, chatID int64, weekStart *time.Time, theme string, expirySeconds int) (string, error)
	getPlaceholderPositionsFunc func(theme string) json.RawMessage
	getGalleryWeeksFunc         func(ctx context.Context, chatID int64) (*services.GalleryWeeksResponse, error)
	getGalleryImagesFunc        func(ctx context.Context, chatID int64, weekStart *time.Time, userID *int64, theme *string) (*services.GalleryImagesResponse, error)
	getGalleryImageURLFunc      func(ctx context.Context, imageID int64, expirySeconds int) (*services.GalleryImageURLResponse, error)
}

func (m *mockCardService) GetUserCard(ctx context.Context, userID int64, chatID int64, weekStart *time.Time) (*repository.UserCard, *repository.CardUser, error) {
	if m.getUserCardFunc != nil {
		return m.getUserCardFunc(ctx, userID, chatID, weekStart)
	}
	return nil, nil, errors.New("not implemented")
}

func (m *mockCardService) GetChatCards(ctx context.Context, req services.GetChatCardsRequest) (*services.GetChatCardsResponse, error) {
	if m.getChatCardsFunc != nil {
		return m.getChatCardsFunc(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockCardService) GetUserHistory(ctx context.Context, userID int64, chatID int64, limit int) (*services.UserHistoryResponse, error) {
	if m.getUserHistoryFunc != nil {
		return m.getUserHistoryFunc(ctx, userID, chatID, limit)
	}
	return nil, errors.New("not implemented")
}

func (m *mockCardService) GetAvailableWeeks(ctx context.Context, chatID int64) (*services.AvailableWeeksResponse, error) {
	if m.getAvailableWeeksFunc != nil {
		return m.getAvailableWeeksFunc(ctx, chatID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockCardService) GetCardImageURL(ctx context.Context, userID int64, chatID int64, weekStart *time.Time, theme string, expirySeconds int) (*services.CardImageURLResponse, error) {
	if m.getCardImageURLFunc != nil {
		return m.getCardImageURLFunc(ctx, userID, chatID, weekStart, theme, expirySeconds)
	}
	return nil, errors.New("not implemented")
}

func (m *mockCardService) GetCardImageURLString(ctx context.Context, userID int64, chatID int64, weekStart *time.Time, theme string, expirySeconds int) (string, error) {
	if m.getCardImageURLStringFunc != nil {
		return m.getCardImageURLStringFunc(ctx, userID, chatID, weekStart, theme, expirySeconds)
	}
	return "", errors.New("not implemented")
}

func (m *mockCardService) GetPlaceholderPositions(theme string) json.RawMessage {
	if m.getPlaceholderPositionsFunc != nil {
		return m.getPlaceholderPositionsFunc(theme)
	}
	return nil
}

func (m *mockCardService) GetGalleryWeeks(ctx context.Context, chatID int64) (*services.GalleryWeeksResponse, error) {
	if m.getGalleryWeeksFunc != nil {
		return m.getGalleryWeeksFunc(ctx, chatID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockCardService) GetGalleryImages(ctx context.Context, chatID int64, weekStart *time.Time, userID *int64, theme *string) (*services.GalleryImagesResponse, error) {
	if m.getGalleryImagesFunc != nil {
		return m.getGalleryImagesFunc(ctx, chatID, weekStart, userID, theme)
	}
	return nil, errors.New("not implemented")
}

func (m *mockCardService) GetGalleryImageURL(ctx context.Context, imageID int64, expirySeconds int) (*services.GalleryImageURLResponse, error) {
	if m.getGalleryImageURLFunc != nil {
		return m.getGalleryImageURLFunc(ctx, imageID, expirySeconds)
	}
	return nil, errors.New("not implemented")
}

// Helper to create a CardHandler with mock service
func newTestCardHandler() (*CardHandler, *mockCardService) {
	mockService := &mockCardService{}
	cfg := &config.Config{
		DefaultCardTheme: "neon_arcade",
	}
	handler := NewCardHandler(mockService, cfg)
	return handler, mockService
}

// TestHandleGetUserCard_ValidRequest tests retrieving a user card with valid parameters
func TestHandleGetUserCard_ValidRequest(t *testing.T) {
	handler, mockService := newTestCardHandler()

	expectedCard := &repository.UserCard{
		ID:        1,
		UserID:    123,
		ChatID:    -1003280306634,
		WeekStart: "2025-01-06",
	}
	expectedUser := &repository.CardUser{
		ID:        123,
		FirstName: "Test User",
	}

	mockService.getUserCardFunc = func(ctx context.Context, userID int64, chatID int64, weekStart *time.Time) (*repository.UserCard, *repository.CardUser, error) {
		if userID != 123 || chatID != -1003280306634 {
			t.Errorf("unexpected userID or chatID: %d, %d", userID, chatID)
		}
		return expectedCard, expectedUser, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/123?chat_id=-1003280306634", nil)
	req = mux.SetURLVars(req, map[string]string{"user_id": "123"})
	rr := httptest.NewRecorder()

	handler.HandleGetUserCard(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["card"] == nil || response["user"] == nil {
		t.Error("expected card and user in response")
	}
}

// TestHandleGetUserCard_MissingChatID tests error when chat_id is missing
func TestHandleGetUserCard_MissingChatID(t *testing.T) {
	handler, _ := newTestCardHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/123", nil)
	req = mux.SetURLVars(req, map[string]string{"user_id": "123"})
	rr := httptest.NewRecorder()

	handler.HandleGetUserCard(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["error"] != "chat_id is required" {
		t.Errorf("unexpected error message: %v", response["error"])
	}
}

// TestHandleGetUserCard_InvalidUserID tests error when user_id is invalid
func TestHandleGetUserCard_InvalidUserID(t *testing.T) {
	handler, _ := newTestCardHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/invalid?chat_id=-1003280306634", nil)
	req = mux.SetURLVars(req, map[string]string{"user_id": "invalid"})
	rr := httptest.NewRecorder()

	handler.HandleGetUserCard(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["error"] != "invalid user_id" {
		t.Errorf("unexpected error message: %v", response["error"])
	}
}

// TestHandleGetUserCard_InvalidChatID tests error when chat_id is invalid
func TestHandleGetUserCard_InvalidChatID(t *testing.T) {
	handler, _ := newTestCardHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/123?chat_id=invalid", nil)
	req = mux.SetURLVars(req, map[string]string{"user_id": "123"})
	rr := httptest.NewRecorder()

	handler.HandleGetUserCard(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["error"] != "invalid chat_id" {
		t.Errorf("unexpected error message: %v", response["error"])
	}
}

// TestHandleGetUserCard_InvalidWeekFormat tests error when week parameter has invalid format
func TestHandleGetUserCard_InvalidWeekFormat(t *testing.T) {
	handler, _ := newTestCardHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/123?chat_id=-1003280306634&week=2025-13-01", nil)
	req = mux.SetURLVars(req, map[string]string{"user_id": "123"})
	rr := httptest.NewRecorder()

	handler.HandleGetUserCard(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["error"] != "invalid week format (use YYYY-MM-DD)" {
		t.Errorf("unexpected error message: %v", response["error"])
	}
}

// TestHandleGetUserCard_CardNotFound tests 404 when card doesn't exist
func TestHandleGetUserCard_CardNotFound(t *testing.T) {
	handler, mockService := newTestCardHandler()

	mockService.getUserCardFunc = func(ctx context.Context, userID int64, chatID int64, weekStart *time.Time) (*repository.UserCard, *repository.CardUser, error) {
		return nil, nil, apperror.ErrCardNotFound
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/123?chat_id=-1003280306634", nil)
	req = mux.SetURLVars(req, map[string]string{"user_id": "123"})
	rr := httptest.NewRecorder()

	handler.HandleGetUserCard(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["error"] != "card not found" {
		t.Errorf("unexpected error message: %v", response["error"])
	}
}

// TestHandleGetUserCard_ServiceError tests 500 when service returns generic error
func TestHandleGetUserCard_ServiceError(t *testing.T) {
	handler, mockService := newTestCardHandler()

	mockService.getUserCardFunc = func(ctx context.Context, userID int64, chatID int64, weekStart *time.Time) (*repository.UserCard, *repository.CardUser, error) {
		return nil, nil, errors.New("database error")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/123?chat_id=-1003280306634", nil)
	req = mux.SetURLVars(req, map[string]string{"user_id": "123"})
	rr := httptest.NewRecorder()

	handler.HandleGetUserCard(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

// TestHandleGetChatCards_ValidRequest tests retrieving chat cards with valid parameters
func TestHandleGetChatCards_ValidRequest(t *testing.T) {
	handler, mockService := newTestCardHandler()

	expectedResponse := &services.GetChatCardsResponse{
		Cards: []repository.CardWithUser{
			{UserID: 123, User: repository.CardUser{ID: 123, FirstName: "Alice"}},
			{UserID: 456, User: repository.CardUser{ID: 456, FirstName: "Bob"}},
		},
		Pagination: services.PaginationInfo{
			Total:  2,
			Limit:  50,
			Offset: 0,
		},
	}

	mockService.getChatCardsFunc = func(ctx context.Context, req services.GetChatCardsRequest) (*services.GetChatCardsResponse, error) {
		if req.ChatID != -1003280306634 || req.SortBy != "mood" || req.Order != "desc" {
			t.Errorf("unexpected request params: chatID=%d sortBy=%s order=%s", req.ChatID, req.SortBy, req.Order)
		}
		return expectedResponse, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards?chat_id=-1003280306634", nil)
	rr := httptest.NewRecorder()

	handler.HandleGetChatCards(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response services.GetChatCardsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.Pagination.Total != 2 {
		t.Errorf("expected total=2, got %d", response.Pagination.Total)
	}
}

// TestHandleGetChatCards_MissingChatID tests error when chat_id is missing
func TestHandleGetChatCards_MissingChatID(t *testing.T) {
	handler, _ := newTestCardHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards", nil)
	rr := httptest.NewRecorder()

	handler.HandleGetChatCards(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["error"] != "chat_id is required" {
		t.Errorf("unexpected error message: %v", response["error"])
	}
}

// TestHandleGetChatCards_InvalidWeekFormat tests error when week parameter has invalid format
func TestHandleGetChatCards_InvalidWeekFormat(t *testing.T) {
	handler, _ := newTestCardHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards?chat_id=-1003280306634&week=invalid-date", nil)
	rr := httptest.NewRecorder()

	handler.HandleGetChatCards(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["error"] != "invalid week format (use YYYY-MM-DD)" {
		t.Errorf("unexpected error message: %v", response["error"])
	}
}

// TestHandleGetChatCards_Pagination tests pagination parameters
func TestHandleGetChatCards_Pagination(t *testing.T) {
	handler, mockService := newTestCardHandler()

	mockService.getChatCardsFunc = func(ctx context.Context, req services.GetChatCardsRequest) (*services.GetChatCardsResponse, error) {
		if req.Limit != 10 || req.Offset != 20 {
			t.Errorf("expected limit=10 offset=20, got limit=%d offset=%d", req.Limit, req.Offset)
		}
		return &services.GetChatCardsResponse{}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards?chat_id=-1003280306634&limit=10&offset=20", nil)
	rr := httptest.NewRecorder()

	handler.HandleGetChatCards(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

// TestHandleGetChatCards_ServiceError tests 500 when service returns error
func TestHandleGetChatCards_ServiceError(t *testing.T) {
	handler, mockService := newTestCardHandler()

	mockService.getChatCardsFunc = func(ctx context.Context, req services.GetChatCardsRequest) (*services.GetChatCardsResponse, error) {
		return nil, errors.New("database error")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards?chat_id=-1003280306634", nil)
	rr := httptest.NewRecorder()

	handler.HandleGetChatCards(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

// TestHandleGetCardImage_ValidRequest tests getting presigned URL for card image
func TestHandleGetCardImage_ValidRequest(t *testing.T) {
	handler, mockService := newTestCardHandler()

	expectedURL := &services.CardImageURLResponse{
		ImageID:   1,
		URL:       "https://s3.example.com/presigned-url",
		ExpiresIn: 3600,
		Theme:     "neon_arcade",
	}

	mockService.getCardImageURLFunc = func(ctx context.Context, userID int64, chatID int64, weekStart *time.Time, theme string, expirySeconds int) (*services.CardImageURLResponse, error) {
		if theme != "neon_arcade" || expirySeconds != 3600 {
			t.Errorf("unexpected theme or expiry: %s, %d", theme, expirySeconds)
		}
		return expectedURL, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/123/image?chat_id=-1003280306634", nil)
	req = mux.SetURLVars(req, map[string]string{"user_id": "123"})
	rr := httptest.NewRecorder()

	handler.HandleGetCardImage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response services.CardImageURLResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.URL != expectedURL.URL {
		t.Errorf("expected URL %s, got %s", expectedURL.URL, response.URL)
	}
}

// TestHandleGetCardImage_ImageNotFound tests 404 when image doesn't exist
func TestHandleGetCardImage_ImageNotFound(t *testing.T) {
	handler, mockService := newTestCardHandler()

	mockService.getCardImageURLFunc = func(ctx context.Context, userID int64, chatID int64, weekStart *time.Time, theme string, expirySeconds int) (*services.CardImageURLResponse, error) {
		return nil, apperror.ErrCardImageNotFound
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/123/image?chat_id=-1003280306634", nil)
	req = mux.SetURLVars(req, map[string]string{"user_id": "123"})
	rr := httptest.NewRecorder()

	handler.HandleGetCardImage(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response["error"] != "card image not found" {
		t.Errorf("unexpected error message: %v", response["error"])
	}
}

// TestHandleGetCardImage_CustomTheme tests using custom theme parameter
func TestHandleGetCardImage_CustomTheme(t *testing.T) {
	handler, mockService := newTestCardHandler()

	mockService.getCardImageURLFunc = func(ctx context.Context, userID int64, chatID int64, weekStart *time.Time, theme string, expirySeconds int) (*services.CardImageURLResponse, error) {
		if theme != "gaming" {
			t.Errorf("expected theme=gaming, got %s", theme)
		}
		return &services.CardImageURLResponse{URL: "https://example.com/gaming-card"}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/123/image?chat_id=-1003280306634&theme=gaming", nil)
	req = mux.SetURLVars(req, map[string]string{"user_id": "123"})
	rr := httptest.NewRecorder()

	handler.HandleGetCardImage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

// TestHandleGetCardImage_CustomExpiry tests custom expiry parameter
func TestHandleGetCardImage_CustomExpiry(t *testing.T) {
	handler, mockService := newTestCardHandler()

	mockService.getCardImageURLFunc = func(ctx context.Context, userID int64, chatID int64, weekStart *time.Time, theme string, expirySeconds int) (*services.CardImageURLResponse, error) {
		if expirySeconds != 7200 {
			t.Errorf("expected expirySeconds=7200, got %d", expirySeconds)
		}
		return &services.CardImageURLResponse{URL: "https://example.com/card"}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/123/image?chat_id=-1003280306634&expires=7200", nil)
	req = mux.SetURLVars(req, map[string]string{"user_id": "123"})
	rr := httptest.NewRecorder()

	handler.HandleGetCardImage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

// TestHandleGetUserHistory_ValidRequest tests retrieving user history
func TestHandleGetUserHistory_ValidRequest(t *testing.T) {
	handler, mockService := newTestCardHandler()

	expectedHistory := &services.UserHistoryResponse{
		User: repository.CardUser{ID: 123, FirstName: "Test User"},
		History: []services.CardSummary{
			{WeekStart: "2025-01-06", WeekEnd: "2025-01-12"},
		},
	}

	mockService.getUserHistoryFunc = func(ctx context.Context, userID int64, chatID int64, limit int) (*services.UserHistoryResponse, error) {
		if limit != 12 {
			t.Errorf("expected default limit=12, got %d", limit)
		}
		return expectedHistory, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/123/history?chat_id=-1003280306634", nil)
	req = mux.SetURLVars(req, map[string]string{"user_id": "123"})
	rr := httptest.NewRecorder()

	handler.HandleGetUserHistory(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response services.UserHistoryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(response.History) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(response.History))
	}
}

// TestHandleGetUserHistory_CustomLimit tests custom limit parameter
func TestHandleGetUserHistory_CustomLimit(t *testing.T) {
	handler, mockService := newTestCardHandler()

	mockService.getUserHistoryFunc = func(ctx context.Context, userID int64, chatID int64, limit int) (*services.UserHistoryResponse, error) {
		if limit != 20 {
			t.Errorf("expected limit=20, got %d", limit)
		}
		return &services.UserHistoryResponse{}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/123/history?chat_id=-1003280306634&limit=20", nil)
	req = mux.SetURLVars(req, map[string]string{"user_id": "123"})
	rr := httptest.NewRecorder()

	handler.HandleGetUserHistory(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

// TestHandleGetUserHistory_MissingChatID tests error when chat_id is missing
func TestHandleGetUserHistory_MissingChatID(t *testing.T) {
	handler, _ := newTestCardHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/123/history", nil)
	req = mux.SetURLVars(req, map[string]string{"user_id": "123"})
	rr := httptest.NewRecorder()

	handler.HandleGetUserHistory(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

// TestHandleGetUserHistory_ServiceError tests 500 when service returns error
func TestHandleGetUserHistory_ServiceError(t *testing.T) {
	handler, mockService := newTestCardHandler()

	mockService.getUserHistoryFunc = func(ctx context.Context, userID int64, chatID int64, limit int) (*services.UserHistoryResponse, error) {
		return nil, errors.New("database error")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/123/history?chat_id=-1003280306634", nil)
	req = mux.SetURLVars(req, map[string]string{"user_id": "123"})
	rr := httptest.NewRecorder()

	handler.HandleGetUserHistory(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

// TestHandleGetAvailableWeeks_ValidRequest tests retrieving available weeks
func TestHandleGetAvailableWeeks_ValidRequest(t *testing.T) {
	handler, mockService := newTestCardHandler()

	expectedWeeks := &services.AvailableWeeksResponse{
		Weeks: []repository.WeekInfo{
			{WeekStart: "2025-01-06", WeekEnd: "2025-01-12", CardCount: 10},
			{WeekStart: "2024-12-30", WeekEnd: "2025-01-05", CardCount: 8},
		},
		Latest: "2025-01-06",
	}

	mockService.getAvailableWeeksFunc = func(ctx context.Context, chatID int64) (*services.AvailableWeeksResponse, error) {
		return expectedWeeks, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/weeks?chat_id=-1003280306634", nil)
	rr := httptest.NewRecorder()

	handler.HandleGetAvailableWeeks(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response services.AvailableWeeksResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(response.Weeks) != 2 {
		t.Errorf("expected 2 weeks, got %d", len(response.Weeks))
	}
}

// TestHandleGetAvailableWeeks_MissingChatID tests error when chat_id is missing
func TestHandleGetAvailableWeeks_MissingChatID(t *testing.T) {
	handler, _ := newTestCardHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/weeks", nil)
	rr := httptest.NewRecorder()

	handler.HandleGetAvailableWeeks(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

// TestHandleGetAvailableWeeks_ServiceError tests 500 when service returns error
func TestHandleGetAvailableWeeks_ServiceError(t *testing.T) {
	handler, mockService := newTestCardHandler()

	mockService.getAvailableWeeksFunc = func(ctx context.Context, chatID int64) (*services.AvailableWeeksResponse, error) {
		return nil, errors.New("database error")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/weeks?chat_id=-1003280306634", nil)
	rr := httptest.NewRecorder()

	handler.HandleGetAvailableWeeks(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}
