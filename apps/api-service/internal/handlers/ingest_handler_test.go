package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"beef-briefing/apps/api-service/internal/middleware"
	"beef-briefing/apps/api-service/internal/testutil"
	"beef-briefing/pkg/config"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupIngestHandlerTest creates a test handler with mock service and config
func setupIngestHandlerTest(t *testing.T) (*IngestHandler, *testutil.MockIngestService, *config.Config) {
	t.Helper()

	cfg := &config.Config{
		MaxUploadSizeMB: 100, // 100 MB limit
		Environment:     "test",
	}

	mockService := testutil.NewMockIngestService()
	handler := NewIngestHandler(mockService, cfg)

	return handler, mockService, cfg
}

// createMultipartRequest creates a multipart form request with update JSON and optional files
func createMultipartRequest(t *testing.T, updateJSON string, files map[string][]byte) *http.Request {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add update field
	if updateJSON != "" {
		if err := writer.WriteField("update", updateJSON); err != nil {
			t.Fatalf("failed to write update field: %v", err)
		}
	}

	// Add file fields
	for fileID, fileData := range files {
		part, err := writer.CreateFormFile(fileID, fileID)
		if err != nil {
			t.Fatalf("failed to create form file %s: %v", fileID, err)
		}
		if _, err := part.Write(fileData); err != nil {
			t.Fatalf("failed to write file data for %s: %v", fileID, err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	return req
}

// =============================================================================
// Ingest Handler Tests
// =============================================================================

func TestHandleIngest_ValidRequest(t *testing.T) {
	handler, mockService, _ := setupIngestHandlerTest(t)

	updateJSON := `{
		"update_id": 12345,
		"message": {
			"message_id": 1,
			"chat": {"id": -1003280306634, "type": "supergroup"},
			"from": {"id": 789, "first_name": "TestUser"},
			"date": 1733611200,
			"text": "Hello World"
		}
	}`

	req := createMultipartRequest(t, updateJSON, nil)
	rr := httptest.NewRecorder()

	handler.HandleIngest(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Verify response body
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got '%s'", resp["status"])
	}

	// Verify service was called
	if !mockService.ProcessUpdateCalled {
		t.Error("expected ProcessUpdate to be called")
	}
	if mockService.LastUpdate == nil {
		t.Fatal("expected LastUpdate to be set")
	}
	if mockService.LastUpdate.UpdateID != 12345 {
		t.Errorf("expected update_id 12345, got %d", mockService.LastUpdate.UpdateID)
	}
}

func TestHandleIngest_WithMediaFiles(t *testing.T) {
	handler, mockService, _ := setupIngestHandlerTest(t)

	updateJSON := `{
		"update_id": 12346,
		"message": {
			"message_id": 2,
			"chat": {"id": -1003280306634, "type": "supergroup"},
			"from": {"id": 789, "first_name": "TestUser"},
			"date": 1733611200,
			"photo": [{"file_id": "photo123", "file_unique_id": "unique123"}]
		}
	}`

	files := map[string][]byte{
		"photo123": []byte("fake-photo-data"),
	}

	req := createMultipartRequest(t, updateJSON, files)
	rr := httptest.NewRecorder()

	handler.HandleIngest(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Verify files were passed to service
	if !mockService.ProcessUpdateCalled {
		t.Error("expected ProcessUpdate to be called")
	}
	if len(mockService.LastFiles) != 1 {
		t.Errorf("expected 1 file, got %d", len(mockService.LastFiles))
	}
	if data, ok := mockService.LastFiles["photo123"]; !ok {
		t.Error("expected photo123 in files map")
	} else if string(data) != "fake-photo-data" {
		t.Errorf("expected 'fake-photo-data', got '%s'", string(data))
	}
}

func TestHandleIngest_MissingUpdateField(t *testing.T) {
	handler, mockService, _ := setupIngestHandlerTest(t)

	// Create multipart request without update field
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	handler.HandleIngest(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "missing update field" {
		t.Errorf("expected error 'missing update field', got '%s'", resp["error"])
	}

	// Verify service was NOT called
	if mockService.ProcessUpdateCalled {
		t.Error("expected ProcessUpdate to NOT be called")
	}
}

func TestHandleIngest_InvalidJSON(t *testing.T) {
	handler, mockService, _ := setupIngestHandlerTest(t)

	// Invalid JSON (missing closing brace)
	updateJSON := `{"update_id": 12345, "message": {`

	req := createMultipartRequest(t, updateJSON, nil)
	rr := httptest.NewRecorder()

	handler.HandleIngest(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "invalid update JSON" {
		t.Errorf("expected error 'invalid update JSON', got '%s'", resp["error"])
	}

	// Verify service was NOT called
	if mockService.ProcessUpdateCalled {
		t.Error("expected ProcessUpdate to NOT be called")
	}
}

func TestHandleIngest_ServiceError(t *testing.T) {
	handler, mockService, _ := setupIngestHandlerTest(t)

	// Configure mock to return error
	mockService.SetProcessUpdateError(fmt.Errorf("database connection failed"))

	updateJSON := `{
		"update_id": 12347,
		"message": {
			"message_id": 3,
			"chat": {"id": -1003280306634, "type": "supergroup"},
			"from": {"id": 789, "first_name": "TestUser"},
			"date": 1733611200,
			"text": "Test"
		}
	}`

	req := createMultipartRequest(t, updateJSON, nil)
	rr := httptest.NewRecorder()

	handler.HandleIngest(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "internal server error" {
		t.Errorf("expected error 'internal server error', got '%s'", resp["error"])
	}

	// Verify service WAS called (error happens during processing)
	if !mockService.ProcessUpdateCalled {
		t.Error("expected ProcessUpdate to be called")
	}
}

func TestHandleIngest_InvalidMultipartForm(t *testing.T) {
	handler, mockService, _ := setupIngestHandlerTest(t)

	// Create request with wrong Content-Type (not multipart)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", strings.NewReader("not-multipart"))
	req.Header.Set("Content-Type", "text/plain")

	rr := httptest.NewRecorder()
	handler.HandleIngest(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "invalid multipart request" {
		t.Errorf("expected error 'invalid multipart request', got '%s'", resp["error"])
	}

	// Verify service was NOT called
	if mockService.ProcessUpdateCalled {
		t.Error("expected ProcessUpdate to NOT be called")
	}
}

func TestHandleIngest_MultipleFiles(t *testing.T) {
	handler, mockService, _ := setupIngestHandlerTest(t)

	updateJSON := `{
		"update_id": 12348,
		"message": {
			"message_id": 4,
			"chat": {"id": -1003280306634, "type": "supergroup"},
			"from": {"id": 789, "first_name": "TestUser"},
			"date": 1733611200,
			"text": "Multiple files test"
		}
	}`

	files := map[string][]byte{
		"file1": []byte("data1"),
		"file2": []byte("data2"),
		"file3": []byte("data3"),
	}

	req := createMultipartRequest(t, updateJSON, files)
	rr := httptest.NewRecorder()

	handler.HandleIngest(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Verify all files were extracted
	if len(mockService.LastFiles) != 3 {
		t.Errorf("expected 3 files, got %d", len(mockService.LastFiles))
	}
	for fileID, expectedData := range files {
		if data, ok := mockService.LastFiles[fileID]; !ok {
			t.Errorf("expected %s in files map", fileID)
		} else if !bytes.Equal(data, expectedData) {
			t.Errorf("file %s: expected %v, got %v", fileID, expectedData, data)
		}
	}
}

func TestHandleIngest_EmptyUpdateJSON(t *testing.T) {
	handler, mockService, _ := setupIngestHandlerTest(t)

	// Empty string for update field
	req := createMultipartRequest(t, "", nil)
	rr := httptest.NewRecorder()

	handler.HandleIngest(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "missing update field" {
		t.Errorf("expected error 'missing update field', got '%s'", resp["error"])
	}

	// Verify service was NOT called
	if mockService.ProcessUpdateCalled {
		t.Error("expected ProcessUpdate to NOT be called")
	}
}

func TestHandleIngest_AllUpdateTypes(t *testing.T) {
	testCases := []struct {
		name       string
		updateJSON string
		updateID   int64
	}{
		{
			name: "message",
			updateJSON: `{
				"update_id": 1,
				"message": {
					"message_id": 1,
					"chat": {"id": -100123, "type": "supergroup"},
					"from": {"id": 456, "first_name": "User"},
					"date": 1733611200,
					"text": "Test"
				}
			}`,
			updateID: 1,
		},
		{
			name: "edited_message",
			updateJSON: `{
				"update_id": 2,
				"edited_message": {
					"message_id": 1,
					"chat": {"id": -100123, "type": "supergroup"},
					"from": {"id": 456, "first_name": "User"},
					"date": 1733611200,
					"edit_date": 1733611300,
					"text": "Edited"
				}
			}`,
			updateID: 2,
		},
		{
			name: "message_reaction",
			updateJSON: `{
				"update_id": 3,
				"message_reaction": {
					"chat": {"id": -100123, "type": "supergroup"},
					"message_id": 1,
					"user": {"id": 456, "first_name": "User"},
					"date": 1733611200,
					"old_reaction": [],
					"new_reaction": [{"type": "emoji", "emoji": "👍"}]
				}
			}`,
			updateID: 3,
		},
		{
			name: "message_reaction_count",
			updateJSON: `{
				"update_id": 4,
				"message_reaction_count": {
					"chat": {"id": -100123, "type": "supergroup"},
					"message_id": 1,
					"date": 1733611200,
					"reactions": [{"type": {"type": "emoji", "emoji": "👍"}, "count": 5}]
				}
			}`,
			updateID: 4,
		},
		{
			name: "my_chat_member",
			updateJSON: `{
				"update_id": 5,
				"my_chat_member": {
					"chat": {"id": -100123, "type": "supergroup"},
					"from": {"id": 456, "first_name": "User"},
					"date": 1733611200,
					"old_chat_member": {"user": {"id": 789, "first_name": "Bot"}, "status": "member"},
					"new_chat_member": {"user": {"id": 789, "first_name": "Bot"}, "status": "administrator"}
				}
			}`,
			updateID: 5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler, mockService, _ := setupIngestHandlerTest(t)

			req := createMultipartRequest(t, tc.updateJSON, nil)
			rr := httptest.NewRecorder()

			handler.HandleIngest(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
			}

			if !mockService.ProcessUpdateCalled {
				t.Error("expected ProcessUpdate to be called")
			}
			if mockService.LastUpdate.UpdateID != tc.updateID {
				t.Errorf("expected update_id %d, got %d", tc.updateID, mockService.LastUpdate.UpdateID)
			}
		})
	}
}

func TestHandleIngest_LargeFile(t *testing.T) {
	handler, mockService, _ := setupIngestHandlerTest(t)

	updateJSON := `{
		"update_id": 12349,
		"message": {
			"message_id": 5,
			"chat": {"id": -1003280306634, "type": "supergroup"},
			"from": {"id": 789, "first_name": "TestUser"},
			"date": 1733611200,
			"text": "Large file test"
		}
	}`

	// Create 1MB file
	largeFile := make([]byte, 1024*1024)
	for i := range largeFile {
		largeFile[i] = byte(i % 256)
	}

	files := map[string][]byte{
		"large_file": largeFile,
	}

	req := createMultipartRequest(t, updateJSON, files)
	rr := httptest.NewRecorder()

	handler.HandleIngest(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Verify large file was extracted correctly
	if data, ok := mockService.LastFiles["large_file"]; !ok {
		t.Error("expected large_file in files map")
	} else if len(data) != len(largeFile) {
		t.Errorf("expected file size %d, got %d", len(largeFile), len(data))
	}
}

// =============================================================================
// Integration with Authentication Middleware Tests
// =============================================================================

func TestHandleIngest_WithAuthMiddleware_ValidKey(t *testing.T) {
	handler, mockService, _ := setupIngestHandlerTest(t)

	// Setup auth middleware
	appKeys := map[string]string{
		"telegram-bot": "test-api-key-12345",
	}
	auth := middleware.NewMultiAPIKeyAuth(appKeys)

	// Wrap handler with auth
	authenticatedHandler := auth.Authenticate(http.HandlerFunc(handler.HandleIngest))

	updateJSON := `{
		"update_id": 12350,
		"message": {
			"message_id": 6,
			"chat": {"id": -1003280306634, "type": "supergroup"},
			"from": {"id": 789, "first_name": "TestUser"},
			"date": 1733611200,
			"text": "Auth test"
		}
	}`

	req := createMultipartRequest(t, updateJSON, nil)
	req.Header.Set("Authorization", "Bearer test-api-key-12345")

	rr := httptest.NewRecorder()
	authenticatedHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if !mockService.ProcessUpdateCalled {
		t.Error("expected ProcessUpdate to be called")
	}
}

func TestHandleIngest_WithAuthMiddleware_MissingHeader(t *testing.T) {
	handler, mockService, _ := setupIngestHandlerTest(t)

	// Setup auth middleware
	appKeys := map[string]string{
		"telegram-bot": "test-api-key-12345",
	}
	auth := middleware.NewMultiAPIKeyAuth(appKeys)

	// Wrap handler with auth
	authenticatedHandler := auth.Authenticate(http.HandlerFunc(handler.HandleIngest))

	updateJSON := `{
		"update_id": 12351,
		"message": {
			"message_id": 7,
			"chat": {"id": -1003280306634, "type": "supergroup"},
			"from": {"id": 789, "first_name": "TestUser"},
			"date": 1733611200,
			"text": "Auth test"
		}
	}`

	req := createMultipartRequest(t, updateJSON, nil)
	// NO Authorization header

	rr := httptest.NewRecorder()
	authenticatedHandler.ServeHTTP(rr, req)

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

	// Verify handler was NOT called
	if mockService.ProcessUpdateCalled {
		t.Error("expected ProcessUpdate to NOT be called (auth failed)")
	}
}

func TestHandleIngest_WithAuthMiddleware_InvalidKey(t *testing.T) {
	handler, mockService, _ := setupIngestHandlerTest(t)

	// Setup auth middleware
	appKeys := map[string]string{
		"telegram-bot": "test-api-key-12345",
	}
	auth := middleware.NewMultiAPIKeyAuth(appKeys)

	// Wrap handler with auth
	authenticatedHandler := auth.Authenticate(http.HandlerFunc(handler.HandleIngest))

	updateJSON := `{
		"update_id": 12352,
		"message": {
			"message_id": 8,
			"chat": {"id": -1003280306634, "type": "supergroup"},
			"from": {"id": 789, "first_name": "TestUser"},
			"date": 1733611200,
			"text": "Auth test"
		}
	}`

	req := createMultipartRequest(t, updateJSON, nil)
	req.Header.Set("Authorization", "Bearer wrong-invalid-key")

	rr := httptest.NewRecorder()
	authenticatedHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "invalid api key" {
		t.Errorf("expected error 'invalid api key', got '%s'", resp["error"])
	}

	// Verify handler was NOT called
	if mockService.ProcessUpdateCalled {
		t.Error("expected ProcessUpdate to NOT be called (auth failed)")
	}
}

// =============================================================================
// extractFiles Method Tests
// =============================================================================

func TestExtractFiles_NoFiles(t *testing.T) {
	handler, _, _ := setupIngestHandlerTest(t)

	updateJSON := `{"update_id": 1}`
	req := createMultipartRequest(t, updateJSON, nil)

	// Parse the form first
	if err := req.ParseMultipartForm(100 * 1024 * 1024); err != nil {
		t.Fatalf("failed to parse multipart form: %v", err)
	}

	files := handler.extractFiles(req)

	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestExtractFiles_SingleFile(t *testing.T) {
	handler, _, _ := setupIngestHandlerTest(t)

	updateJSON := `{"update_id": 1}`
	fileData := map[string][]byte{
		"testfile": []byte("test data"),
	}

	req := createMultipartRequest(t, updateJSON, fileData)

	// Parse the form first
	if err := req.ParseMultipartForm(100 * 1024 * 1024); err != nil {
		t.Fatalf("failed to parse multipart form: %v", err)
	}

	files := handler.extractFiles(req)

	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
	if data, ok := files["testfile"]; !ok {
		t.Error("expected testfile in files map")
	} else if string(data) != "test data" {
		t.Errorf("expected 'test data', got '%s'", string(data))
	}
}

func TestExtractFiles_MultipleFiles(t *testing.T) {
	handler, _, _ := setupIngestHandlerTest(t)

	updateJSON := `{"update_id": 1}`
	fileData := map[string][]byte{
		"file1": []byte("data1"),
		"file2": []byte("data2"),
		"file3": []byte("data3"),
	}

	req := createMultipartRequest(t, updateJSON, fileData)

	// Parse the form first
	if err := req.ParseMultipartForm(100 * 1024 * 1024); err != nil {
		t.Fatalf("failed to parse multipart form: %v", err)
	}

	files := handler.extractFiles(req)

	if len(files) != 3 {
		t.Errorf("expected 3 files, got %d", len(files))
	}
	for fileID, expectedData := range fileData {
		if data, ok := files[fileID]; !ok {
			t.Errorf("expected %s in files map", fileID)
		} else if !bytes.Equal(data, expectedData) {
			t.Errorf("file %s: expected %v, got %v", fileID, expectedData, data)
		}
	}
}

func TestExtractFiles_NilMultipartForm(t *testing.T) {
	handler, _, _ := setupIngestHandlerTest(t)

	// Create request without multipart form
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", strings.NewReader(""))

	files := handler.extractFiles(req)

	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}
