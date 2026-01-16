package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"beef-briefing/apps/api-service/internal/apperror"
	"beef-briefing/apps/api-service/internal/game/shop"
	"beef-briefing/apps/api-service/internal/httputil"
	"beef-briefing/apps/api-service/internal/middleware"
	"beef-briefing/apps/api-service/internal/repository"
	"beef-briefing/apps/api-service/internal/services"
	"beef-briefing/pkg/config"

	"github.com/gorilla/mux"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// ArenaHandler handles HTTP requests for arena game endpoints.
type ArenaHandler struct {
	service *services.ArenaService
	config  *config.Config
}

// NewArenaHandler creates a new ArenaHandler.
func NewArenaHandler(service *services.ArenaService, cfg *config.Config) *ArenaHandler {
	return &ArenaHandler{
		service: service,
		config:  cfg,
	}
}

// chatAccessError represents an error during chat access validation with HTTP status
type chatAccessError struct {
	message string
	status  int
}

func (e *chatAccessError) Error() string { return e.message }

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

// extractTournamentIDFromURL extracts the tournament ID from URL parameters.
// Returns a string error if the ID is missing or invalid.
func extractTournamentIDFromURL(r *http.Request) (string, error) {
	vars := mux.Vars(r)
	tournamentID := vars["tournament_id"]
	if tournamentID == "" {
		return "", fmt.Errorf("tournament id is required")
	}
	return tournamentID, nil
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

// HandleListMatches lists active matches for a chat.
// GET /api/v1/mini-app/arena/matches?chat_id=X
func (h *ArenaHandler) HandleListMatches(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setTransactionName(ctx, "api:arena:list-matches")

	_, chatID, err := parseChatIDWithAuth(r, false)
	if err != nil {
		handleChatAccessError(w, err)
		return
	}

	addTransactionAttribute(ctx, "chat_id", chatID)

	matches, err := h.service.GetActiveMatches(ctx, chatID)
	if err != nil {
		logAndNoticeError(ctx, "failed to list matches", err)
		httputil.RespondError(w, "failed to list matches", http.StatusInternalServerError)
		return
	}

	httputil.RespondJSON(w, map[string]interface{}{
		"matches": matches,
	}, http.StatusOK)
}

// HandleCreateMatch creates a new match.
// POST /api/v1/mini-app/arena/match
func (h *ArenaHandler) HandleCreateMatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setTransactionName(ctx, "api:arena:create-match")

	claims := requireJWTClaims(ctx, w)
	if claims == nil {
		return
	}

	var req CreateMatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ChatID == 0 {
		// Use chat from JWT if not specified
		if claims.ChatID != nil {
			req.ChatID = *claims.ChatID
		} else {
			httputil.RespondError(w, "chat_id is required", http.StatusBadRequest)
			return
		}
	}

	// Verify chat access
	if claims.ChatID == nil {
		httputil.RespondError(w, "chat context required", http.StatusForbidden)
		return
	}
	if *claims.ChatID != req.ChatID {
		httputil.RespondError(w, "access denied to this chat", http.StatusForbidden)
		return
	}

	match, err := h.createMatchInternal(ctx, req.ChatID, claims.UserID)
	if err != nil {
		handleServiceError(ctx, w, err, "create match")
		return
	}

	addTransactionAttribute(ctx, "match_id", match.ID)
	httputil.RespondJSON(w, match, http.StatusCreated)
}

// HandleGetMatch retrieves a match by ID.
// GET /api/v1/mini-app/arena/match/{id}
func (h *ArenaHandler) HandleGetMatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setTransactionName(ctx, "api:arena:get-match")

	claims := requireJWTClaims(ctx, w)
	if claims == nil {
		return
	}

	matchID, err := extractMatchIDFromURL(r)
	if err != nil {
		httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		return
	}

	addTransactionAttribute(ctx, "match_id", matchID)
	addTransactionAttribute(ctx, "user_id", claims.UserID)

	match, err := h.service.GetMatch(ctx, matchID)
	if err != nil {
		handleServiceError(ctx, w, err, "get match")
		return
	}

	// Verify chat access
	if claims.ChatID == nil {
		httputil.RespondError(w, "chat context required", http.StatusForbidden)
		return
	}
	if *claims.ChatID != match.ChatID {
		httputil.RespondError(w, "access denied to this chat", http.StatusForbidden)
		return
	}

	httputil.RespondJSON(w, match, http.StatusOK)
}

// HandleJoinMatch joins a match.
// POST /api/v1/mini-app/arena/match/{id}/join
func (h *ArenaHandler) HandleJoinMatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setTransactionName(ctx, "api:arena:join-match")

	claims := requireJWTClaims(ctx, w)
	if claims == nil {
		return
	}

	matchID, err := extractMatchIDFromURL(r)
	if err != nil {
		httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		return
	}

	addTransactionAttribute(ctx, "match_id", matchID)
	addTransactionAttribute(ctx, "user_id", claims.UserID)

	match, err := h.service.JoinMatch(ctx, matchID, claims.UserID)
	if err != nil {
		handleServiceError(ctx, w, err, "join match")
		return
	}

	httputil.RespondJSON(w, match, http.StatusOK)
}

// HandleLeaveMatch leaves a match.
// POST /api/v1/mini-app/arena/match/{id}/leave
func (h *ArenaHandler) HandleLeaveMatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setTransactionName(ctx, "api:arena:leave-match")

	claims := requireJWTClaims(ctx, w)
	if claims == nil {
		return
	}

	matchID, err := extractMatchIDFromURL(r)
	if err != nil {
		httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		return
	}

	addTransactionAttribute(ctx, "match_id", matchID)
	addTransactionAttribute(ctx, "user_id", claims.UserID)

	err = h.service.LeaveMatch(ctx, matchID, claims.UserID)
	if err != nil {
		handleServiceError(ctx, w, err, "leave match")
		return
	}

	httputil.RespondOK(w)
}

// HandleStartMatch starts a match early (creator only).
// POST /api/v1/mini-app/arena/match/{id}/start
func (h *ArenaHandler) HandleStartMatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setTransactionName(ctx, "api:arena:start-match")

	claims := requireJWTClaims(ctx, w)
	if claims == nil {
		return
	}

	matchID, err := extractMatchIDFromURL(r)
	if err != nil {
		httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		return
	}

	addTransactionAttribute(ctx, "match_id", matchID)
	addTransactionAttribute(ctx, "user_id", claims.UserID)

	match, err := h.startMatchInternal(ctx, matchID, claims.UserID)
	if err != nil {
		handleServiceError(ctx, w, err, "start match")
		return
	}

	httputil.RespondJSON(w, match, http.StatusOK)
}

// HandleGetShop retrieves the shop state.
// GET /api/v1/mini-app/arena/match/{id}/shop
func (h *ArenaHandler) HandleGetShop(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setTransactionName(ctx, "api:arena:get-shop")

	claims := requireJWTClaims(ctx, w)
	if claims == nil {
		return
	}

	matchID, err := extractMatchIDFromURL(r)
	if err != nil {
		httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		return
	}

	addTransactionAttribute(ctx, "match_id", matchID)
	addTransactionAttribute(ctx, "user_id", claims.UserID)

	shop, err := h.service.GetShop(ctx, matchID, claims.UserID)
	if err != nil {
		handleServiceError(ctx, w, err, "get shop")
		return
	}

	httputil.RespondJSON(w, shop, http.StatusOK)
}

// HandleBuyCard purchases a card from the shop.
// POST /api/v1/mini-app/arena/match/{id}/buy
func (h *ArenaHandler) HandleBuyCard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setTransactionName(ctx, "api:arena:buy-card")

	claims := requireJWTClaims(ctx, w)
	if claims == nil {
		return
	}

	matchID, err := extractMatchIDFromURL(r)
	if err != nil {
		httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req BuyCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	addTransactionAttribute(ctx, "match_id", matchID)
	addTransactionAttribute(ctx, "user_id", claims.UserID)
	addTransactionAttribute(ctx, "card_index", req.CardIndex)

	shopState, err := h.service.BuyCard(ctx, matchID, claims.UserID, req.CardIndex)
	if err != nil {
		handleServiceError(ctx, w, err, "buy card")
		return
	}

	httputil.RespondJSON(w, shopState, http.StatusOK)
}

// HandleReroll rerolls the shop cards.
// POST /api/v1/mini-app/arena/match/{id}/reroll
func (h *ArenaHandler) HandleReroll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setTransactionName(ctx, "api:arena:reroll")

	claims := requireJWTClaims(ctx, w)
	if claims == nil {
		return
	}

	matchID, err := extractMatchIDFromURL(r)
	if err != nil {
		httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		return
	}

	addTransactionAttribute(ctx, "match_id", matchID)
	addTransactionAttribute(ctx, "user_id", claims.UserID)

	shopState, err := h.service.Reroll(ctx, matchID, claims.UserID)
	if err != nil {
		handleServiceError(ctx, w, err, "reroll")
		return
	}

	httputil.RespondJSON(w, shopState, http.StatusOK)
}

// HandleUpgrade upgrades a team card.
// POST /api/v1/mini-app/arena/match/{id}/upgrade
func (h *ArenaHandler) HandleUpgrade(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setTransactionName(ctx, "api:arena:upgrade")

	claims := requireJWTClaims(ctx, w)
	if claims == nil {
		return
	}

	matchID, err := extractMatchIDFromURL(r)
	if err != nil {
		httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req UpgradeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	addTransactionAttribute(ctx, "match_id", matchID)
	addTransactionAttribute(ctx, "user_id", claims.UserID)
	addTransactionAttribute(ctx, "team_slot", req.TeamSlot)
	addTransactionAttribute(ctx, "upgrade_type", req.UpgradeType)

	upgradeType := shop.UpgradeATK
	if req.UpgradeType == "hp" {
		upgradeType = shop.UpgradeHP
	}

	shopState, err := h.service.UpgradeCard(ctx, matchID, claims.UserID, req.TeamSlot, upgradeType)
	if err != nil {
		handleServiceError(ctx, w, err, "upgrade card")
		return
	}

	httputil.RespondJSON(w, shopState, http.StatusOK)
}

// HandleSetOrder sets the team battle order.
// POST /api/v1/mini-app/arena/match/{id}/order
func (h *ArenaHandler) HandleSetOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setTransactionName(ctx, "api:arena:set-order")

	claims := requireJWTClaims(ctx, w)
	if claims == nil {
		return
	}

	matchID, err := extractMatchIDFromURL(r)
	if err != nil {
		httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req SetOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	addTransactionAttribute(ctx, "match_id", matchID)
	addTransactionAttribute(ctx, "user_id", claims.UserID)

	shopState, err := h.service.SetTeamOrder(ctx, matchID, claims.UserID, req.Order)
	if err != nil {
		logAndNoticeError(ctx, "failed to set order", err)
		httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		return
	}

	httputil.RespondJSON(w, shopState, http.StatusOK)
}

// HandleSubmitTeam submits the team for battle.
// POST /api/v1/mini-app/arena/match/{id}/team
func (h *ArenaHandler) HandleSubmitTeam(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setTransactionName(ctx, "api:arena:submit-team")

	claims := requireJWTClaims(ctx, w)
	if claims == nil {
		return
	}

	matchID, err := extractMatchIDFromURL(r)
	if err != nil {
		httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		return
	}

	addTransactionAttribute(ctx, "match_id", matchID)
	addTransactionAttribute(ctx, "user_id", claims.UserID)

	shopState, err := h.service.SubmitTeam(ctx, matchID, claims.UserID)
	if err != nil {
		handleServiceError(ctx, w, err, "submit team")
		return
	}

	httputil.RespondJSON(w, shopState, http.StatusOK)
}

// HandleGetBattle retrieves battle results.
// GET /api/v1/mini-app/arena/match/{id}/battle
func (h *ArenaHandler) HandleGetBattle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setTransactionName(ctx, "api:arena:get-battle")

	claims := requireJWTClaims(ctx, w)
	if claims == nil {
		return
	}

	matchID, err := extractMatchIDFromURL(r)
	if err != nil {
		httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		return
	}

	addTransactionAttribute(ctx, "match_id", matchID)
	addTransactionAttribute(ctx, "user_id", claims.UserID)

	battle, err := h.service.GetBattle(ctx, matchID, claims.UserID)
	if err != nil {
		handleServiceError(ctx, w, err, "get battle")
		return
	}

	httputil.RespondJSON(w, battle, http.StatusOK)
}

// HandleGetLeaderboard retrieves the game leaderboard.
// GET /api/v1/mini-app/arena/leaderboard?chat_id=X&type=ranked
func (h *ArenaHandler) HandleGetLeaderboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setTransactionName(ctx, "api:arena:leaderboard")

	_, chatID, err := parseChatIDWithAuth(r, true)
	if err != nil {
		handleChatAccessError(w, err)
		return
	}

	matchType := r.URL.Query().Get("type")
	if matchType == "" {
		matchType = "ranked"
	}

	limit := httputil.ParseIntWithDefault(r, "limit", 50, 1, 100)
	offset := httputil.ParseIntWithDefault(r, "offset", 0, 0, 10000)

	addTransactionAttribute(ctx, "chat_id", chatID)
	addTransactionAttribute(ctx, "match_type", matchType)
	addTransactionAttribute(ctx, "limit", limit)

	entries, err := h.service.GetLeaderboard(ctx, chatID, matchType, limit, offset)
	if err != nil {
		logAndNoticeError(ctx, "failed to get leaderboard", err)
		httputil.RespondError(w, "failed to get leaderboard", http.StatusInternalServerError)
		return
	}

	httputil.RespondJSON(w, map[string]interface{}{
		"entries": entries,
		"type":    matchType,
	}, http.StatusOK)
}

// HandleGetHistory retrieves the user's match history.
// GET /api/v1/mini-app/arena/history?chat_id=X&limit=20&offset=0
func (h *ArenaHandler) HandleGetHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setTransactionName(ctx, "api:arena:history")

	claims, chatID, err := parseChatIDWithAuth(r, true)
	if err != nil {
		handleChatAccessError(w, err)
		return
	}

	limit := httputil.ParseIntWithDefault(r, "limit", 20, 1, 50)
	offset := httputil.ParseIntWithDefault(r, "offset", 0, 0, 10000)

	addTransactionAttribute(ctx, "chat_id", chatID)
	addTransactionAttribute(ctx, "user_id", claims.UserID)
	addTransactionAttribute(ctx, "limit", limit)

	entries, total, err := h.service.GetMatchHistory(ctx, chatID, claims.UserID, limit, offset)
	if err != nil {
		logAndNoticeError(ctx, "failed to get match history", err)
		httputil.RespondError(w, "failed to get match history", http.StatusInternalServerError)
		return
	}

	// Transform entries for response
	type MatchHistoryResponse struct {
		MatchID      string  `json:"match_id"`
		MatchType    string  `json:"match_type"`
		YourPhotoURL *string `json:"your_photo_url,omitempty"`
		Opponent     struct {
			UserID    int64   `json:"user_id"`
			FirstName string  `json:"first_name"`
			Username  string  `json:"username,omitempty"`
			PhotoURL  *string `json:"photo_url,omitempty"`
		} `json:"opponent"`
		Result       string          `json:"result"`
		YourTeam     json.RawMessage `json:"your_team"`
		OpponentTeam json.RawMessage `json:"opponent_team"`
		CompletedAt  string          `json:"completed_at"`
	}

	matches := make([]MatchHistoryResponse, 0, len(entries))
	for _, e := range entries {
		m := MatchHistoryResponse{
			MatchID:      e.MatchID,
			MatchType:    string(e.MatchType),
			Result:       e.Result,
			YourTeam:     e.YourTeam,
			OpponentTeam: e.OpponentTeam,
			CompletedAt:  e.CompletedAt.Format(time.RFC3339),
		}
		m.Opponent.UserID = e.OpponentID
		m.Opponent.FirstName = e.OpponentName
		m.Opponent.Username = e.OpponentUser

		// Generate presigned URLs for photos
		if e.YourPhotoKey != nil && *e.YourPhotoKey != "" {
			if url, err := h.service.GetPhotoPresignedURL(ctx, *e.YourPhotoKey); err == nil && url != "" {
				m.YourPhotoURL = &url
			}
		}
		if e.OpponentPhotoKey != nil && *e.OpponentPhotoKey != "" {
			if url, err := h.service.GetPhotoPresignedURL(ctx, *e.OpponentPhotoKey); err == nil && url != "" {
				m.Opponent.PhotoURL = &url
			}
		}

		matches = append(matches, m)
	}

	httputil.RespondJSON(w, map[string]interface{}{
		"matches":  matches,
		"total":    total,
		"has_more": offset+len(entries) < total,
	}, http.StatusOK)
}

// HandleGetH2H retrieves head-to-head record against a specific opponent.
// GET /api/v1/mini-app/arena/h2h?chat_id=X&opponent_id=Y
func (h *ArenaHandler) HandleGetH2H(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:h2h")
	}

	claims, chatID, err := parseChatIDWithAuth(r, true)
	if err != nil {
		handleChatAccessError(w, err)
		return
	}

	opponentID, err := httputil.ParseInt64(r, "opponent_id")
	if err != nil || opponentID == 0 {
		httputil.RespondError(w, "opponent_id is required", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("user_id", claims.UserID)
		txn.AddAttribute("opponent_id", opponentID)
	}

	record, err := h.service.GetH2HRecord(ctx, chatID, claims.UserID, opponentID)
	if err != nil {
		slog.Error("failed to get h2h record", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "failed to get h2h record", http.StatusInternalServerError)
		return
	}

	// Get recent matches vs opponent
	recentMatches, err := h.service.GetRecentMatchesVsOpponent(ctx, chatID, claims.UserID, opponentID, 10)
	if err != nil {
		slog.Error("failed to get recent matches", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "failed to get recent matches", http.StatusInternalServerError)
		return
	}

	// Transform record for response
	type OpponentInfo struct {
		UserID    int64  `json:"user_id"`
		FirstName string `json:"first_name"`
		Username  string `json:"username,omitempty"`
	}

	type H2HResponse struct {
		Opponent     OpponentInfo `json:"opponent"`
		Wins         int          `json:"wins"`
		Losses       int          `json:"losses"`
		Draws        int          `json:"draws"`
		TotalMatches int          `json:"total_matches"`
		WinRate      float64      `json:"win_rate"`
		LastPlayed   *string      `json:"last_played,omitempty"`
	}

	type RecentMatchResponse struct {
		MatchID      string          `json:"match_id"`
		MatchType    string          `json:"match_type"`
		Result       string          `json:"result"`
		YourTeam     json.RawMessage `json:"your_team"`
		OpponentTeam json.RawMessage `json:"opponent_team"`
		CompletedAt  string          `json:"completed_at"`
	}

	var response struct {
		Record        *H2HResponse          `json:"record"`
		RecentMatches []RecentMatchResponse `json:"recent_matches"`
	}

	if record != nil {
		totalMatches := record.Wins + record.Losses + record.Draws
		var winRate float64
		if totalMatches > 0 {
			winRate = float64(record.Wins) / float64(totalMatches)
		}

		response.Record = &H2HResponse{
			Opponent: OpponentInfo{
				UserID:    record.OpponentID,
				FirstName: record.OpponentName,
				Username:  record.OpponentUser,
			},
			Wins:         record.Wins,
			Losses:       record.Losses,
			Draws:        record.Draws,
			TotalMatches: totalMatches,
			WinRate:      winRate,
		}
		if record.LastMatchAt != nil {
			t := record.LastMatchAt.Format(time.RFC3339)
			response.Record.LastPlayed = &t
		}
	}

	response.RecentMatches = make([]RecentMatchResponse, 0, len(recentMatches))
	for _, m := range recentMatches {
		response.RecentMatches = append(response.RecentMatches, RecentMatchResponse{
			MatchID:      m.MatchID,
			MatchType:    string(m.MatchType),
			Result:       m.Result,
			YourTeam:     m.YourTeam,
			OpponentTeam: m.OpponentTeam,
			CompletedAt:  m.CompletedAt.Format(time.RFC3339),
		})
	}

	httputil.RespondJSON(w, response, http.StatusOK)
}

// HandleGetProfile retrieves the current user's profile with stats and recent matches.
// GET /api/v1/mini-app/arena/profile?chat_id=X
func (h *ArenaHandler) HandleGetProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:profile")
	}

	claims, chatID, err := parseChatIDWithAuth(r, true)
	if err != nil {
		handleChatAccessError(w, err)
		return
	}

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("user_id", claims.UserID)
	}

	profile, err := h.service.GetProfile(ctx, chatID, claims.UserID)
	if err != nil {
		slog.Error("failed to get profile", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "failed to get profile", http.StatusInternalServerError)
		return
	}

	if profile == nil {
		// User has no arena data yet
		httputil.RespondJSON(w, map[string]interface{}{
			"profile":        nil,
			"recent_matches": []interface{}{},
		}, http.StatusOK)
		return
	}

	// Transform recent matches for response
	type MatchResponse struct {
		MatchID   string `json:"match_id"`
		MatchType string `json:"match_type"`
		Opponent  struct {
			UserID    int64  `json:"user_id"`
			FirstName string `json:"first_name"`
			Username  string `json:"username,omitempty"`
		} `json:"opponent"`
		Result       string          `json:"result"`
		YourTeam     json.RawMessage `json:"your_team"`
		OpponentTeam json.RawMessage `json:"opponent_team"`
		CompletedAt  string          `json:"completed_at"`
	}

	matches := make([]MatchResponse, 0, len(profile.RecentMatches))
	for _, e := range profile.RecentMatches {
		m := MatchResponse{
			MatchID:      e.MatchID,
			MatchType:    string(e.MatchType),
			Result:       e.Result,
			YourTeam:     e.YourTeam,
			OpponentTeam: e.OpponentTeam,
			CompletedAt:  e.CompletedAt.Format(time.RFC3339),
		}
		m.Opponent.UserID = e.OpponentID
		m.Opponent.FirstName = e.OpponentName
		m.Opponent.Username = e.OpponentUser
		matches = append(matches, m)
	}

	// Build profile response
	type ProfileStatsResponse struct {
		UserID                  int64   `json:"user_id"`
		FirstName               string  `json:"first_name"`
		Username                string  `json:"username,omitempty"`
		RankedWins              int     `json:"ranked_wins"`
		RankedLosses            int     `json:"ranked_losses"`
		RankedDraws             int     `json:"ranked_draws"`
		RankedMatches           int     `json:"ranked_matches"`
		RankedWinRate           float64 `json:"ranked_win_rate"`
		RankedTournamentsPlayed int     `json:"ranked_tournaments_played"`
		RankedTournamentsWon    int     `json:"ranked_tournaments_won"`
		RankedCurrentStreak     int     `json:"ranked_current_streak"`
		RankedBestStreak        int     `json:"ranked_best_streak"`
		RankedRank              int     `json:"ranked_rank"`
		RegularWins             int     `json:"regular_wins"`
		RegularLosses           int     `json:"regular_losses"`
		RegularDraws            int     `json:"regular_draws"`
		RegularMatchesPlayed    int     `json:"regular_matches_played"`
		RegularWinRate          float64 `json:"regular_win_rate"`
		RegularCurrentStreak    int     `json:"regular_current_streak"`
		RegularBestStreak       int     `json:"regular_best_streak"`
		RegularRank             int     `json:"regular_rank"`
		TotalMatches            int     `json:"total_matches"`
		TotalWins               int     `json:"total_wins"`
		TotalDamageDealt        int     `json:"total_damage_dealt"`
		FirstMatchAt            *string `json:"first_match_at,omitempty"`
		LastMatchAt             *string `json:"last_match_at,omitempty"`
	}

	// Calculate derived fields
	rankedMatches := profile.RankedWins + profile.RankedLosses + profile.RankedDraws
	var rankedWinRate float64
	if rankedMatches > 0 {
		rankedWinRate = float64(profile.RankedWins) / float64(rankedMatches)
	}

	regularMatches := profile.RegularWins + profile.RegularLosses + profile.RegularDraws
	var regularWinRate float64
	if regularMatches > 0 {
		regularWinRate = float64(profile.RegularWins) / float64(regularMatches)
	}

	totalMatches := rankedMatches + regularMatches
	totalWins := profile.RankedWins + profile.RegularWins

	profileResp := ProfileStatsResponse{
		UserID:                  profile.UserID,
		FirstName:               profile.FirstName,
		Username:                profile.Username,
		RankedWins:              profile.RankedWins,
		RankedLosses:            profile.RankedLosses,
		RankedDraws:             profile.RankedDraws,
		RankedMatches:           rankedMatches,
		RankedWinRate:           rankedWinRate,
		RankedTournamentsPlayed: profile.RankedTournamentsPlayed,
		RankedTournamentsWon:    profile.RankedTournamentsWon,
		RankedCurrentStreak:     profile.RankedCurrentStreak,
		RankedBestStreak:        profile.RankedBestStreak,
		RankedRank:              profile.RankedRank,
		RegularWins:             profile.RegularWins,
		RegularLosses:           profile.RegularLosses,
		RegularDraws:            profile.RegularDraws,
		RegularMatchesPlayed:    profile.RegularMatchesPlayed,
		RegularWinRate:          regularWinRate,
		RegularCurrentStreak:    profile.RegularCurrentStreak,
		RegularBestStreak:       profile.RegularBestStreak,
		RegularRank:             profile.RegularRank,
		TotalMatches:            totalMatches,
		TotalWins:               totalWins,
		TotalDamageDealt:        0, // Not tracked in current schema
	}
	if profile.FirstMatchAt != nil {
		t := profile.FirstMatchAt.Format(time.RFC3339)
		profileResp.FirstMatchAt = &t
	}
	if profile.LastMatchAt != nil {
		t := profile.LastMatchAt.Format(time.RFC3339)
		profileResp.LastMatchAt = &t
	}

	httputil.RespondJSON(w, map[string]interface{}{
		"profile":        profileResp,
		"recent_matches": matches,
	}, http.StatusOK)
}

// HandleShareResult allows sharing a match result to the group.
// POST /api/v1/mini-app/arena/match/{id}/share
func (h *ArenaHandler) HandleShareResult(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:share-result")
	}

	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		httputil.RespondError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	matchID := vars["id"]
	if matchID == "" {
		httputil.RespondError(w, "match id is required", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("match_id", matchID)
		txn.AddAttribute("user_id", claims.UserID)
	}

	// Get the match to verify access and status
	match, err := h.service.GetMatch(ctx, matchID)
	if err != nil {
		slog.Error("failed to get match", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "match not found", http.StatusNotFound)
		return
	}

	// Verify the user was in this match
	isParticipant := false
	for _, p := range match.Participants {
		if p.UserID == claims.UserID {
			isParticipant = true
			break
		}
	}
	if !isParticipant {
		httputil.RespondError(w, "not a participant in this match", http.StatusForbidden)
		return
	}

	// Verify match is completed
	if match.Status != repository.MatchStatusCompleted {
		httputil.RespondError(w, "match not completed", http.StatusBadRequest)
		return
	}

	// Return share info - the actual sharing is done by the bot
	httputil.RespondJSON(w, map[string]interface{}{
		"success":  true,
		"match_id": matchID,
		"chat_id":  match.ChatID,
	}, http.StatusOK)
}

// =============================================================================
// Bot-accessible endpoints (API key authenticated, not JWT)
// =============================================================================

// BotCreateMatchRequest represents the request to create a match from bot
type BotCreateMatchRequest struct {
	ChatID        int64 `json:"chat_id"`
	CreatorUserID int64 `json:"creator_user_id"`
}

// BotJoinMatchRequest represents the request to join a match from bot
type BotJoinMatchRequest struct {
	UserID int64 `json:"user_id"`
}

// HandleBotCreateMatch creates a new match (bot endpoint).
// POST /api/v1/arena/match
func (h *ArenaHandler) HandleBotCreateMatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setTransactionName(ctx, "api:arena:bot-create-match")

	var req BotCreateMatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ChatID == 0 || req.CreatorUserID == 0 {
		httputil.RespondError(w, "chat_id and creator_user_id are required", http.StatusBadRequest)
		return
	}

	match, err := h.createMatchInternal(ctx, req.ChatID, req.CreatorUserID)
	if err != nil {
		handleServiceError(ctx, w, err, "create match")
		return
	}

	addTransactionAttribute(ctx, "match_id", match.ID)
	addTransactionAttribute(ctx, "chat_id", req.ChatID)

	httputil.RespondJSON(w, match, http.StatusCreated)
}

// HandleBotGetMatch retrieves a match by ID (bot endpoint).
// GET /api/v1/arena/match/{id}
func (h *ArenaHandler) HandleBotGetMatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setTransactionName(ctx, "api:arena:bot-get-match")

	matchID, err := extractMatchIDFromURL(r)
	if err != nil {
		httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		return
	}

	addTransactionAttribute(ctx, "match_id", matchID)

	match, err := h.getMatchInternal(ctx, matchID, nil)
	if err != nil {
		handleServiceError(ctx, w, err, "get match")
		return
	}

	httputil.RespondJSON(w, match, http.StatusOK)
}

// HandleBotJoinMatch joins a match (bot endpoint).
// POST /api/v1/arena/match/{id}/join
func (h *ArenaHandler) HandleBotJoinMatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setTransactionName(ctx, "api:arena:bot-join-match")

	matchID, err := extractMatchIDFromURL(r)
	if err != nil {
		httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req BotJoinMatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == 0 {
		httputil.RespondError(w, "user_id is required", http.StatusBadRequest)
		return
	}

	addTransactionAttribute(ctx, "match_id", matchID)
	addTransactionAttribute(ctx, "user_id", req.UserID)

	match, err := h.joinMatchInternal(ctx, matchID, req.UserID)
	if err != nil {
		handleServiceError(ctx, w, err, "join match")
		return
	}

	httputil.RespondJSON(w, match, http.StatusOK)
}

// HandleBotLeaveMatch leaves a match (bot endpoint).
// POST /api/v1/arena/match/{id}/leave
func (h *ArenaHandler) HandleBotLeaveMatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setTransactionName(ctx, "api:arena:bot-leave-match")

	matchID, err := extractMatchIDFromURL(r)
	if err != nil {
		httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req BotJoinMatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == 0 {
		httputil.RespondError(w, "user_id is required", http.StatusBadRequest)
		return
	}

	addTransactionAttribute(ctx, "match_id", matchID)
	addTransactionAttribute(ctx, "user_id", req.UserID)

	err = h.leaveMatchInternal(ctx, matchID, req.UserID)
	if err != nil {
		handleServiceError(ctx, w, err, "leave match")
		return
	}

	httputil.RespondOK(w)
}

// HandleBotStartMatch starts a match (bot endpoint, creator only).
// POST /api/v1/arena/match/{id}/start
func (h *ArenaHandler) HandleBotStartMatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setTransactionName(ctx, "api:arena:bot-start-match")

	matchID, err := extractMatchIDFromURL(r)
	if err != nil {
		httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req BotJoinMatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == 0 {
		httputil.RespondError(w, "user_id is required", http.StatusBadRequest)
		return
	}

	addTransactionAttribute(ctx, "match_id", matchID)
	addTransactionAttribute(ctx, "user_id", req.UserID)

	match, err := h.startMatchInternal(ctx, matchID, req.UserID)
	if err != nil {
		handleServiceError(ctx, w, err, "start match")
		return
	}

	httputil.RespondJSON(w, match, http.StatusOK)
}

// HandleBotGetPendingMatches retrieves matches that need action (expired deadlines).
// GET /api/v1/arena/matches/pending
func (h *ArenaHandler) HandleBotGetPendingMatches(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:bot-get-pending")
	}

	matches, err := h.service.GetPendingMatches(ctx)
	if err != nil {
		slog.Error("failed to get pending matches", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "failed to get pending matches", http.StatusInternalServerError)
		return
	}

	httputil.RespondJSON(w, map[string]interface{}{
		"matches": matches,
	}, http.StatusOK)
}

// HandleBotAutoStartMatch auto-starts a match when join deadline expires.
// POST /api/v1/arena/match/{id}/auto-start
func (h *ArenaHandler) HandleBotAutoStartMatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:bot-auto-start")
	}

	vars := mux.Vars(r)
	matchID := vars["id"]

	if txn != nil {
		txn.AddAttribute("match_id", matchID)
	}

	result, err := h.service.AutoStartMatch(ctx, matchID)
	if err != nil {
		slog.Error("failed to auto-start match", "error", err, "match_id", matchID)
		if txn != nil {
			txn.NoticeError(err)
		}
		switch err {
		case apperror.ErrMatchNotFound:
			httputil.RespondError(w, "match not found", http.StatusNotFound)
		case apperror.ErrMatchNotOpen:
			httputil.RespondError(w, "match is not in open state", http.StatusBadRequest)
		default:
			httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		}
		return
	}

	httputil.RespondJSON(w, result, http.StatusOK)
}

// HandleBotForceSubmitTeams forces team submission for all unready participants.
// POST /api/v1/arena/match/{id}/force-submit
func (h *ArenaHandler) HandleBotForceSubmitTeams(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:bot-force-submit")
	}

	vars := mux.Vars(r)
	matchID := vars["id"]

	if txn != nil {
		txn.AddAttribute("match_id", matchID)
	}

	result, err := h.service.ForceSubmitTeams(ctx, matchID)
	if err != nil {
		slog.Error("failed to force submit teams", "error", err, "match_id", matchID)
		if txn != nil {
			txn.NoticeError(err)
		}
		switch err {
		case apperror.ErrMatchNotFound:
			httputil.RespondError(w, "match not found", http.StatusNotFound)
		case apperror.ErrMatchNotInShopPhase:
			httputil.RespondError(w, "match is not in shop phase", http.StatusBadRequest)
		default:
			httputil.RespondError(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	httputil.RespondJSON(w, result, http.StatusOK)
}

// =====================================================
// RANKED TOURNAMENT ENDPOINTS (Bot API Key Auth)
// =====================================================

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

// HandleBotGetTodayTournament gets today's tournament for a chat.
// GET /api/v1/arena/tournament/today?chat_id=X&date=YYYY-MM-DD
func (h *ArenaHandler) HandleBotGetTodayTournament(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:bot-get-today-tournament")
	}

	chatID, err := httputil.ParseInt64(r, "chat_id")
	if err != nil || chatID == 0 {
		httputil.RespondError(w, "chat_id is required", http.StatusBadRequest)
		return
	}

	date := r.URL.Query().Get("date")
	if date == "" {
		httputil.RespondError(w, "date is required (YYYY-MM-DD)", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("date", date)
	}

	tournament, err := h.service.GetTodayTournament(ctx, chatID, date)
	if err != nil {
		slog.Error("failed to get today's tournament", "error", err, "chat_id", chatID)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	httputil.RespondJSON(w, map[string]interface{}{
		"tournament": tournament,
	}, http.StatusOK)
}

// HandleBotGetTournament gets a tournament by ID.
// GET /api/v1/arena/tournament/{id}
func (h *ArenaHandler) HandleBotGetTournament(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:bot-get-tournament")
	}

	vars := mux.Vars(r)
	tournamentIDStr := vars["id"]

	var tournamentID int64
	if _, err := json.Number(tournamentIDStr).Int64(); err == nil {
		tournamentID, _ = json.Number(tournamentIDStr).Int64()
	} else {
		httputil.RespondError(w, "invalid tournament id", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("tournament_id", tournamentID)
	}

	tournament, err := h.service.GetTournamentByID(ctx, tournamentID)
	if err != nil {
		slog.Error("failed to get tournament", "error", err, "tournament_id", tournamentID)
		if txn != nil {
			txn.NoticeError(err)
		}
		switch err {
		case apperror.ErrTournamentNotFound:
			httputil.RespondError(w, "tournament not found", http.StatusNotFound)
		default:
			httputil.RespondError(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	httputil.RespondJSON(w, map[string]interface{}{
		"tournament": tournament,
	}, http.StatusOK)
}

// HandleBotGetPendingAnnouncements gets tournaments that need announcement.
// GET /api/v1/arena/tournaments/pending-announcements
func (h *ArenaHandler) HandleBotGetPendingAnnouncements(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:bot-get-pending-announcements")
	}

	// Use current time for timezone calculations
	tournaments, err := h.service.GetTournamentsNeedingAnnouncement(ctx, timeNow())
	if err != nil {
		slog.Error("failed to get pending announcements", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	httputil.RespondJSON(w, map[string]interface{}{
		"tournaments": tournaments,
	}, http.StatusOK)
}

// HandleBotAnnounceTournament creates and announces a tournament.
// POST /api/v1/arena/tournament/announce
func (h *ArenaHandler) HandleBotAnnounceTournament(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:bot-announce-tournament")
	}

	var req TournamentAnnounceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ChatID == 0 || req.Date == "" {
		httputil.RespondError(w, "chat_id and date are required", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("chat_id", req.ChatID)
		txn.AddAttribute("date", req.Date)
	}

	// Get or create tournament
	tournament, err := h.service.GetOrCreateTodayTournament(ctx, req.ChatID, req.Date)
	if err != nil {
		slog.Error("failed to create tournament", "error", err, "chat_id", req.ChatID)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Mark as announced
	if err := h.service.SetTournamentAnnounced(ctx, tournament.ID, req.MessageID); err != nil {
		slog.Error("failed to mark tournament announced", "error", err, "tournament_id", tournament.ID)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Refetch to get updated status
	tournament, _ = h.service.GetTournamentByID(ctx, tournament.ID)

	httputil.RespondJSON(w, map[string]interface{}{
		"tournament": tournament,
	}, http.StatusOK)
}

// HandleBotJoinTournament adds a user to a tournament.
// POST /api/v1/arena/tournament/join
func (h *ArenaHandler) HandleBotJoinTournament(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:bot-join-tournament")
	}

	var req TournamentJoinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ChatID == 0 || req.UserID == 0 {
		httputil.RespondError(w, "chat_id and user_id are required", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("chat_id", req.ChatID)
		txn.AddAttribute("user_id", req.UserID)
	}

	// Get today's tournament (using today's date in the default timezone)
	date := timeNow().Format("2006-01-02")
	tournament, err := h.service.GetTodayTournament(ctx, req.ChatID, date)
	if err != nil {
		slog.Error("failed to get today's tournament", "error", err, "chat_id", req.ChatID)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if tournament == nil {
		httputil.RespondError(w, "no tournament today", http.StatusNotFound)
		return
	}

	// Join tournament
	tournament, err = h.service.JoinTournament(ctx, tournament.ID, req.UserID)
	if err != nil {
		slog.Error("failed to join tournament", "error", err, "tournament_id", tournament.ID)
		if txn != nil {
			txn.NoticeError(err)
		}
		switch err {
		case apperror.ErrTournamentNotOpen:
			httputil.RespondError(w, "tournament is not open for registration", http.StatusBadRequest)
		case apperror.ErrTournamentRegistrationClosed:
			httputil.RespondError(w, "tournament registration has closed", http.StatusBadRequest)
		case apperror.ErrAlreadyRegistered:
			httputil.RespondError(w, "already registered for this tournament", http.StatusConflict)
		default:
			httputil.RespondError(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	httputil.RespondJSON(w, map[string]interface{}{
		"tournament": tournament,
	}, http.StatusOK)
}

// HandleBotLeaveTournament removes a user from a tournament.
// POST /api/v1/arena/tournament/leave
func (h *ArenaHandler) HandleBotLeaveTournament(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:bot-leave-tournament")
	}

	var req TournamentJoinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ChatID == 0 || req.UserID == 0 {
		httputil.RespondError(w, "chat_id and user_id are required", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("chat_id", req.ChatID)
		txn.AddAttribute("user_id", req.UserID)
	}

	// Get today's tournament
	date := timeNow().Format("2006-01-02")
	tournament, err := h.service.GetTodayTournament(ctx, req.ChatID, date)
	if err != nil {
		slog.Error("failed to get today's tournament", "error", err, "chat_id", req.ChatID)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if tournament == nil {
		httputil.RespondError(w, "no tournament today", http.StatusNotFound)
		return
	}

	// Leave tournament
	tournament, err = h.service.LeaveTournament(ctx, tournament.ID, req.UserID)
	if err != nil {
		slog.Error("failed to leave tournament", "error", err, "tournament_id", tournament.ID)
		if txn != nil {
			txn.NoticeError(err)
		}
		switch err {
		case apperror.ErrTournamentRegistrationClosed:
			httputil.RespondError(w, "tournament registration has closed", http.StatusBadRequest)
		case apperror.ErrNotRegistered:
			httputil.RespondError(w, "not registered for this tournament", http.StatusBadRequest)
		default:
			httputil.RespondError(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	httputil.RespondJSON(w, map[string]interface{}{
		"tournament": tournament,
	}, http.StatusOK)
}

// HandleBotGetPendingClose gets tournaments that need registration closed.
// GET /api/v1/arena/tournaments/pending-close
func (h *ArenaHandler) HandleBotGetPendingClose(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:bot-get-pending-close")
	}

	tournaments, err := h.service.GetTournamentsNeedingClose(ctx, timeNow())
	if err != nil {
		slog.Error("failed to get pending close tournaments", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	httputil.RespondJSON(w, map[string]interface{}{
		"tournaments": tournaments,
	}, http.StatusOK)
}

// HandleBotCloseTournament closes registration and starts a tournament.
// POST /api/v1/arena/tournament/{id}/close
func (h *ArenaHandler) HandleBotCloseTournament(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:bot-close-tournament")
	}

	vars := mux.Vars(r)
	tournamentIDStr := vars["id"]

	var tournamentID int64
	if _, err := json.Number(tournamentIDStr).Int64(); err == nil {
		tournamentID, _ = json.Number(tournamentIDStr).Int64()
	} else {
		httputil.RespondError(w, "invalid tournament id", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("tournament_id", tournamentID)
	}

	result, err := h.service.CloseAndStartTournament(ctx, tournamentID)
	if err != nil {
		slog.Error("failed to close tournament", "error", err, "tournament_id", tournamentID)
		if txn != nil {
			txn.NoticeError(err)
		}
		switch err {
		case apperror.ErrTournamentNotFound:
			httputil.RespondError(w, "tournament not found", http.StatusNotFound)
		default:
			httputil.RespondError(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	httputil.RespondJSON(w, result, http.StatusOK)
}

// HandleBotGetPendingRounds gets tournaments with pending rounds.
// GET /api/v1/arena/tournaments/pending-rounds
func (h *ArenaHandler) HandleBotGetPendingRounds(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:bot-get-pending-rounds")
	}

	tournaments, err := h.service.GetTournamentsWithPendingRounds(ctx)
	if err != nil {
		slog.Error("failed to get pending rounds", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	httputil.RespondJSON(w, map[string]interface{}{
		"tournaments": tournaments,
	}, http.StatusOK)
}

// HandleBotGetShareData returns data needed to share a match result.
// GET /api/v1/arena/match/{id}/share-data
func (h *ArenaHandler) HandleBotGetShareData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:bot-get-share-data")
	}

	vars := mux.Vars(r)
	matchID := vars["id"]
	if matchID == "" {
		httputil.RespondError(w, "match id is required", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("match_id", matchID)
	}

	// Get match details
	match, err := h.service.GetMatch(ctx, matchID)
	if err != nil {
		slog.Error("failed to get match", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "match not found", http.StatusNotFound)
		return
	}

	if match.Status != repository.MatchStatusCompleted {
		httputil.RespondError(w, "match not completed", http.StatusBadRequest)
		return
	}

	// Find winner and loser names
	var winnerName, loserName string
	for _, p := range match.Participants {
		if match.WinnerUserID != nil && p.UserID == *match.WinnerUserID {
			winnerName = p.FirstName
		} else {
			loserName = p.FirstName
		}
	}

	// Format the message
	matchTypeLabel := "Casual"
	if match.MatchType == "ranked" {
		matchTypeLabel = "Ranked"
	}

	var message string
	if match.WinnerUserID != nil {
		message = fmt.Sprintf("🏆 *BEEF ARENA RESULT*\n\n"+
			"*%s* defeated *%s*!\n\n"+
			"📊 Match Type: %s",
			winnerName, loserName, matchTypeLabel)
	} else {
		message = fmt.Sprintf("🏆 *BEEF ARENA RESULT*\n\n"+
			"Match ended in a *DRAW*!\n\n"+
			"📊 Match Type: %s",
			matchTypeLabel)
	}

	httputil.RespondJSON(w, map[string]interface{}{
		"chat_id":    match.ChatID,
		"message":    message,
		"winner_id":  match.WinnerUserID,
		"match_type": match.MatchType,
	}, http.StatusOK)
}

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
	}

	httputil.RespondJSON(w, constants, http.StatusOK)
}

// timeNow is a function variable for testing purposes
var timeNow = func() time.Time {
	return time.Now()
}
