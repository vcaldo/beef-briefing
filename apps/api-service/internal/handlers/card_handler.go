package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"beef-briefing/apps/api-service/internal/services"
	"beef-briefing/pkg/config"

	"github.com/gorilla/mux"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// CardHandler handles HTTP requests for user card endpoints.
type CardHandler struct {
	cardService *services.CardService
	config      *config.Config
}

// NewCardHandler creates a new CardHandler.
func NewCardHandler(cardService *services.CardService, cfg *config.Config) *CardHandler {
	return &CardHandler{
		cardService: cardService,
		config:      cfg,
	}
}

// respondJSON writes a JSON response.
func (h *CardHandler) respondJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError writes a JSON error response.
func (h *CardHandler) respondError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// HandleGetUserCard handles GET /api/v1/cards/{user_id}
func (h *CardHandler) HandleGetUserCard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	// Extract path parameter
	vars := mux.Vars(r)
	userIDStr := vars["user_id"]
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		h.respondError(w, "invalid user_id", http.StatusBadRequest)
		return
	}

	// Extract required chat_id query parameter
	chatIDStr := r.URL.Query().Get("chat_id")
	if chatIDStr == "" {
		h.respondError(w, "chat_id is required", http.StatusBadRequest)
		return
	}
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		h.respondError(w, "invalid chat_id", http.StatusBadRequest)
		return
	}

	// Optional week parameter
	var weekStart *time.Time
	weekStr := r.URL.Query().Get("week")
	if weekStr != "" {
		parsed, err := time.Parse("2006-01-02", weekStr)
		if err != nil {
			h.respondError(w, "invalid week format (use YYYY-MM-DD)", http.StatusBadRequest)
			return
		}
		weekStart = &parsed
	}

	if txn != nil {
		txn.AddAttribute("user_id", userID)
		txn.AddAttribute("chat_id", chatID)
	}

	// Get card from service
	card, user, err := h.cardService.GetUserCard(ctx, userID, chatID, weekStart)
	if err != nil {
		if errors.Is(err, services.ErrCardNotFound) {
			h.respondError(w, "card not found", http.StatusNotFound)
			return
		}
		slog.Error("failed to get user card", "user_id", userID, "chat_id", chatID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		h.respondError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"card": card,
		"user": user,
	}

	h.respondJSON(w, response, http.StatusOK)
}

// HandleGetChatCards handles GET /api/v1/cards
func (h *CardHandler) HandleGetChatCards(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	// Parse required chat_id query parameter
	chatIDStr := r.URL.Query().Get("chat_id")
	if chatIDStr == "" {
		h.respondError(w, "chat_id is required", http.StatusBadRequest)
		return
	}
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		h.respondError(w, "invalid chat_id", http.StatusBadRequest)
		return
	}

	// Optional week parameter
	var weekStart *time.Time
	if weekStr := r.URL.Query().Get("week"); weekStr != "" {
		parsed, err := time.Parse("2006-01-02", weekStr)
		if err != nil {
			h.respondError(w, "invalid week format (use YYYY-MM-DD)", http.StatusBadRequest)
			return
		}
		weekStart = &parsed
	}

	// Sort parameters
	sortBy := r.URL.Query().Get("sort_by")
	if sortBy == "" {
		sortBy = "mood"
	}

	order := r.URL.Query().Get("order")
	if order == "" {
		order = "desc"
	}

	// Pagination parameters
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("sort_by", sortBy)
	}

	// Get cards from service
	result, err := h.cardService.GetChatCards(ctx, services.GetChatCardsRequest{
		ChatID:    chatID,
		WeekStart: weekStart,
		SortBy:    sortBy,
		Order:     order,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		slog.Error("failed to get chat cards", "chat_id", chatID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		h.respondError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.respondJSON(w, result, http.StatusOK)
}

// HandleGetUserHistory handles GET /api/v1/cards/{user_id}/history
func (h *CardHandler) HandleGetUserHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	// Extract path parameter
	vars := mux.Vars(r)
	userIDStr := vars["user_id"]
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		h.respondError(w, "invalid user_id", http.StatusBadRequest)
		return
	}

	// Required chat_id query parameter
	chatIDStr := r.URL.Query().Get("chat_id")
	if chatIDStr == "" {
		h.respondError(w, "chat_id is required", http.StatusBadRequest)
		return
	}
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		h.respondError(w, "invalid chat_id", http.StatusBadRequest)
		return
	}

	// Optional limit parameter
	limit := 12
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 52 {
			limit = parsed
		}
	}

	if txn != nil {
		txn.AddAttribute("user_id", userID)
		txn.AddAttribute("chat_id", chatID)
	}

	// Get history from service
	result, err := h.cardService.GetUserHistory(ctx, userID, chatID, limit)
	if err != nil {
		slog.Error("failed to get user history", "user_id", userID, "chat_id", chatID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		h.respondError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.respondJSON(w, result, http.StatusOK)
}

// HandleGetAvailableWeeks handles GET /api/v1/cards/weeks
func (h *CardHandler) HandleGetAvailableWeeks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	// Required chat_id query parameter
	chatIDStr := r.URL.Query().Get("chat_id")
	if chatIDStr == "" {
		h.respondError(w, "chat_id is required", http.StatusBadRequest)
		return
	}
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		h.respondError(w, "invalid chat_id", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
	}

	weeks, err := h.cardService.GetAvailableWeeks(ctx, chatID)
	if err != nil {
		slog.Error("failed to get available weeks", "chat_id", chatID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		h.respondError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.respondJSON(w, weeks, http.StatusOK)
}
