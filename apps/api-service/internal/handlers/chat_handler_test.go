package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gorilla/mux"

	"beef-briefing/apps/api-service/internal/testutil"
)

func TestGetChatInfo_ValidRequest(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	chatID := int64(-1002345678901)

	// Clean up any existing test data first
	_, _ = tdb.DB.Exec(`DELETE FROM chats WHERE id = $1`, chatID)
	defer func() {
		_, _ = tdb.DB.Exec(`DELETE FROM chats WHERE id = $1`, chatID)
	}()

	// Insert test chat
	_, err := tdb.DB.Exec(`
		INSERT INTO chats (id, type, timezone, ranked_tournaments_enabled)
		VALUES ($1, $2, $3, $4)
	`, chatID, "supergroup", "America/New_York", true)
	if err != nil {
		t.Fatalf("failed to insert test chat: %v", err)
	}

	handler := NewChatHandler(tdb.DB)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/-1002345678901", nil)
	req = mux.SetURLVars(req, map[string]string{"chat_id": "-1002345678901"})
	w := httptest.NewRecorder()

	handler.GetChatInfo(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response ChatInfo
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.ChatID != -1002345678901 {
		t.Errorf("expected chat_id -1002345678901, got %d", response.ChatID)
	}
	if response.Timezone != "America/New_York" {
		t.Errorf("expected timezone America/New_York, got %s", response.Timezone)
	}
	if !response.RankedTournamentsEnabled {
		t.Error("expected ranked_tournaments_enabled to be true")
	}
}

func TestGetChatInfo_ChatNotFound(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	handler := NewChatHandler(tdb.DB)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/-1002345678901", nil)
	req = mux.SetURLVars(req, map[string]string{"chat_id": "-1002345678901"})
	w := httptest.NewRecorder()

	handler.GetChatInfo(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	expectedBody := "Chat not found\n"
	if w.Body.String() != expectedBody {
		t.Errorf("expected body %q, got %q", expectedBody, w.Body.String())
	}
}

func TestGetChatInfo_InvalidChatID(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	handler := NewChatHandler(tdb.DB)

	testCases := []struct {
		name   string
		chatID string
	}{
		{"non-numeric", "abc"},
		{"empty", ""},
		{"special chars", "!@#$"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/"+tc.chatID, nil)
			req = mux.SetURLVars(req, map[string]string{"chat_id": tc.chatID})
			w := httptest.NewRecorder()

			handler.GetChatInfo(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d", w.Code)
			}

			expectedBody := "Invalid chat ID\n"
			if w.Body.String() != expectedBody {
				t.Errorf("expected body %q, got %q", expectedBody, w.Body.String())
			}
		})
	}
}

func TestGetChatInfo_NullTimezone(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	// Clean up test data
	defer func() {
		_, _ = tdb.DB.Exec(`DELETE FROM chats WHERE id = -1002345678902`)
	}()

	// Insert test chat with NULL timezone
	_, err := tdb.DB.Exec(`
		INSERT INTO chats (id, type, timezone, ranked_tournaments_enabled)
		VALUES ($1, $2, NULL, $3)
	`, -1002345678902, "supergroup", false)
	if err != nil {
		t.Fatalf("failed to insert test chat: %v", err)
	}

	handler := NewChatHandler(tdb.DB)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/-1002345678902", nil)
	req = mux.SetURLVars(req, map[string]string{"chat_id": "-1002345678902"})
	w := httptest.NewRecorder()

	handler.GetChatInfo(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response ChatInfo
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Timezone != "" {
		t.Errorf("expected empty timezone for NULL value, got %s", response.Timezone)
	}
	if response.RankedTournamentsEnabled {
		t.Error("expected ranked_tournaments_enabled to be false")
	}
}

func TestGetChatInfo_RankedTournamentsDisabled(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	// Clean up test data
	defer func() {
		_, _ = tdb.DB.Exec(`DELETE FROM chats WHERE id = -1002345678903`)
	}()

	// Insert test chat with ranked tournaments disabled
	_, err := tdb.DB.Exec(`
		INSERT INTO chats (id, type, timezone, ranked_tournaments_enabled)
		VALUES ($1, $2, $3, $4)
	`, -1002345678903, "supergroup", "UTC", false)
	if err != nil {
		t.Fatalf("failed to insert test chat: %v", err)
	}

	handler := NewChatHandler(tdb.DB)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/-1002345678903", nil)
	req = mux.SetURLVars(req, map[string]string{"chat_id": "-1002345678903"})
	w := httptest.NewRecorder()

	handler.GetChatInfo(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response ChatInfo
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.RankedTournamentsEnabled {
		t.Error("expected ranked_tournaments_enabled to be false")
	}
}

func TestGetChatInfo_ResponseContentType(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	// Clean up test data
	defer func() {
		_, _ = tdb.DB.Exec(`DELETE FROM chats WHERE id = -1002345678904`)
	}()

	// Insert test chat
	_, err := tdb.DB.Exec(`
		INSERT INTO chats (id, type, timezone, ranked_tournaments_enabled)
		VALUES ($1, $2, $3, $4)
	`, -1002345678904, "supergroup", "UTC", true)
	if err != nil {
		t.Fatalf("failed to insert test chat: %v", err)
	}

	handler := NewChatHandler(tdb.DB)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/-1002345678904", nil)
	req = mux.SetURLVars(req, map[string]string{"chat_id": "-1002345678904"})
	w := httptest.NewRecorder()

	handler.GetChatInfo(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}
}

func TestGetChatInfo_PrivateChat(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	chatID := int64(123456789)

	// Clean up any existing test data first
	_, _ = tdb.DB.Exec(`DELETE FROM chats WHERE id = $1`, chatID)
	defer func() {
		_, _ = tdb.DB.Exec(`DELETE FROM chats WHERE id = $1`, chatID)
	}()

	// Insert test private chat (positive chat_id)
	_, err := tdb.DB.Exec(`
		INSERT INTO chats (id, type, timezone, ranked_tournaments_enabled)
		VALUES ($1, $2, $3, $4)
	`, chatID, "private", "Asia/Tokyo", false)
	if err != nil {
		t.Fatalf("failed to insert test chat: %v", err)
	}

	handler := NewChatHandler(tdb.DB)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/123456789", nil)
	req = mux.SetURLVars(req, map[string]string{"chat_id": "123456789"})
	w := httptest.NewRecorder()

	handler.GetChatInfo(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response ChatInfo
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.ChatID != 123456789 {
		t.Errorf("expected chat_id 123456789, got %d", response.ChatID)
	}
	if response.Timezone != "Asia/Tokyo" {
		t.Errorf("expected timezone Asia/Tokyo, got %s", response.Timezone)
	}
}

func TestGetChatInfo_MultipleChats(t *testing.T) {
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	// Clean up test data
	defer func() {
		_, _ = tdb.DB.Exec(`DELETE FROM chats WHERE id IN (-1002345678905, -1002345678906, -1002345678907)`)
	}()

	// Insert multiple test chats
	chats := []struct {
		id      int64
		tz      string
		ranked  bool
	}{
		{-1002345678905, "America/New_York", true},
		{-1002345678906, "Europe/London", false},
		{-1002345678907, "Asia/Tokyo", true},
	}

	for _, chat := range chats {
		_, err := tdb.DB.Exec(`
			INSERT INTO chats (id, type, timezone, ranked_tournaments_enabled)
			VALUES ($1, $2, $3, $4)
		`, chat.id, "supergroup", chat.tz, chat.ranked)
		if err != nil {
			t.Fatalf("failed to insert test chat: %v", err)
		}
	}

	handler := NewChatHandler(tdb.DB)

	// Test each chat independently
	for _, chat := range chats {
		t.Run(chat.tz, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/"+strconv.FormatInt(chat.id, 10), nil)
			req = mux.SetURLVars(req, map[string]string{"chat_id": strconv.FormatInt(chat.id, 10)})
			w := httptest.NewRecorder()

			handler.GetChatInfo(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}

			var response ChatInfo
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if response.ChatID != chat.id {
				t.Errorf("expected chat_id %d, got %d", chat.id, response.ChatID)
			}
			if response.Timezone != chat.tz {
				t.Errorf("expected timezone %s, got %s", chat.tz, response.Timezone)
			}
			if response.RankedTournamentsEnabled != chat.ranked {
				t.Errorf("expected ranked_tournaments_enabled %t, got %t", chat.ranked, response.RankedTournamentsEnabled)
			}
		})
	}
}
