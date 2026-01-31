package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"beef-briefing/apps/telegram-bot/internal/client"

	"github.com/go-telegram/bot"
)

// WebhookHandler handles internal webhook requests for updating match messages
type WebhookHandler struct {
	apiClient *client.APIClient
	bot       *bot.Bot
}

// NewWebhookHandler creates a new WebhookHandler
func NewWebhookHandler(apiClient *client.APIClient, b *bot.Bot) *WebhookHandler {
	return &WebhookHandler{
		apiClient: apiClient,
		bot:       b,
	}
}

// UpdateMatchMessageRequest is the request body for updating a match message
type UpdateMatchMessageRequest struct {
	MatchID           string `json:"match_id"`
	ChatID            int64  `json:"chat_id"`
	TelegramMessageID int64  `json:"telegram_message_id"`
}

// HandleUpdateMatchMessage handles POST /internal/update-match-message
// This endpoint is called by the API service when a participant joins/leaves via Mini App
func (h *WebhookHandler) HandleUpdateMatchMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UpdateMatchMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("failed to decode webhook request", "error", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.MatchID == "" || req.ChatID == 0 || req.TelegramMessageID == 0 {
		slog.Warn("webhook request missing required fields", "match_id", req.MatchID, "chat_id", req.ChatID, "telegram_message_id", req.TelegramMessageID)
		http.Error(w, "match_id, chat_id, and telegram_message_id are required", http.StatusBadRequest)
		return
	}

	slog.Info("received update match message webhook", "match_id", req.MatchID, "chat_id", req.ChatID, "telegram_message_id", req.TelegramMessageID)

	// Fetch fresh match data from API
	ctx := context.Background()
	match, err := h.apiClient.GetArenaMatch(ctx, req.MatchID)
	if err != nil {
		slog.Error("failed to get match for webhook update", "match_id", req.MatchID, "error", err)
		http.Error(w, "failed to get match", http.StatusInternalServerError)
		return
	}

	// Build keyboard using shared function and update the message
	keyboard := BuildMatchKeyboard(match.ID, match.Participants)

	_, err = h.bot.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
		ChatID:      req.ChatID,
		MessageID:   int(req.TelegramMessageID),
		ReplyMarkup: keyboard,
	})
	if err != nil {
		slog.Error("failed to update match message via webhook", "match_id", req.MatchID, "chat_id", req.ChatID, "telegram_message_id", req.TelegramMessageID, "error", err)
		// Don't return error - the update attempt was made
	} else {
		slog.Info("updated match message via webhook", "match_id", req.MatchID, "chat_id", req.ChatID, "participant_count", len(match.Participants))
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}
