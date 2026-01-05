package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"beef-briefing/apps/api-service/internal/middleware"
	"beef-briefing/apps/api-service/internal/services"
	"beef-briefing/pkg/config"

	"github.com/gorilla/mux"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// MiniAppHandler handles HTTP requests for Mini App endpoints.
type MiniAppHandler struct {
	service     *services.MiniAppService
	cardService *services.CardService
	config      *config.Config
}

// NewMiniAppHandler creates a new MiniAppHandler.
func NewMiniAppHandler(service *services.MiniAppService, cardService *services.CardService, cfg *config.Config) *MiniAppHandler {
	return &MiniAppHandler{
		service:     service,
		cardService: cardService,
		config:      cfg,
	}
}

// AuthRequest represents the authentication request body
type AuthRequest struct {
	InitData string `json:"init_data"`
}

// HandleAuth handles Mini App authentication.
// POST /api/v1/mini-app/auth
func (h *MiniAppHandler) HandleAuth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("failed to decode auth request", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.InitData == "" {
		writeError(w, "init_data is required", http.StatusBadRequest)
		return
	}

	response, err := h.service.Authenticate(ctx, req.InitData)
	if err != nil {
		slog.Warn("Mini App auth failed", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		writeError(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if txn != nil {
		txn.AddAttribute("user_id", response.UserID)
		if response.ChatID != nil {
			txn.AddAttribute("chat_id", *response.ChatID)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleStats handles chat overview statistics.
// GET /api/v1/mini-app/stats?chat_id=...&period=30d
func (h *MiniAppHandler) HandleStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	// Get JWT claims from context
	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse chat_id
	chatIDStr := r.URL.Query().Get("chat_id")
	if chatIDStr == "" {
		writeError(w, "chat_id is required", http.StatusBadRequest)
		return
	}
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		writeError(w, "invalid chat_id", http.StatusBadRequest)
		return
	}

	// Verify chat access
	if claims.ChatID != nil && *claims.ChatID != chatID {
		writeError(w, "access denied to this chat", http.StatusForbidden)
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("period", period)
	}

	stats, err := h.service.GetOverviewStats(ctx, chatID, period)
	if err != nil {
		slog.Error("failed to get stats", "error", err, "chat_id", chatID)
		if txn != nil {
			txn.NoticeError(err)
		}
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// HandleActivity handles daily activity timeline.
// GET /api/v1/mini-app/activity?chat_id=...&period=30d
func (h *MiniAppHandler) HandleActivity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	// Get JWT claims from context
	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse chat_id
	chatIDStr := r.URL.Query().Get("chat_id")
	if chatIDStr == "" {
		writeError(w, "chat_id is required", http.StatusBadRequest)
		return
	}
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		writeError(w, "invalid chat_id", http.StatusBadRequest)
		return
	}

	// Verify chat access
	if claims.ChatID != nil && *claims.ChatID != chatID {
		writeError(w, "access denied to this chat", http.StatusForbidden)
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("period", period)
	}

	activity, err := h.service.GetDailyActivity(ctx, chatID, period)
	if err != nil {
		slog.Error("failed to get activity", "error", err, "chat_id", chatID)
		if txn != nil {
			txn.NoticeError(err)
		}
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"data": activity,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleLeaderboard handles user leaderboard.
// GET /api/v1/mini-app/leaderboard?chat_id=...&period=30d&metric=message_count&page=1&limit=20
func (h *MiniAppHandler) HandleLeaderboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	// Get JWT claims from context
	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse chat_id
	chatIDStr := r.URL.Query().Get("chat_id")
	if chatIDStr == "" {
		writeError(w, "chat_id is required", http.StatusBadRequest)
		return
	}
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		writeError(w, "invalid chat_id", http.StatusBadRequest)
		return
	}

	// Verify chat access
	if claims.ChatID != nil && *claims.ChatID != chatID {
		writeError(w, "access denied to this chat", http.StatusForbidden)
		return
	}

	// Parse query params
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}

	metric := r.URL.Query().Get("metric")
	if metric == "" {
		metric = "message_count"
	}

	// Validate metric
	validMetrics := map[string]bool{
		"message_count":      true,
		"reactions_sent":     true,
		"reactions_received": true,
		"active_days":        true,
	}
	if !validMetrics[metric] {
		writeError(w, "invalid metric. Must be one of: message_count, reactions_sent, reactions_received, active_days", http.StatusBadRequest)
		return
	}

	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("period", period)
		txn.AddAttribute("metric", metric)
		txn.AddAttribute("page", page)
		txn.AddAttribute("limit", limit)
	}

	users, total, err := h.service.GetUserRankings(ctx, chatID, metric, period, page, limit)
	if err != nil {
		slog.Error("failed to get leaderboard", "error", err, "chat_id", chatID)
		if txn != nil {
			txn.NoticeError(err)
		}
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"users": users,
		"total": total,
		"page":  page,
		"limit": limit,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func writeError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// HandleGalleryWeeks handles GET /api/v1/mini-app/gallery/weeks?chat_id=...
func (h *MiniAppHandler) HandleGalleryWeeks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	// Get JWT claims from context
	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse chat_id
	chatIDStr := r.URL.Query().Get("chat_id")
	if chatIDStr == "" {
		writeError(w, "chat_id is required", http.StatusBadRequest)
		return
	}
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		writeError(w, "invalid chat_id", http.StatusBadRequest)
		return
	}

	// Verify chat access
	if claims.ChatID != nil && *claims.ChatID != chatID {
		writeError(w, "access denied to this chat", http.StatusForbidden)
		return
	}

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
	}

	weeks, err := h.cardService.GetGalleryWeeks(ctx, chatID)
	if err != nil {
		slog.Error("failed to get gallery weeks", "error", err, "chat_id", chatID)
		if txn != nil {
			txn.NoticeError(err)
		}
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(weeks)
}

// HandleGalleryImages handles GET /api/v1/mini-app/gallery/images?chat_id=...&week_start=...
func (h *MiniAppHandler) HandleGalleryImages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	// Get JWT claims from context
	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse chat_id
	chatIDStr := r.URL.Query().Get("chat_id")
	if chatIDStr == "" {
		writeError(w, "chat_id is required", http.StatusBadRequest)
		return
	}
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		writeError(w, "invalid chat_id", http.StatusBadRequest)
		return
	}

	// Verify chat access
	if claims.ChatID != nil && *claims.ChatID != chatID {
		writeError(w, "access denied to this chat", http.StatusForbidden)
		return
	}

	// Parse optional week_start
	var weekStart *time.Time
	if ws := r.URL.Query().Get("week_start"); ws != "" {
		t, err := time.Parse("2006-01-02", ws)
		if err != nil {
			writeError(w, "invalid week_start format (expected YYYY-MM-DD)", http.StatusBadRequest)
			return
		}
		weekStart = &t
	}

	// Parse optional user_id filter
	var userID *int64
	if uid := r.URL.Query().Get("user_id"); uid != "" {
		u, err := strconv.ParseInt(uid, 10, 64)
		if err != nil {
			writeError(w, "invalid user_id", http.StatusBadRequest)
			return
		}
		userID = &u
	}

	// Parse optional theme filter
	var theme *string
	if t := r.URL.Query().Get("theme"); t != "" {
		theme = &t
	}

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
		if weekStart != nil {
			txn.AddAttribute("week_start", weekStart.Format("2006-01-02"))
		}
	}

	images, err := h.cardService.GetGalleryImages(ctx, chatID, weekStart, userID, theme)
	if err != nil {
		slog.Error("failed to get gallery images", "error", err, "chat_id", chatID)
		if txn != nil {
			txn.NoticeError(err)
		}
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(images)
}

// HandleGalleryImageURL handles GET /api/v1/mini-app/gallery/image/{id}?expires=...
func (h *MiniAppHandler) HandleGalleryImageURL(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	// Get JWT claims from context
	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse image ID from path
	vars := mux.Vars(r)
	imageIDStr := vars["id"]
	if imageIDStr == "" {
		writeError(w, "image id is required", http.StatusBadRequest)
		return
	}
	imageID, err := strconv.ParseInt(imageIDStr, 10, 64)
	if err != nil {
		writeError(w, "invalid image id", http.StatusBadRequest)
		return
	}

	// Parse optional expires param
	expiresIn := 3600
	if exp := r.URL.Query().Get("expires"); exp != "" {
		if e, err := strconv.Atoi(exp); err == nil && e >= 60 && e <= 86400 {
			expiresIn = e
		}
	}

	if txn != nil {
		txn.AddAttribute("image_id", imageID)
		txn.AddAttribute("expires_in", expiresIn)
	}

	result, err := h.cardService.GetGalleryImageURL(ctx, imageID, expiresIn)
	if err != nil {
		if errors.Is(err, services.ErrCardImageNotFound) {
			writeError(w, "image not found", http.StatusNotFound)
			return
		}
		slog.Error("failed to get gallery image URL", "error", err, "image_id", imageID)
		if txn != nil {
			txn.NoticeError(err)
		}
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleReactionsOverview handles reactions overview statistics.
// GET /api/v1/mini-app/reactions-overview?chat_id=...&period=30d&limit=10
func (h *MiniAppHandler) HandleReactionsOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	// Get JWT claims from context
	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse chat_id
	chatIDStr := r.URL.Query().Get("chat_id")
	if chatIDStr == "" {
		writeError(w, "chat_id is required", http.StatusBadRequest)
		return
	}
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		writeError(w, "invalid chat_id", http.StatusBadRequest)
		return
	}

	// Verify chat access
	if claims.ChatID != nil && *claims.ChatID != chatID {
		writeError(w, "access denied to this chat", http.StatusForbidden)
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}

	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 50 {
			limit = l
		}
	}

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("period", period)
		txn.AddAttribute("limit", limit)
	}

	response, err := h.service.GetReactionsOverview(ctx, chatID, period, limit)
	if err != nil {
		slog.Error("failed to get reactions overview", "error", err, "chat_id", chatID)
		if txn != nil {
			txn.NoticeError(err)
		}
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleRepliesOverview handles replies overview statistics.
// GET /api/v1/mini-app/replies-overview?chat_id=...&period=30d&limit=10
func (h *MiniAppHandler) HandleRepliesOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	// Get JWT claims from context
	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse chat_id
	chatIDStr := r.URL.Query().Get("chat_id")
	if chatIDStr == "" {
		writeError(w, "chat_id is required", http.StatusBadRequest)
		return
	}
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		writeError(w, "invalid chat_id", http.StatusBadRequest)
		return
	}

	// Verify chat access
	if claims.ChatID != nil && *claims.ChatID != chatID {
		writeError(w, "access denied to this chat", http.StatusForbidden)
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}

	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 50 {
			limit = l
		}
	}

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("period", period)
		txn.AddAttribute("limit", limit)
	}

	response, err := h.service.GetRepliesOverview(ctx, chatID, period, limit)
	if err != nil {
		slog.Error("failed to get replies overview", "error", err, "chat_id", chatID)
		if txn != nil {
			txn.NoticeError(err)
		}
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleProfile handles user profile data.
// GET /api/v1/mini-app/profile?chat_id=...&period=30d
func (h *MiniAppHandler) HandleProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	// Get JWT claims from context
	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse chat_id
	chatIDStr := r.URL.Query().Get("chat_id")
	if chatIDStr == "" {
		writeError(w, "chat_id is required", http.StatusBadRequest)
		return
	}
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		writeError(w, "invalid chat_id", http.StatusBadRequest)
		return
	}

	// Verify chat access
	if claims.ChatID != nil && *claims.ChatID != chatID {
		writeError(w, "access denied to this chat", http.StatusForbidden)
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}

	// Use the authenticated user's ID
	userID := claims.UserID

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("user_id", userID)
		txn.AddAttribute("period", period)
	}

	response, err := h.service.GetUserProfile(ctx, chatID, userID, period)
	if err != nil {
		slog.Error("failed to get user profile", "error", err, "chat_id", chatID, "user_id", userID)
		if txn != nil {
			txn.NoticeError(err)
		}
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleHeatmap handles activity heatmap data.
// GET /api/v1/mini-app/heatmap?chat_id=...&period=max&include_user=true
func (h *MiniAppHandler) HandleHeatmap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	// Get JWT claims from context
	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse chat_id
	chatIDStr := r.URL.Query().Get("chat_id")
	if chatIDStr == "" {
		writeError(w, "chat_id is required", http.StatusBadRequest)
		return
	}
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		writeError(w, "invalid chat_id", http.StatusBadRequest)
		return
	}

	// Verify chat access
	if claims.ChatID != nil && *claims.ChatID != chatID {
		writeError(w, "access denied to this chat", http.StatusForbidden)
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "max"
	}

	// Include user heatmap by default
	includeUser := true
	if iu := r.URL.Query().Get("include_user"); iu == "false" {
		includeUser = false
	}

	var userID *int64
	if includeUser {
		userID = &claims.UserID
	}

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("period", period)
		txn.AddAttribute("include_user", includeUser)
	}

	response, err := h.service.GetHeatmapData(ctx, chatID, userID, period)
	if err != nil {
		slog.Error("failed to get heatmap data", "error", err, "chat_id", chatID)
		if txn != nil {
			txn.NoticeError(err)
		}
		writeError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
