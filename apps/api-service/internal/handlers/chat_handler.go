package handlers

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// ChatHandler handles HTTP requests for chat endpoints
type ChatHandler struct {
	db *sql.DB
}

// NewChatHandler creates a new ChatHandler
func NewChatHandler(db *sql.DB) *ChatHandler {
	return &ChatHandler{
		db: db,
	}
}

// ChatInfo represents chat configuration information
type ChatInfo struct {
	ChatID                   int64  `json:"chat_id"`
	Timezone                 string `json:"timezone"`
	RankedTournamentsEnabled bool   `json:"ranked_tournaments_enabled"`
}

// GetChatInfo handles GET /api/v1/chat/{chat_id}
func (h *ChatHandler) GetChatInfo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chatIDStr := vars["chat_id"]

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		slog.Warn("invalid chat_id parameter", "chat_id", chatIDStr, "error", err)
		http.Error(w, "Invalid chat ID", http.StatusBadRequest)
		return
	}

	var timezone sql.NullString
	var rankedEnabled bool

	err = h.db.QueryRowContext(
		r.Context(),
		`SELECT id, timezone, ranked_tournaments_enabled
         FROM chats
         WHERE id = $1`,
		chatID,
	).Scan(&chatID, &timezone, &rankedEnabled)

	if err == sql.ErrNoRows {
		slog.Debug("chat not found", "chat_id", chatID)
		http.Error(w, "Chat not found", http.StatusNotFound)
		return
	}
	if err != nil {
		slog.Error("failed to query chat", "chat_id", chatID, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := ChatInfo{
		ChatID:                   chatID,
		Timezone:                 timezone.String,
		RankedTournamentsEnabled: rankedEnabled,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}
