package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"beef-briefing/apps/api-service/internal/apperror"
	"beef-briefing/pkg/config"

	"github.com/gorilla/mux"
)

// TestHandleUserPhotos_ValidRequest tests successful POST /profile-photos/user endpoint.
func TestHandleUserPhotos_ValidRequest(t *testing.T) {
	mockService := newMockProfilePhotoService()
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add metadata
	metadata := `{"user_id": 123, "photos": [{"file_id": "file1", "file_unique_id": "unique1", "width": 160, "height": 160}]}`
	_ = writer.WriteField("metadata", metadata)

	// Add file
	part, _ := writer.CreateFormFile("file1", "photo.jpg")
	_, _ = io.Copy(part, bytes.NewReader([]byte("fake image data")))

	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/profile-photos/user", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	handler.HandleUserPhotos(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if !mockService.processUserPhotosCalled {
		t.Error("expected ProcessUserPhotos to be called")
	}

	if mockService.lastUserID != 123 {
		t.Errorf("expected user_id=123, got %d", mockService.lastUserID)
	}

	if len(mockService.lastUserPhotos) != 1 {
		t.Errorf("expected 1 photo, got %d", len(mockService.lastUserPhotos))
	}

	if len(mockService.lastUserFiles) != 1 {
		t.Errorf("expected 1 file, got %d", len(mockService.lastUserFiles))
	}
}

// TestHandleUserPhotos_MissingMetadata tests POST /profile-photos/user without metadata field.
func TestHandleUserPhotos_MissingMetadata(t *testing.T) {
	mockService := newMockProfilePhotoService()
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/profile-photos/user", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	handler.HandleUserPhotos(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "missing metadata field" {
		t.Errorf("expected error='missing metadata field', got %v", response["error"])
	}

	if mockService.processUserPhotosCalled {
		t.Error("expected ProcessUserPhotos NOT to be called")
	}
}

// TestHandleUserPhotos_InvalidJSON tests POST /profile-photos/user with invalid JSON metadata.
func TestHandleUserPhotos_InvalidJSON(t *testing.T) {
	mockService := newMockProfilePhotoService()
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("metadata", "invalid json {")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/profile-photos/user", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	handler.HandleUserPhotos(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "invalid metadata JSON" {
		t.Errorf("expected error='invalid metadata JSON', got %v", response["error"])
	}
}

// TestHandleUserPhotos_MissingUserID tests POST /profile-photos/user with missing user_id.
func TestHandleUserPhotos_MissingUserID(t *testing.T) {
	mockService := newMockProfilePhotoService()
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("metadata", `{"photos": []}`)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/profile-photos/user", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	handler.HandleUserPhotos(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "user_id is required" {
		t.Errorf("expected error='user_id is required', got %v", response["error"])
	}
}

// TestHandleUserPhotos_ServiceError tests POST /profile-photos/user when service returns error.
func TestHandleUserPhotos_ServiceError(t *testing.T) {
	mockService := newMockProfilePhotoService()
	mockService.setProcessUserPhotosError(errors.New("database error"))
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("metadata", `{"user_id": 123, "photos": []}`)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/profile-photos/user", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	handler.HandleUserPhotos(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "internal server error" {
		t.Errorf("expected error='internal server error', got %v", response["error"])
	}
}

// TestHandleChatPhotos_ValidRequest tests successful POST /profile-photos/chat endpoint.
func TestHandleChatPhotos_ValidRequest(t *testing.T) {
	mockService := newMockProfilePhotoService()
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("metadata", `{"chat_id": -1001234567890, "photos": [{"file_id": "file1", "file_unique_id": "unique1", "photo_size": "big"}]}`)

	part, _ := writer.CreateFormFile("file1", "photo.jpg")
	_, _ = io.Copy(part, bytes.NewReader([]byte("fake image data")))

	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/profile-photos/chat", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	handler.HandleChatPhotos(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if !mockService.processChatPhotosCalled {
		t.Error("expected ProcessChatPhotos to be called")
	}

	if mockService.lastChatID != -1001234567890 {
		t.Errorf("expected chat_id=-1001234567890, got %d", mockService.lastChatID)
	}
}

// TestHandleChatPhotos_MissingMetadata tests POST /profile-photos/chat without metadata.
func TestHandleChatPhotos_MissingMetadata(t *testing.T) {
	mockService := newMockProfilePhotoService()
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/profile-photos/chat", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	handler.HandleChatPhotos(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "missing metadata field" {
		t.Errorf("expected error='missing metadata field', got %v", response["error"])
	}
}

// TestHandleChatPhotos_InvalidJSON tests POST /profile-photos/chat with invalid JSON.
func TestHandleChatPhotos_InvalidJSON(t *testing.T) {
	mockService := newMockProfilePhotoService()
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("metadata", "not valid json")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/profile-photos/chat", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	handler.HandleChatPhotos(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestHandleChatPhotos_MissingChatID tests POST /profile-photos/chat with missing chat_id.
func TestHandleChatPhotos_MissingChatID(t *testing.T) {
	mockService := newMockProfilePhotoService()
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("metadata", `{"photos": []}`)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/profile-photos/chat", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	handler.HandleChatPhotos(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "chat_id is required" {
		t.Errorf("expected error='chat_id is required', got %v", response["error"])
	}
}

// TestHandleChatPhotos_ServiceError tests POST /profile-photos/chat when service returns error.
func TestHandleChatPhotos_ServiceError(t *testing.T) {
	mockService := newMockProfilePhotoService()
	mockService.setProcessChatPhotosError(errors.New("storage error"))
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("metadata", `{"chat_id": -1001234567890, "photos": []}`)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/profile-photos/chat", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	handler.HandleChatPhotos(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

// TestHandleGetUsers_ValidRequest tests successful GET /users endpoint.
func TestHandleGetUsers_ValidRequest(t *testing.T) {
	mockService := newMockProfilePhotoService()
	mockService.setUserIDs([]int64{123, 456, 789})
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	w := httptest.NewRecorder()

	handler.HandleGetUsers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if !mockService.getAllUserIDsCalled {
		t.Error("expected GetAllUserIDs to be called")
	}

	var response map[string][]int64
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response["user_ids"]) != 3 {
		t.Errorf("expected 3 user IDs, got %d", len(response["user_ids"]))
	}
}

// TestHandleGetUsers_EmptyResult tests GET /users with no users.
func TestHandleGetUsers_EmptyResult(t *testing.T) {
	mockService := newMockProfilePhotoService()
	mockService.setUserIDs([]int64{})
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	w := httptest.NewRecorder()

	handler.HandleGetUsers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string][]int64
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response["user_ids"]) != 0 {
		t.Errorf("expected 0 user IDs, got %d", len(response["user_ids"]))
	}
}

// TestHandleGetUsers_ServiceError tests GET /users when service returns error.
func TestHandleGetUsers_ServiceError(t *testing.T) {
	mockService := newMockProfilePhotoService()
	mockService.setGetAllUserIDsError(errors.New("database connection failed"))
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	w := httptest.NewRecorder()

	handler.HandleGetUsers(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "internal server error" {
		t.Errorf("expected error='internal server error', got %v", response["error"])
	}
}

// TestHandleGetChats_ValidRequest tests successful GET /chats endpoint.
func TestHandleGetChats_ValidRequest(t *testing.T) {
	mockService := newMockProfilePhotoService()
	mockService.setChatIDs([]int64{-1001234567890, -1001234567891})
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chats", nil)
	w := httptest.NewRecorder()

	handler.HandleGetChats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if !mockService.getAllChatIDsCalled {
		t.Error("expected GetAllChatIDs to be called")
	}

	var response map[string][]int64
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response["chat_ids"]) != 2 {
		t.Errorf("expected 2 chat IDs, got %d", len(response["chat_ids"]))
	}
}

// TestHandleGetChats_EmptyResult tests GET /chats with no chats.
func TestHandleGetChats_EmptyResult(t *testing.T) {
	mockService := newMockProfilePhotoService()
	mockService.setChatIDs([]int64{})
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chats", nil)
	w := httptest.NewRecorder()

	handler.HandleGetChats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string][]int64
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response["chat_ids"]) != 0 {
		t.Errorf("expected 0 chat IDs, got %d", len(response["chat_ids"]))
	}
}

// TestHandleGetChats_ServiceError tests GET /chats when service returns error.
func TestHandleGetChats_ServiceError(t *testing.T) {
	mockService := newMockProfilePhotoService()
	mockService.setGetAllChatIDsError(errors.New("database timeout"))
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chats", nil)
	w := httptest.NewRecorder()

	handler.HandleGetChats(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

// TestHandleGetUserPhoto_ValidRequest tests successful GET /users/{id}/photo endpoint.
func TestHandleGetUserPhoto_ValidRequest(t *testing.T) {
	mockService := newMockProfilePhotoService()
	mockService.setUserPhotoURL("https://minio.example.com/photos/abc123?token=xyz")
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/123/photo?size=large", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "123"})
	w := httptest.NewRecorder()

	handler.HandleGetUserPhoto(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if !mockService.getUserPhotoCalled {
		t.Error("expected GetUserPhoto to be called")
	}

	if mockService.lastGetUserPhotoID != 123 {
		t.Errorf("expected user_id=123, got %d", mockService.lastGetUserPhotoID)
	}

	if mockService.lastGetUserPhotoSize != "large" {
		t.Errorf("expected size=large, got %s", mockService.lastGetUserPhotoSize)
	}

	var response PhotoURLResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.URL != "https://minio.example.com/photos/abc123?token=xyz" {
		t.Errorf("unexpected URL: %s", response.URL)
	}

	if response.ExpiresIn != 3600 {
		t.Errorf("expected expires_in=3600, got %d", response.ExpiresIn)
	}
}

// TestHandleGetUserPhoto_DefaultSize tests GET /users/{id}/photo with default size.
func TestHandleGetUserPhoto_DefaultSize(t *testing.T) {
	mockService := newMockProfilePhotoService()
	mockService.setUserPhotoURL("https://example.com/photo.jpg")
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/456/photo", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "456"})
	w := httptest.NewRecorder()

	handler.HandleGetUserPhoto(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Default size should be "large"
	if mockService.lastGetUserPhotoSize != "large" {
		t.Errorf("expected default size=large, got %s", mockService.lastGetUserPhotoSize)
	}
}

// TestHandleGetUserPhoto_InvalidUserID tests GET /users/{id}/photo with invalid user ID.
func TestHandleGetUserPhoto_InvalidUserID(t *testing.T) {
	mockService := newMockProfilePhotoService()
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/invalid/photo", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "invalid"})
	w := httptest.NewRecorder()

	handler.HandleGetUserPhoto(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "invalid user_id" {
		t.Errorf("expected error='invalid user_id', got %v", response["error"])
	}

	if mockService.getUserPhotoCalled {
		t.Error("expected GetUserPhoto NOT to be called")
	}
}

// TestHandleGetUserPhoto_InvalidSize tests GET /users/{id}/photo with invalid size parameter.
func TestHandleGetUserPhoto_InvalidSize(t *testing.T) {
	mockService := newMockProfilePhotoService()
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/123/photo?size=huge", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "123"})
	w := httptest.NewRecorder()

	handler.HandleGetUserPhoto(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "invalid size parameter, must be small, medium, or large" {
		t.Errorf("unexpected error: %v", response["error"])
	}
}

// TestHandleGetUserPhoto_PhotoNotFound tests GET /users/{id}/photo when photo doesn't exist.
func TestHandleGetUserPhoto_PhotoNotFound(t *testing.T) {
	mockService := newMockProfilePhotoService()
	mockService.setGetUserPhotoError(apperror.ErrPhotoNotFound)
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/123/photo", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "123"})
	w := httptest.NewRecorder()

	handler.HandleGetUserPhoto(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "photo not found" {
		t.Errorf("expected error='photo not found', got %v", response["error"])
	}
}

// TestHandleGetUserPhoto_ServiceError tests GET /users/{id}/photo when service returns error.
func TestHandleGetUserPhoto_ServiceError(t *testing.T) {
	mockService := newMockProfilePhotoService()
	mockService.setGetUserPhotoError(errors.New("minio connection failed"))
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/123/photo", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "123"})
	w := httptest.NewRecorder()

	handler.HandleGetUserPhoto(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

// TestHandleGetUserPhoto_AllSizes tests GET /users/{id}/photo with all valid size values.
func TestHandleGetUserPhoto_AllSizes(t *testing.T) {
	sizes := []string{"small", "medium", "large"}

	for _, size := range sizes {
		t.Run(size, func(t *testing.T) {
			mockService := newMockProfilePhotoService()
			mockService.setUserPhotoURL("https://example.com/photo.jpg")
			cfg := &config.Config{}

			handler := NewProfilePhotoHandler(mockService, cfg)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/users/123/photo?size="+size, nil)
			req = mux.SetURLVars(req, map[string]string{"id": "123"})
			w := httptest.NewRecorder()

			handler.HandleGetUserPhoto(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200 for size=%s, got %d", size, w.Code)
			}

			if mockService.lastGetUserPhotoSize != size {
				t.Errorf("expected size=%s, got %s", size, mockService.lastGetUserPhotoSize)
			}
		})
	}
}

// TestHandleGetChatPhoto_ValidRequest tests successful GET /chats/{id}/photo endpoint.
func TestHandleGetChatPhoto_ValidRequest(t *testing.T) {
	mockService := newMockProfilePhotoService()
	mockService.setChatPhotoURL("https://minio.example.com/photos/chat123?token=xyz")
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chats/-1001234567890/photo?size=large", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "-1001234567890"})
	w := httptest.NewRecorder()

	handler.HandleGetChatPhoto(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if !mockService.getChatPhotoCalled {
		t.Error("expected GetChatPhoto to be called")
	}

	if mockService.lastGetChatPhotoID != -1001234567890 {
		t.Errorf("expected chat_id=-1001234567890, got %d", mockService.lastGetChatPhotoID)
	}

	if mockService.lastGetChatPhotoSize != "large" {
		t.Errorf("expected size=large, got %s", mockService.lastGetChatPhotoSize)
	}

	var response PhotoURLResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.URL != "https://minio.example.com/photos/chat123?token=xyz" {
		t.Errorf("unexpected URL: %s", response.URL)
	}

	if response.ExpiresIn != 3600 {
		t.Errorf("expected expires_in=3600, got %d", response.ExpiresIn)
	}
}

// TestHandleGetChatPhoto_DefaultSize tests GET /chats/{id}/photo with default size.
func TestHandleGetChatPhoto_DefaultSize(t *testing.T) {
	mockService := newMockProfilePhotoService()
	mockService.setChatPhotoURL("https://example.com/photo.jpg")
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chats/-1001234567890/photo", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "-1001234567890"})
	w := httptest.NewRecorder()

	handler.HandleGetChatPhoto(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Default size should be "large"
	if mockService.lastGetChatPhotoSize != "large" {
		t.Errorf("expected default size=large, got %s", mockService.lastGetChatPhotoSize)
	}
}

// TestHandleGetChatPhoto_InvalidChatID tests GET /chats/{id}/photo with invalid chat ID.
func TestHandleGetChatPhoto_InvalidChatID(t *testing.T) {
	mockService := newMockProfilePhotoService()
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chats/notanumber/photo", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "notanumber"})
	w := httptest.NewRecorder()

	handler.HandleGetChatPhoto(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "invalid chat_id" {
		t.Errorf("expected error='invalid chat_id', got %v", response["error"])
	}

	if mockService.getChatPhotoCalled {
		t.Error("expected GetChatPhoto NOT to be called")
	}
}

// TestHandleGetChatPhoto_InvalidSize tests GET /chats/{id}/photo with invalid size parameter.
func TestHandleGetChatPhoto_InvalidSize(t *testing.T) {
	mockService := newMockProfilePhotoService()
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chats/-1001234567890/photo?size=extra-large", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "-1001234567890"})
	w := httptest.NewRecorder()

	handler.HandleGetChatPhoto(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "invalid size parameter, must be small, medium, or large" {
		t.Errorf("unexpected error: %v", response["error"])
	}
}

// TestHandleGetChatPhoto_PhotoNotFound tests GET /chats/{id}/photo when photo doesn't exist.
func TestHandleGetChatPhoto_PhotoNotFound(t *testing.T) {
	mockService := newMockProfilePhotoService()
	mockService.setGetChatPhotoError(apperror.ErrPhotoNotFound)
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chats/-1001234567890/photo", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "-1001234567890"})
	w := httptest.NewRecorder()

	handler.HandleGetChatPhoto(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "photo not found" {
		t.Errorf("expected error='photo not found', got %v", response["error"])
	}
}

// TestHandleGetChatPhoto_ServiceError tests GET /chats/{id}/photo when service returns error.
func TestHandleGetChatPhoto_ServiceError(t *testing.T) {
	mockService := newMockProfilePhotoService()
	mockService.setGetChatPhotoError(errors.New("storage unavailable"))
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chats/-1001234567890/photo", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "-1001234567890"})
	w := httptest.NewRecorder()

	handler.HandleGetChatPhoto(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

// TestHandleGetChatPhoto_AllSizes tests GET /chats/{id}/photo with all valid size values.
func TestHandleGetChatPhoto_AllSizes(t *testing.T) {
	sizes := []string{"small", "medium", "large"}

	for _, size := range sizes {
		t.Run(size, func(t *testing.T) {
			mockService := newMockProfilePhotoService()
			mockService.setChatPhotoURL("https://example.com/photo.jpg")
			cfg := &config.Config{}

			handler := NewProfilePhotoHandler(mockService, cfg)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/chats/-1001234567890/photo?size="+size, nil)
			req = mux.SetURLVars(req, map[string]string{"id": "-1001234567890"})
			w := httptest.NewRecorder()

			handler.HandleGetChatPhoto(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200 for size=%s, got %d", size, w.Code)
			}

			if mockService.lastGetChatPhotoSize != size {
				t.Errorf("expected size=%s, got %s", size, mockService.lastGetChatPhotoSize)
			}
		})
	}
}

// TestHandleUserPhotos_MultiplePhotos tests POST /profile-photos/user with multiple photos.
func TestHandleUserPhotos_MultiplePhotos(t *testing.T) {
	mockService := newMockProfilePhotoService()
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	metadata := `{"user_id": 123, "photos": [
		{"file_id": "file1", "file_unique_id": "unique1", "width": 160, "height": 160},
		{"file_id": "file2", "file_unique_id": "unique2", "width": 320, "height": 320},
		{"file_id": "file3", "file_unique_id": "unique3", "width": 640, "height": 640}
	]}`
	_ = writer.WriteField("metadata", metadata)

	// Add multiple files
	for _, fileID := range []string{"file1", "file2", "file3"} {
		part, _ := writer.CreateFormFile(fileID, fileID+".jpg")
		_, _ = io.Copy(part, bytes.NewReader([]byte("fake image data for "+fileID)))
	}

	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/profile-photos/user", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	handler.HandleUserPhotos(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if len(mockService.lastUserPhotos) != 3 {
		t.Errorf("expected 3 photos, got %d", len(mockService.lastUserPhotos))
	}

	if len(mockService.lastUserFiles) != 3 {
		t.Errorf("expected 3 files, got %d", len(mockService.lastUserFiles))
	}
}

// TestNewProfilePhotoHandler tests the constructor function.
func TestNewProfilePhotoHandler(t *testing.T) {
	mockService := newMockProfilePhotoService()
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}

	if handler.profilePhotoService == nil {
		t.Error("expected non-nil profilePhotoService")
	}

	if handler.config == nil {
		t.Error("expected non-nil config")
	}
}

// TestHandleUserPhotos_ContentTypeValidation tests that Content-Type is validated.
func TestHandleUserPhotos_ContentTypeValidation(t *testing.T) {
	mockService := newMockProfilePhotoService()
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	// Send non-multipart request
	req := httptest.NewRequest(http.MethodPost, "/api/v1/profile-photos/user", bytes.NewReader([]byte(`{"user_id": 123}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleUserPhotos(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for wrong content type, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "invalid multipart request" {
		t.Errorf("expected error='invalid multipart request', got %v", response["error"])
	}
}

// TestHandleChatPhotos_ContentTypeValidation tests that Content-Type is validated for chat photos.
func TestHandleChatPhotos_ContentTypeValidation(t *testing.T) {
	mockService := newMockProfilePhotoService()
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	// Send non-multipart request
	req := httptest.NewRequest(http.MethodPost, "/api/v1/profile-photos/chat", bytes.NewReader([]byte(`{"chat_id": -123}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandleChatPhotos(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for wrong content type, got %d", w.Code)
	}
}

// TestHandleUserPhotos_EmptyPhotosArray tests POST /profile-photos/user with empty photos array.
func TestHandleUserPhotos_EmptyPhotosArray(t *testing.T) {
	mockService := newMockProfilePhotoService()
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("metadata", `{"user_id": 123, "photos": []}`)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/profile-photos/user", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	handler.HandleUserPhotos(w, req)

	// Should still succeed - service handles the empty case
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if !mockService.processUserPhotosCalled {
		t.Error("expected ProcessUserPhotos to be called even with empty photos")
	}
}

// TestHandleChatPhotos_MultiplePhotos tests POST /profile-photos/chat with big and small sizes.
func TestHandleChatPhotos_MultiplePhotos(t *testing.T) {
	mockService := newMockProfilePhotoService()
	cfg := &config.Config{}

	handler := NewProfilePhotoHandler(mockService, cfg)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	metadata := `{"chat_id": -1001234567890, "photos": [
		{"file_id": "small_file", "file_unique_id": "unique1", "photo_size": "small"},
		{"file_id": "big_file", "file_unique_id": "unique2", "photo_size": "big"}
	]}`
	_ = writer.WriteField("metadata", metadata)

	part1, _ := writer.CreateFormFile("small_file", "small.jpg")
	_, _ = io.Copy(part1, bytes.NewReader([]byte("small photo data")))

	part2, _ := writer.CreateFormFile("big_file", "big.jpg")
	_, _ = io.Copy(part2, bytes.NewReader([]byte("big photo data with more bytes")))

	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/profile-photos/chat", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	handler.HandleChatPhotos(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if len(mockService.lastChatPhotos) != 2 {
		t.Errorf("expected 2 photos, got %d", len(mockService.lastChatPhotos))
	}

	if len(mockService.lastChatFiles) != 2 {
		t.Errorf("expected 2 files, got %d", len(mockService.lastChatFiles))
	}
}
