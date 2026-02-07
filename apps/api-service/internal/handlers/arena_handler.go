package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"beef-briefing/apps/api-service/internal/apperror"
	"beef-briefing/apps/api-service/internal/client"
	"beef-briefing/apps/api-service/internal/game/shop"
	"beef-briefing/apps/api-service/internal/httputil"
	"beef-briefing/apps/api-service/internal/middleware"
	"beef-briefing/apps/api-service/internal/services"
	"beef-briefing/pkg/config"

	"github.com/gorilla/mux"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// ArenaHandler handles HTTP requests for arena game endpoints.
type ArenaHandler struct {
	service   *services.ArenaService
	config    *config.Config
	botClient *client.BotClient
}

// NewArenaHandler creates a new ArenaHandler.
func NewArenaHandler(service *services.ArenaService, cfg *config.Config, botClient *client.BotClient) *ArenaHandler {
	return &ArenaHandler{
		service:   service,
		config:    cfg,
		botClient: botClient,
	}
}

// =============================================================================
// Shared Types
// =============================================================================

// chatAccessError represents an error during chat access validation with HTTP status
type chatAccessError struct {
	message string
	status  int
}

func (e *chatAccessError) Error() string { return e.message }

// CreateMatchRequest represents the request to create a match
type CreateMatchRequest struct {
	ChatID int64 `json:"chat_id"`
}

// BuyCardRequest represents the request to buy a card
type BuyCardRequest struct {
	CardIndex int `json:"card_index"`
}

// UpgradeRequest represents the request to upgrade a card
type UpgradeRequest struct {
	TeamSlot    int    `json:"team_slot"`
	UpgradeType string `json:"upgrade_type"` // "atk" or "hp"
}

// SetOrderRequest represents the request to set team order
type SetOrderRequest struct {
	Order []int `json:"order"`
}

// BotCreateMatchRequest represents the request to create a match from bot
type BotCreateMatchRequest struct {
	ChatID        int64 `json:"chat_id"`
	CreatorUserID int64 `json:"creator_user_id"`
}

// BotJoinMatchRequest represents the request to join a match from bot
type BotJoinMatchRequest struct {
	UserID int64 `json:"user_id"`
}

// TournamentAnnounceRequest represents a request to announce a tournament
type TournamentAnnounceRequest struct {
	ChatID    int64  `json:"chat_id"`
	Date      string `json:"date"`
	MessageID int64  `json:"message_id"`
}

// TournamentJoinRequest represents a request to join/leave a tournament
type TournamentJoinRequest struct {
	ChatID int64 `json:"chat_id"`
	UserID int64 `json:"user_id"`
}

// =============================================================================
// Shared Helper Functions
// =============================================================================

// parseChatIDWithAuth parses chat_id from request, validates JWT claims, and verifies access.
// If chat_id is not provided and allowFallback is true, uses chatID from JWT claims.
// Returns (claims, chatID, error). Error is *chatAccessError which contains the HTTP status.
func parseChatIDWithAuth(r *http.Request, allowFallback bool) (*middleware.MiniAppClaims, int64, error) {
	ctx := r.Context()
	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		return nil, 0, &chatAccessError{"unauthorized", http.StatusUnauthorized}
	}

	chatID, err := httputil.ParseInt64(r, "chat_id")
	if err != nil || chatID == 0 {
		if allowFallback && claims.ChatID != nil {
			chatID = *claims.ChatID
		} else {
			return nil, 0, &chatAccessError{"chat_id is required", http.StatusBadRequest}
		}
	}

	// Verify chat access
	if claims.ChatID == nil {
		return nil, 0, &chatAccessError{"chat context required", http.StatusForbidden}
	}
	if *claims.ChatID != chatID {
		return nil, 0, &chatAccessError{"access denied to this chat", http.StatusForbidden}
	}

	return claims, chatID, nil
}

// handleChatAccessError writes an HTTP error response for chat access errors
func handleChatAccessError(w http.ResponseWriter, err error) {
	if cae, ok := err.(*chatAccessError); ok {
		httputil.RespondError(w, cae.message, cae.status)
	} else {
		httputil.RespondError(w, "internal error", http.StatusInternalServerError)
	}
}

// extractMatchIDFromURL extracts the match ID from URL parameters.
// Returns a string error if the ID is missing or invalid.
func extractMatchIDFromURL(r *http.Request) (string, error) {
	vars := mux.Vars(r)
	matchID := vars["id"]
	if matchID == "" {
		return "", fmt.Errorf("match id is required")
	}
	return matchID, nil
}

// logAndNoticeError logs an error and notifies New Relic of the error.
// This consolidates the error logging and New Relic tracking pattern that appears throughout handlers.
// For expected shop-phase errors (team submitted, match not in shop), logs at DEBUG level to reduce log noise.
func logAndNoticeError(ctx context.Context, msg string, err error) {
	// Log expected errors at DEBUG to reduce noise; unexpected errors at ERROR level
	isExpectedError := err == apperror.ErrTeamAlreadySubmitted || err == apperror.ErrMatchNotInShopPhase
	if isExpectedError {
		slog.Debug(msg, "error", err)
	} else {
		slog.Error(msg, "error", err)
	}
	// Still notify New Relic for visibility in APM
	if txn := newrelic.FromContext(ctx); txn != nil {
		txn.NoticeError(err)
	}
}

// handleServiceError maps ArenaService errors to appropriate HTTP responses.
// It logs the error and responds with the appropriate HTTP status code.
// The errOperationName is used in the default error message (e.g., "buy card").
func handleServiceError(ctx context.Context, w http.ResponseWriter, err error, errOperationName string) {
	logAndNoticeError(ctx, fmt.Sprintf("failed to %s", errOperationName), err)

	switch err {
	// Match/participant errors
	case apperror.ErrMatchNotFound:
		httputil.RespondError(w, "match not found", http.StatusNotFound)
	case apperror.ErrNotParticipant:
		httputil.RespondError(w, "not a participant in this match", http.StatusForbidden)
	case apperror.ErrNotCreator:
		httputil.RespondError(w, "only the match creator can perform this action", http.StatusForbidden)
	case apperror.ErrAlreadyJoined:
		httputil.RespondError(w, "already joined this match", http.StatusBadRequest)
	case apperror.ErrRoundNotFound:
		httputil.RespondError(w, "round not found", http.StatusNotFound)

	// Match state errors
	case apperror.ErrMatchNotInShopPhase:
		httputil.RespondError(w, "match is not in shop phase", http.StatusBadRequest)
	case apperror.ErrShopPhaseExpired:
		httputil.RespondError(w, "shop phase has expired", http.StatusBadRequest)
	case apperror.ErrMatchNotOpen:
		httputil.RespondError(w, "match is not open for joining", http.StatusBadRequest)

	// Shop/team errors
	case apperror.ErrTeamAlreadySubmitted:
		httputil.RespondError(w, "team already submitted", http.StatusBadRequest)
	case apperror.ErrTeamFull:
		httputil.RespondError(w, "team is full (max 3 cards)", http.StatusBadRequest)
	case apperror.ErrInvalidCardIndex:
		httputil.RespondError(w, "invalid card index", http.StatusBadRequest)

	// Card/coin errors
	case apperror.ErrCardAlreadyPurchased:
		httputil.RespondError(w, "card already purchased", http.StatusBadRequest)
	case apperror.ErrNotEnoughCoins:
		httputil.RespondError(w, "not enough coins", http.StatusBadRequest)
	case apperror.ErrRerollAlreadyUsed:
		httputil.RespondError(w, "reroll already used", http.StatusBadRequest)
	case apperror.ErrRerollAfterPurchase:
		httputil.RespondError(w, "cannot reroll after purchasing cards", http.StatusBadRequest)
	case apperror.ErrNotEnoughCards:
		httputil.RespondError(w, "not enough cards in group (minimum 10 required)", http.StatusBadRequest)
	case apperror.ErrActiveMatchExists:
		httputil.RespondError(w, "an active match already exists. Please wait for it to complete before creating a new one.", http.StatusBadRequest)

	// Default: unknown error
	default:
		httputil.RespondError(w, fmt.Sprintf("failed to %s", errOperationName), http.StatusInternalServerError)
	}
}

// requireJWTClaims extracts JWT claims from the request context and returns them.
// If no claims are found, it responds with an unauthorized error and returns nil.
func requireJWTClaims(ctx context.Context, w http.ResponseWriter) *middleware.MiniAppClaims {
	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		httputil.RespondError(w, "unauthorized", http.StatusUnauthorized)
		return nil
	}
	return claims
}

// validateMatchChatAccess fetches a match and validates the user has access to its chat.
// Returns the match if valid, or writes an error response and returns nil if invalid.
func (h *ArenaHandler) validateMatchChatAccess(ctx context.Context, w http.ResponseWriter, matchID string, claims *middleware.MiniAppClaims) *services.MatchResponse {
	match, err := h.service.GetMatch(ctx, matchID)
	if err != nil {
		handleServiceError(ctx, w, err, "get match")
		return nil
	}

	if claims.ChatID == nil {
		httputil.RespondError(w, "chat context required", http.StatusForbidden)
		return nil
	}
	if *claims.ChatID != match.ChatID {
		httputil.RespondError(w, "access denied to this chat", http.StatusForbidden)
		return nil
	}

	return match
}

// setTransactionName sets the New Relic transaction name if a transaction exists.
func setTransactionName(ctx context.Context, name string) {
	if txn := newrelic.FromContext(ctx); txn != nil {
		txn.SetName(name)
	}
}

// addTransactionAttribute adds an attribute to the New Relic transaction if one exists.
func addTransactionAttribute(ctx context.Context, key string, value interface{}) {
	if txn := newrelic.FromContext(ctx); txn != nil {
		txn.AddAttribute(key, value)
	}
}

// =============================================================================
// INTERNAL METHODS - Consolidate duplicate bot vs mini-app handler logic
// =============================================================================

// createMatchInternal creates a new match with the given chat ID and creator user ID.
// This is the shared business logic called by both mini-app (JWT) and bot (API key) handlers.
func (h *ArenaHandler) createMatchInternal(ctx context.Context, chatID, creatorUserID int64) (*services.MatchResponse, error) {
	return h.service.CreateMatch(ctx, chatID, creatorUserID)
}

// getMatchInternal retrieves a match and verifies the user has access to the match's chat.
// The claims parameter is used to verify chat access (for mini-app endpoints).
// If claims is nil, no chat verification is performed (for bot endpoints).
func (h *ArenaHandler) getMatchInternal(ctx context.Context, matchID string, claims *middleware.MiniAppClaims) (*services.MatchResponse, error) {
	match, err := h.service.GetMatch(ctx, matchID)
	if err != nil {
		return nil, err
	}

	// If claims are provided (mini-app endpoint), verify chat access
	if claims != nil {
		if claims.ChatID == nil {
			return nil, fmt.Errorf("chat context required")
		}
		if *claims.ChatID != match.ChatID {
			return nil, fmt.Errorf("access denied to this chat")
		}
	}

	return match, nil
}

// joinMatchInternal joins a user to a match.
// This is the shared business logic called by both handlers.
func (h *ArenaHandler) joinMatchInternal(ctx context.Context, matchID string, userID int64) (*services.MatchResponse, error) {
	return h.service.JoinMatch(ctx, matchID, userID)
}

// leaveMatchInternal removes a user from a match.
// This is the shared business logic called by both handlers.
func (h *ArenaHandler) leaveMatchInternal(ctx context.Context, matchID string, userID int64) error {
	return h.service.LeaveMatch(ctx, matchID, userID)
}

// startMatchInternal starts a match early (creator only).
// This is the shared business logic called by both handlers.
func (h *ArenaHandler) startMatchInternal(ctx context.Context, matchID string, userID int64) (*services.MatchResponse, error) {
	return h.service.StartMatch(ctx, matchID, userID)
}

// =============================================================================
// CONSTANTS ENDPOINT (Mini-App JWT authenticated)
// =============================================================================

// HandleGetConstants returns game configuration constants.
// GET /api/v1/mini-app/arena/constants
func (h *ArenaHandler) HandleGetConstants(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:constants")
	}

	// Just return the constants - no auth validation needed beyond JWT which is enforced by middleware
	constants := map[string]interface{}{
		"costs": map[string]int{
			"card":    shop.CardCost,
			"reroll":  shop.RerollCost,
			"upgrade": shop.UpgradeCost,
		},
		"sizes": map[string]int{
			"shop": shop.ShopSize,
			"team": shop.TeamSize,
		},
		"upgrades": map[string]int{
			"atk_amount": shop.ATKUpgradeAmount,
			"hp_amount":  shop.HPUpgradeAmount,
		},
		"timings": map[string]int{
			"shop_phase_duration":  180,
			"join_window_duration": 300,
		},
		"hp_bar_thresholds": map[string]interface{}{
			"high":   66,
			"medium": 33,
			"colors": map[string]string{
				"high":   "#22c55e",
				"medium": "#eab308",
				"low":    "#ef4444",
			},
		},
		"timer_thresholds": map[string]interface{}{
			"safe":    120,
			"warning": 30,
			"colors": map[string]string{
				"safe":    "#22c55e",
				"warning": "#eab308",
				"urgent":  "#ef4444",
			},
		},
	}

	httputil.RespondJSON(w, constants, http.StatusOK)
}

// timeNow is a function variable for testing purposes
var timeNow = func() time.Time {
	return time.Now()
}
