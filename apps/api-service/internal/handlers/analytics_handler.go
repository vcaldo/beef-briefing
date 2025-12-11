package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"beef-briefing/apps/api-service/internal/models"
	"beef-briefing/apps/api-service/internal/services"

	"github.com/gorilla/mux"
	"github.com/newrelic/go-agent/v3/newrelic"
)

type AnalyticsHandler struct {
	analyticsService *services.AnalyticsService
}

func NewAnalyticsHandler(analyticsService *services.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{
		analyticsService: analyticsService,
	}
}

// Helper to parse chat ID from URL
func (h *AnalyticsHandler) parseChatID(r *http.Request) (int64, error) {
	vars := mux.Vars(r)
	chatID, err := strconv.ParseInt(vars["chat_id"], 10, 64)
	if err != nil {
		return 0, err
	}
	return chatID, nil
}

// Helper to parse and validate time range from query params
func (h *AnalyticsHandler) parseTimeRange(r *http.Request) (time.Time, time.Time, error) {
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	if startDateStr == "" || endDateStr == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("start_date and end_date are required")
	}

	startDate, err := time.Parse(time.RFC3339, startDateStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start_date format, use RFC3339")
	}

	endDate, err := time.Parse(time.RFC3339, endDateStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid end_date format, use RFC3339")
	}

	// Validate using TimeRangeRequest
	tr := models.TimeRangeRequest{StartDate: startDate, EndDate: endDate}
	if err := tr.Validate(); err != nil {
		return time.Time{}, time.Time{}, err
	}

	return startDate, endDate, nil
}

// Helper to write standard JSON response
func (h *AnalyticsHandler) writeResponse(w http.ResponseWriter, chatID int64, startDate, endDate time.Time, data interface{}) {
	response := models.StandardResponse{
		Data: data,
		Metadata: models.Metadata{
			ChatID:    chatID,
			StartDate: startDate,
			EndDate:   endDate,
			Generated: time.Now(),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

// Helper to write error response
func (h *AnalyticsHandler) writeError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// HandleOverview - GET /api/v1/analytics/chats/{chat_id}/overview
func (h *AnalyticsHandler) HandleOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	chatID, err := h.parseChatID(r)
	if err != nil {
		h.writeError(w, "invalid chat_id", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
	}

	startDate, endDate, err := h.parseTimeRange(r)
	if err != nil {
		h.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	overview, err := h.analyticsService.GetOverview(ctx, chatID, startDate, endDate)
	if err != nil {
		slog.Error("failed to get overview", "chat_id", chatID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		h.writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.writeResponse(w, chatID, startDate, endDate, overview)
}

// HandleLeaderboard - GET /api/v1/analytics/chats/{chat_id}/leaderboard
func (h *AnalyticsHandler) HandleLeaderboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	chatID, err := h.parseChatID(r)
	if err != nil {
		h.writeError(w, "invalid chat_id", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
	}

	startDate, endDate, err := h.parseTimeRange(r)
	if err != nil {
		h.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	metric := r.URL.Query().Get("metric")
	if metric == "" {
		metric = "messages" // default
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 10
	}

	if txn != nil {
		txn.AddAttribute("metric", metric)
		txn.AddAttribute("limit", limit)
	}

	leaderboard, err := h.analyticsService.GetLeaderboard(ctx, chatID, startDate, endDate, metric, limit)
	if err != nil {
		slog.Error("failed to get leaderboard", "chat_id", chatID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		h.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.writeResponse(w, chatID, startDate, endDate, leaderboard)
}

// HandleUserDetail - GET /api/v1/analytics/chats/{chat_id}/users/{user_id}
func (h *AnalyticsHandler) HandleUserDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	chatID, err := h.parseChatID(r)
	if err != nil {
		h.writeError(w, "invalid chat_id", http.StatusBadRequest)
		return
	}

	vars := mux.Vars(r)
	userID, err := strconv.ParseInt(vars["user_id"], 10, 64)
	if err != nil {
		h.writeError(w, "invalid user_id", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("user_id", userID)
	}

	startDate, endDate, err := h.parseTimeRange(r)
	if err != nil {
		h.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	userDetail, err := h.analyticsService.GetUserDetail(ctx, chatID, userID, startDate, endDate)
	if err != nil {
		slog.Error("failed to get user detail", "chat_id", chatID, "user_id", userID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		h.writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.writeResponse(w, chatID, startDate, endDate, userDetail)
}

// HandleTimeline - GET /api/v1/analytics/chats/{chat_id}/timeline
func (h *AnalyticsHandler) HandleTimeline(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	chatID, err := h.parseChatID(r)
	if err != nil {
		h.writeError(w, "invalid chat_id", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
	}

	startDate, endDate, err := h.parseTimeRange(r)
	if err != nil {
		h.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	granularity := r.URL.Query().Get("granularity")
	if granularity == "" {
		granularity = "day" // default
	}

	if txn != nil {
		txn.AddAttribute("granularity", granularity)
	}

	timeline, err := h.analyticsService.GetTimeline(ctx, chatID, startDate, endDate, granularity)
	if err != nil {
		slog.Error("failed to get timeline", "chat_id", chatID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		h.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.writeResponse(w, chatID, startDate, endDate, timeline)
}

// HandleHeatmap - GET /api/v1/analytics/chats/{chat_id}/heatmap
func (h *AnalyticsHandler) HandleHeatmap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	chatID, err := h.parseChatID(r)
	if err != nil {
		h.writeError(w, "invalid chat_id", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
	}

	startDate, endDate, err := h.parseTimeRange(r)
	if err != nil {
		h.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	heatmap, err := h.analyticsService.GetHeatmap(ctx, chatID, startDate, endDate)
	if err != nil {
		slog.Error("failed to get heatmap", "chat_id", chatID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		h.writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.writeResponse(w, chatID, startDate, endDate, heatmap)
}

// HandleTopContent - GET /api/v1/analytics/chats/{chat_id}/top-content
func (h *AnalyticsHandler) HandleTopContent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	chatID, err := h.parseChatID(r)
	if err != nil {
		h.writeError(w, "invalid chat_id", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
	}

	startDate, endDate, err := h.parseTimeRange(r)
	if err != nil {
		h.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	metric := r.URL.Query().Get("metric")
	if metric == "" {
		metric = "most_reacted" // default
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 10
	}

	if txn != nil {
		txn.AddAttribute("metric", metric)
		txn.AddAttribute("limit", limit)
	}

	topContent, err := h.analyticsService.GetTopContent(ctx, chatID, startDate, endDate, metric, limit)
	if err != nil {
		slog.Error("failed to get top content", "chat_id", chatID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		h.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.writeResponse(w, chatID, startDate, endDate, topContent)
}

// HandleCompare - GET /api/v1/analytics/chats/{chat_id}/compare
func (h *AnalyticsHandler) HandleCompare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	chatID, err := h.parseChatID(r)
	if err != nil {
		h.writeError(w, "invalid chat_id", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
	}

	startDate, endDate, err := h.parseTimeRange(r)
	if err != nil {
		h.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Parse user IDs from query param: ?user_ids=123,456,789
	userIDsStr := r.URL.Query().Get("user_ids")
	if userIDsStr == "" {
		h.writeError(w, "user_ids parameter required", http.StatusBadRequest)
		return
	}

	userIDStrs := strings.Split(userIDsStr, ",")
	var userIDs []int64
	for _, idStr := range userIDStrs {
		id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64)
		if err != nil {
			h.writeError(w, "invalid user_ids format", http.StatusBadRequest)
			return
		}
		userIDs = append(userIDs, id)
	}

	if txn != nil {
		txn.AddAttribute("compare_user_count", len(userIDs))
	}

	comparison, err := h.analyticsService.CompareUsers(ctx, chatID, userIDs, startDate, endDate)
	if err != nil {
		slog.Error("failed to compare users", "chat_id", chatID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		h.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.writeResponse(w, chatID, startDate, endDate, comparison)
}

// HandleListChats - GET /api/v1/analytics/chats
// Returns all chats with summary statistics (no time range required)
func (h *AnalyticsHandler) HandleListChats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	chats, err := h.analyticsService.ListChats(ctx)
	if err != nil {
		slog.Error("failed to list chats", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		h.writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if txn != nil {
		txn.AddAttribute("chats_count", len(chats))
	}

	// Write response without time range metadata
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := map[string]interface{}{
		"data": chats,
		"metadata": map[string]interface{}{
			"generated_at": time.Now(),
			"total_count":  len(chats),
		},
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

// HandleGetChat - GET /api/v1/analytics/chats/{chat_id}/info
// Returns detailed information about a single chat (no time range required)
func (h *AnalyticsHandler) HandleGetChat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	chatID, err := h.parseChatID(r)
	if err != nil {
		h.writeError(w, "invalid chat_id", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
	}

	chat, err := h.analyticsService.GetChat(ctx, chatID)
	if err != nil {
		slog.Error("failed to get chat", "chat_id", chatID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		if strings.Contains(err.Error(), "not found") {
			h.writeError(w, "chat not found", http.StatusNotFound)
			return
		}
		h.writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Write response without time range metadata
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := map[string]interface{}{
		"data": chat,
		"metadata": map[string]interface{}{
			"chat_id":      chatID,
			"generated_at": time.Now(),
		},
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}
