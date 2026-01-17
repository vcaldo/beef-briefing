package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"beef-briefing/apps/api-service/internal/services"
	"beef-briefing/pkg/config"
)

// TestHandleGetMessages_ValidRequest tests successful GET /messages endpoint.
func TestHandleGetMessages_ValidRequest(t *testing.T) {
	mockService := newMockMLService()
	mockService.setMessages([]services.MLMessage{
		{
			ID:        1,
			MessageID: 100,
			ChatID:    -1001234567890,
			UserID:    ptrInt64(123),
			Text:      "Test message 1",
		},
		{
			ID:        2,
			MessageID: 101,
			ChatID:    -1001234567890,
			UserID:    ptrInt64(124),
			Text:      "Test message 2",
		},
	}, false)

	handler := NewMLHandler(mockService, &config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ml/messages?limit=100", nil)
	w := httptest.NewRecorder()

	handler.HandleGetMessages(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response services.GetMessagesResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(response.Messages))
	}

	if response.HasMore {
		t.Error("expected HasMore=false, got true")
	}

	if !mockService.getUnprocessedMessagesCalled {
		t.Error("expected GetUnprocessedMessages to be called")
	}

	if mockService.lastGetUnprocessedLimit != 100 {
		t.Errorf("expected limit=100, got %d", mockService.lastGetUnprocessedLimit)
	}
}

// TestHandleGetMessages_DefaultLimit tests GET /messages with default limit.
func TestHandleGetMessages_DefaultLimit(t *testing.T) {
	mockService := newMockMLService()
	mockService.setMessages([]services.MLMessage{}, false)

	handler := NewMLHandler(mockService, &config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ml/messages", nil)
	w := httptest.NewRecorder()

	handler.HandleGetMessages(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Default limit should be 500
	if mockService.lastGetUnprocessedLimit != 500 {
		t.Errorf("expected default limit=500, got %d", mockService.lastGetUnprocessedLimit)
	}
}

// TestHandleGetMessages_InvalidLimit tests GET /messages with invalid limit.
func TestHandleGetMessages_InvalidLimit(t *testing.T) {
	mockService := newMockMLService()
	mockService.setMessages([]services.MLMessage{}, false)

	handler := NewMLHandler(mockService, &config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ml/messages?limit=invalid", nil)
	w := httptest.NewRecorder()

	handler.HandleGetMessages(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Invalid limit should use default 500
	if mockService.lastGetUnprocessedLimit != 500 {
		t.Errorf("expected default limit=500 for invalid input, got %d", mockService.lastGetUnprocessedLimit)
	}
}

// TestHandleGetMessages_ServiceError tests GET /messages with service error.
func TestHandleGetMessages_ServiceError(t *testing.T) {
	mockService := newMockMLService()
	mockService.setGetUnprocessedMessagesError(errors.New("database connection failed"))

	handler := NewMLHandler(mockService, &config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ml/messages", nil)
	w := httptest.NewRecorder()

	handler.HandleGetMessages(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if response["error"] == nil {
		t.Error("expected error field in response")
	}
}

// TestHandlePostResults_ValidRequest tests successful POST /results endpoint.
func TestHandlePostResults_ValidRequest(t *testing.T) {
	mockService := newMockMLService()

	handler := NewMLHandler(mockService, &config.Config{})

	requestBody := services.SaveResultsRequest{
		Results: []services.MLResultRequest{
			{
				MessageID: 100,
				ChatID:    -1001234567890,
				Sentiment: &services.SentimentInput{
					Label: "positive",
					Scores: services.SentimentScore{
						Positive: 0.8,
						Neutral:  0.15,
						Negative: 0.05,
					},
				},
			},
		},
		ProcessorVersion: "v1.0.0",
	}

	body, _ := json.Marshal(requestBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ml/results", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandlePostResults(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if !mockService.saveResultsCalled {
		t.Error("expected SaveResults to be called")
	}

	if mockService.lastSaveRequest == nil {
		t.Fatal("expected LastSaveRequest to be set")
	}

	if len(mockService.lastSaveRequest.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(mockService.lastSaveRequest.Results))
	}

	if mockService.lastSaveRequest.ProcessorVersion != "v1.0.0" {
		t.Errorf("expected processor_version=v1.0.0, got %s", mockService.lastSaveRequest.ProcessorVersion)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", response["status"])
	}

	if int(response["saved"].(float64)) != 1 {
		t.Errorf("expected saved=1, got %v", response["saved"])
	}
}

// TestHandlePostResults_EmptyResults tests POST /results with empty results array.
func TestHandlePostResults_EmptyResults(t *testing.T) {
	mockService := newMockMLService()

	handler := NewMLHandler(mockService, &config.Config{})

	requestBody := services.SaveResultsRequest{
		Results:          []services.MLResultRequest{},
		ProcessorVersion: "v1.0.0",
	}

	body, _ := json.Marshal(requestBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ml/results", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandlePostResults(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if response["error"] != "results array is empty" {
		t.Errorf("expected error='results array is empty', got %v", response["error"])
	}

	if mockService.saveResultsCalled {
		t.Error("expected SaveResults NOT to be called for empty results")
	}
}

// TestHandlePostResults_InvalidJSON tests POST /results with invalid JSON.
func TestHandlePostResults_InvalidJSON(t *testing.T) {
	mockService := newMockMLService()

	handler := NewMLHandler(mockService, &config.Config{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ml/results", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandlePostResults(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if response["error"] != "invalid request body" {
		t.Errorf("expected error='invalid request body', got %v", response["error"])
	}

	if mockService.saveResultsCalled {
		t.Error("expected SaveResults NOT to be called for invalid JSON")
	}
}

// TestHandlePostResults_ServiceError tests POST /results with service error.
func TestHandlePostResults_ServiceError(t *testing.T) {
	mockService := newMockMLService()
	mockService.setSaveResultsError(errors.New("database write failed"))

	handler := NewMLHandler(mockService, &config.Config{})

	requestBody := services.SaveResultsRequest{
		Results: []services.MLResultRequest{
			{
				MessageID: 100,
				ChatID:    -1001234567890,
			},
		},
		ProcessorVersion: "v1.0.0",
	}

	body, _ := json.Marshal(requestBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ml/results", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandlePostResults(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if response["error"] == nil {
		t.Error("expected error field in response")
	}
}

// TestHandlePostResults_DefaultProcessorVersion tests POST /results with missing processor version.
func TestHandlePostResults_DefaultProcessorVersion(t *testing.T) {
	mockService := newMockMLService()

	handler := NewMLHandler(mockService, &config.Config{})

	requestBody := services.SaveResultsRequest{
		Results: []services.MLResultRequest{
			{
				MessageID: 100,
				ChatID:    -1001234567890,
			},
		},
		ProcessorVersion: "", // Empty version should default to "unknown"
	}

	body, _ := json.Marshal(requestBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ml/results", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.HandlePostResults(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if mockService.lastSaveRequest == nil {
		t.Fatal("expected LastSaveRequest to be set")
	}

	if mockService.lastSaveRequest.ProcessorVersion != "unknown" {
		t.Errorf("expected default processor_version=unknown, got %s", mockService.lastSaveRequest.ProcessorVersion)
	}
}

// TestHandleGetStatus_ValidRequest tests successful GET /status endpoint.
func TestHandleGetStatus_ValidRequest(t *testing.T) {
	mockService := newMockMLService()
	mockService.setStats(&services.ProcessingStats{
		TotalWithText:  1000,
		Processed:      800,
		Unprocessed:    200,
		SentimentCount: 750,
		ToxicityCount:  800,
		ToxicMessages:  50,
		HumorCount:     600,
		QuestionCount:  500,
		NERCount:       700,
		TopicCount:     0,
	})

	handler := NewMLHandler(mockService, &config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ml/status", nil)
	w := httptest.NewRecorder()

	handler.HandleGetStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if !mockService.getProcessingStatsCalled {
		t.Error("expected GetProcessingStats to be called")
	}

	var response services.ProcessingStats
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.TotalWithText != 1000 {
		t.Errorf("expected TotalWithText=1000, got %d", response.TotalWithText)
	}

	if response.Processed != 800 {
		t.Errorf("expected Processed=800, got %d", response.Processed)
	}

	if response.Unprocessed != 200 {
		t.Errorf("expected Unprocessed=200, got %d", response.Unprocessed)
	}

	if response.ToxicMessages != 50 {
		t.Errorf("expected ToxicMessages=50, got %d", response.ToxicMessages)
	}
}

// TestHandleGetStatus_EmptyStats tests GET /status with empty stats.
func TestHandleGetStatus_EmptyStats(t *testing.T) {
	mockService := newMockMLService()
	mockService.setStats(&services.ProcessingStats{
		TotalWithText:  0,
		Processed:      0,
		Unprocessed:    0,
		SentimentCount: 0,
		ToxicityCount:  0,
		ToxicMessages:  0,
		HumorCount:     0,
		QuestionCount:  0,
		NERCount:       0,
		TopicCount:     0,
	})

	handler := NewMLHandler(mockService, &config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ml/status", nil)
	w := httptest.NewRecorder()

	handler.HandleGetStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response services.ProcessingStats
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.TotalWithText != 0 {
		t.Errorf("expected TotalWithText=0, got %d", response.TotalWithText)
	}

	if response.Processed != 0 {
		t.Errorf("expected Processed=0, got %d", response.Processed)
	}
}

// TestHandleGetStatus_ServiceError tests GET /status with service error.
func TestHandleGetStatus_ServiceError(t *testing.T) {
	mockService := newMockMLService()
	mockService.setGetProcessingStatsError(errors.New("database query failed"))

	handler := NewMLHandler(mockService, &config.Config{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ml/status", nil)
	w := httptest.NewRecorder()

	handler.HandleGetStatus(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if response["error"] == nil {
		t.Error("expected error field in response")
	}
}

// Helper function to create a pointer to int64
func ptrInt64(v int64) *int64 {
	return &v
}
