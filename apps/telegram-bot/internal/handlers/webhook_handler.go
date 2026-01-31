package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"beef-briefing/apps/telegram-bot/internal/client"

	"github.com/go-telegram/bot"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// WebhookHandler handles internal webhook requests for updating match messages
type WebhookHandler struct {
	apiClient *client.APIClient
	bot       *bot.Bot
	nrApp     *newrelic.Application
}

// NewWebhookHandler creates a new WebhookHandler
func NewWebhookHandler(apiClient *client.APIClient, b *bot.Bot, nrApp *newrelic.Application) *WebhookHandler {
	return &WebhookHandler{
		apiClient: apiClient,
		bot:       b,
		nrApp:     nrApp,
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
	// Start New Relic transaction
	var txn *newrelic.Transaction
	if h.nrApp != nil {
		txn = h.nrApp.StartTransaction("bot:webhook-update-match")
		defer txn.End()
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UpdateMatchMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("failed to decode webhook request", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.MatchID == "" || req.ChatID == 0 || req.TelegramMessageID == 0 {
		slog.Warn("webhook request missing required fields", "match_id", req.MatchID, "chat_id", req.ChatID, "telegram_message_id", req.TelegramMessageID)
		http.Error(w, "match_id, chat_id, and telegram_message_id are required", http.StatusBadRequest)
		return
	}

	// Add attributes after parsing request
	if txn != nil {
		txn.AddAttribute("match_id", req.MatchID)
		txn.AddAttribute("chat_id", req.ChatID)
		txn.AddAttribute("telegram_message_id", req.TelegramMessageID)
	}

	slog.Info("received update match message webhook", "match_id", req.MatchID, "chat_id", req.ChatID, "telegram_message_id", req.TelegramMessageID)

	// Create context with transaction for downstream calls
	ctx := r.Context()
	if txn != nil {
		ctx = newrelic.NewContext(ctx, txn)
	}

	// Track API call segment
	var apiSegment *newrelic.Segment
	if txn != nil {
		apiSegment = txn.StartSegment("webhook:get-match")
	}
	match, err := h.apiClient.GetArenaMatch(ctx, req.MatchID)
	if apiSegment != nil {
		apiSegment.End()
	}
	if err != nil {
		if txn != nil {
			txn.NoticeError(err)
		}
		slog.Error("failed to get match for webhook update", "match_id", req.MatchID, "error", err)
		http.Error(w, "failed to get match", http.StatusInternalServerError)
		return
	}

	// Build keyboard using shared function and update the message
	keyboard := BuildMatchKeyboard(match.ID, match.Participants)

	// Track bot update segment
	var botSegment *newrelic.Segment
	if txn != nil {
		botSegment = txn.StartSegment("webhook:edit-message")
	}
	_, err = h.bot.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
		ChatID:      req.ChatID,
		MessageID:   int(req.TelegramMessageID),
		ReplyMarkup: keyboard,
	})
	if botSegment != nil {
		botSegment.End()
	}
	if err != nil {
		if txn != nil {
			txn.NoticeError(err)
		}
		slog.Error("failed to update match message via webhook", "match_id", req.MatchID, "chat_id", req.ChatID, "telegram_message_id", req.TelegramMessageID, "error", err)
		// Don't return error - the update attempt was made
	} else {
		slog.Info("updated match message via webhook", "match_id", req.MatchID, "chat_id", req.ChatID, "participant_count", len(match.Participants))
	}

	// Add success attribute
	if txn != nil {
		txn.AddAttribute("participant_count", len(match.Participants))
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}
