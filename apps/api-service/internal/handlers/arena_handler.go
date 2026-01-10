package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

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
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:list-matches")
	}

	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		httputil.RespondError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	chatID, err := httputil.ParseInt64(r, "chat_id")
	if err != nil || chatID == 0 {
		httputil.RespondError(w, "chat_id is required", http.StatusBadRequest)
		return
	}

	// Verify chat access
	if claims.ChatID != nil && *claims.ChatID != chatID {
		httputil.RespondError(w, "access denied to this chat", http.StatusForbidden)
		return
	}

	matches, err := h.service.GetActiveMatches(ctx, chatID)
	if err != nil {
		slog.Error("failed to list matches", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
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
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:create-match")
	}

	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		httputil.RespondError(w, "unauthorized", http.StatusUnauthorized)
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
	if claims.ChatID != nil && *claims.ChatID != req.ChatID {
		httputil.RespondError(w, "access denied to this chat", http.StatusForbidden)
		return
	}

	match, err := h.service.CreateMatch(ctx, req.ChatID, claims.UserID)
	if err != nil {
		slog.Error("failed to create match", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		if err == services.ErrNotEnoughCards {
			httputil.RespondError(w, err.Error(), http.StatusBadRequest)
			return
		}
		httputil.RespondError(w, "failed to create match", http.StatusInternalServerError)
		return
	}

	if txn != nil {
		txn.AddAttribute("match_id", match.ID)
	}

	httputil.RespondJSON(w, match, http.StatusCreated)
}

// HandleGetMatch retrieves a match by ID.
// GET /api/v1/mini-app/arena/match/{id}
func (h *ArenaHandler) HandleGetMatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:get-match")
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

	match, err := h.service.GetMatch(ctx, matchID)
	if err != nil {
		slog.Error("failed to get match", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		if err == services.ErrMatchNotFound {
			httputil.RespondError(w, "match not found", http.StatusNotFound)
			return
		}
		httputil.RespondError(w, "failed to get match", http.StatusInternalServerError)
		return
	}

	// Verify chat access
	if claims.ChatID != nil && *claims.ChatID != match.ChatID {
		httputil.RespondError(w, "access denied to this chat", http.StatusForbidden)
		return
	}

	httputil.RespondJSON(w, match, http.StatusOK)
}

// HandleJoinMatch joins a match.
// POST /api/v1/mini-app/arena/match/{id}/join
func (h *ArenaHandler) HandleJoinMatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:join-match")
	}

	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		httputil.RespondError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	matchID := vars["id"]

	match, err := h.service.JoinMatch(ctx, matchID, claims.UserID)
	if err != nil {
		slog.Error("failed to join match", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		switch err {
		case services.ErrMatchNotFound:
			httputil.RespondError(w, "match not found", http.StatusNotFound)
		case services.ErrMatchNotOpen:
			httputil.RespondError(w, "match is not open for joining", http.StatusBadRequest)
		default:
			httputil.RespondError(w, "failed to join match", http.StatusInternalServerError)
		}
		return
	}

	httputil.RespondJSON(w, match, http.StatusOK)
}

// HandleLeaveMatch leaves a match.
// POST /api/v1/mini-app/arena/match/{id}/leave
func (h *ArenaHandler) HandleLeaveMatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:leave-match")
	}

	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		httputil.RespondError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	matchID := vars["id"]

	err := h.service.LeaveMatch(ctx, matchID, claims.UserID)
	if err != nil {
		slog.Error("failed to leave match", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		switch err {
		case services.ErrMatchNotFound:
			httputil.RespondError(w, "match not found", http.StatusNotFound)
		case services.ErrMatchNotOpen:
			httputil.RespondError(w, "cannot leave after match has started", http.StatusBadRequest)
		default:
			httputil.RespondError(w, "failed to leave match", http.StatusInternalServerError)
		}
		return
	}

	httputil.RespondOK(w)
}

// HandleStartMatch starts a match early (creator only).
// POST /api/v1/mini-app/arena/match/{id}/start
func (h *ArenaHandler) HandleStartMatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:start-match")
	}

	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		httputil.RespondError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	matchID := vars["id"]

	match, err := h.service.StartMatch(ctx, matchID, claims.UserID)
	if err != nil {
		slog.Error("failed to start match", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		switch err {
		case services.ErrMatchNotFound:
			httputil.RespondError(w, "match not found", http.StatusNotFound)
		case services.ErrNotCreator:
			httputil.RespondError(w, "only the match creator can start the match", http.StatusForbidden)
		case services.ErrMatchNotOpen:
			httputil.RespondError(w, "match has already started", http.StatusBadRequest)
		default:
			httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		}
		return
	}

	httputil.RespondJSON(w, match, http.StatusOK)
}

// HandleGetShop retrieves the shop state.
// GET /api/v1/mini-app/arena/match/{id}/shop
func (h *ArenaHandler) HandleGetShop(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:get-shop")
	}

	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		httputil.RespondError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	matchID := vars["id"]

	shop, err := h.service.GetShop(ctx, matchID, claims.UserID)
	if err != nil {
		slog.Error("failed to get shop", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		switch err {
		case services.ErrMatchNotFound:
			httputil.RespondError(w, "match not found", http.StatusNotFound)
		case services.ErrNotParticipant:
			httputil.RespondError(w, "not a participant in this match", http.StatusForbidden)
		default:
			httputil.RespondError(w, "failed to get shop", http.StatusInternalServerError)
		}
		return
	}

	httputil.RespondJSON(w, shop, http.StatusOK)
}

// HandleBuyCard purchases a card from the shop.
// POST /api/v1/mini-app/arena/match/{id}/buy
func (h *ArenaHandler) HandleBuyCard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:buy-card")
	}

	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		httputil.RespondError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	matchID := vars["id"]

	var req BuyCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	shopState, err := h.service.BuyCard(ctx, matchID, claims.UserID, req.CardIndex)
	if err != nil {
		slog.Error("failed to buy card", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		switch err {
		case services.ErrMatchNotFound:
			httputil.RespondError(w, "match not found", http.StatusNotFound)
		case services.ErrNotParticipant:
			httputil.RespondError(w, "not a participant in this match", http.StatusForbidden)
		case services.ErrMatchNotInShopPhase, services.ErrShopPhaseExpired:
			httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		case services.ErrNotEnoughCoins, services.ErrTeamFull, services.ErrCardAlreadyPurchased, services.ErrInvalidCardIndex:
			httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		default:
			httputil.RespondError(w, "failed to buy card", http.StatusInternalServerError)
		}
		return
	}

	httputil.RespondJSON(w, shopState, http.StatusOK)
}

// HandleReroll rerolls the shop cards.
// POST /api/v1/mini-app/arena/match/{id}/reroll
func (h *ArenaHandler) HandleReroll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:reroll")
	}

	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		httputil.RespondError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	matchID := vars["id"]

	shopState, err := h.service.Reroll(ctx, matchID, claims.UserID)
	if err != nil {
		slog.Error("failed to reroll", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		switch err {
		case services.ErrMatchNotFound:
			httputil.RespondError(w, "match not found", http.StatusNotFound)
		case services.ErrNotParticipant:
			httputil.RespondError(w, "not a participant in this match", http.StatusForbidden)
		case services.ErrNotEnoughCoins:
			httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		default:
			httputil.RespondError(w, "failed to reroll", http.StatusInternalServerError)
		}
		return
	}

	httputil.RespondJSON(w, shopState, http.StatusOK)
}

// HandleUpgrade upgrades a team card.
// POST /api/v1/mini-app/arena/match/{id}/upgrade
func (h *ArenaHandler) HandleUpgrade(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:upgrade")
	}

	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		httputil.RespondError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	matchID := vars["id"]

	var req UpgradeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	upgradeType := shop.UpgradeATK
	if req.UpgradeType == "hp" {
		upgradeType = shop.UpgradeHP
	}

	shopState, err := h.service.UpgradeCard(ctx, matchID, claims.UserID, req.TeamSlot, upgradeType)
	if err != nil {
		slog.Error("failed to upgrade card", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		switch err {
		case services.ErrMatchNotFound:
			httputil.RespondError(w, "match not found", http.StatusNotFound)
		case services.ErrNotParticipant:
			httputil.RespondError(w, "not a participant in this match", http.StatusForbidden)
		case services.ErrNotEnoughCoins, services.ErrInvalidCardIndex:
			httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		default:
			httputil.RespondError(w, "failed to upgrade card", http.StatusInternalServerError)
		}
		return
	}

	httputil.RespondJSON(w, shopState, http.StatusOK)
}

// HandleSetOrder sets the team battle order.
// POST /api/v1/mini-app/arena/match/{id}/order
func (h *ArenaHandler) HandleSetOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:set-order")
	}

	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		httputil.RespondError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	matchID := vars["id"]

	var req SetOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	shopState, err := h.service.SetTeamOrder(ctx, matchID, claims.UserID, req.Order)
	if err != nil {
		slog.Error("failed to set order", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		return
	}

	httputil.RespondJSON(w, shopState, http.StatusOK)
}

// HandleSubmitTeam submits the team for battle.
// POST /api/v1/mini-app/arena/match/{id}/team
func (h *ArenaHandler) HandleSubmitTeam(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:submit-team")
	}

	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		httputil.RespondError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	matchID := vars["id"]

	shopState, err := h.service.SubmitTeam(ctx, matchID, claims.UserID)
	if err != nil {
		slog.Error("failed to submit team", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		switch err {
		case services.ErrMatchNotFound:
			httputil.RespondError(w, "match not found", http.StatusNotFound)
		case services.ErrNotParticipant:
			httputil.RespondError(w, "not a participant in this match", http.StatusForbidden)
		case services.ErrTeamAlreadySubmitted:
			httputil.RespondError(w, "team already submitted", http.StatusBadRequest)
		default:
			httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		}
		return
	}

	httputil.RespondJSON(w, shopState, http.StatusOK)
}

// HandleGetBattle retrieves battle results.
// GET /api/v1/mini-app/arena/match/{id}/battle
func (h *ArenaHandler) HandleGetBattle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:get-battle")
	}

	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		httputil.RespondError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	matchID := vars["id"]

	battle, err := h.service.GetBattle(ctx, matchID, claims.UserID)
	if err != nil {
		slog.Error("failed to get battle", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		switch err {
		case services.ErrMatchNotFound:
			httputil.RespondError(w, "match not found", http.StatusNotFound)
		case services.ErrNotParticipant:
			httputil.RespondError(w, "not a participant in this match", http.StatusForbidden)
		default:
			httputil.RespondError(w, "failed to get battle", http.StatusInternalServerError)
		}
		return
	}

	httputil.RespondJSON(w, battle, http.StatusOK)
}

// HandleGetLeaderboard retrieves the game leaderboard.
// GET /api/v1/mini-app/arena/leaderboard?chat_id=X&type=ranked
func (h *ArenaHandler) HandleGetLeaderboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:leaderboard")
	}

	claims := middleware.GetClaimsFromContext(ctx)
	if claims == nil {
		httputil.RespondError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	chatID, err := httputil.ParseInt64(r, "chat_id")
	if err != nil || chatID == 0 {
		if claims.ChatID != nil {
			chatID = *claims.ChatID
		} else {
			httputil.RespondError(w, "chat_id is required", http.StatusBadRequest)
			return
		}
	}

	// Verify chat access
	if claims.ChatID != nil && *claims.ChatID != chatID {
		httputil.RespondError(w, "access denied to this chat", http.StatusForbidden)
		return
	}

	matchType := r.URL.Query().Get("type")
	if matchType == "" {
		matchType = "ranked"
	}

	limit := httputil.ParseIntWithDefault(r, "limit", 50, 1, 100)
	offset := httputil.ParseIntWithDefault(r, "offset", 0, 0, 10000)

	entries, err := h.service.GetLeaderboard(ctx, chatID, matchType, limit, offset)
	if err != nil {
		slog.Error("failed to get leaderboard", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "failed to get leaderboard", http.StatusInternalServerError)
		return
	}

	httputil.RespondJSON(w, map[string]interface{}{
		"entries": entries,
		"type":    matchType,
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
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:bot-create-match")
	}

	var req BotCreateMatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ChatID == 0 || req.CreatorUserID == 0 {
		httputil.RespondError(w, "chat_id and creator_user_id are required", http.StatusBadRequest)
		return
	}

	match, err := h.service.CreateMatch(ctx, req.ChatID, req.CreatorUserID)
	if err != nil {
		slog.Error("failed to create match", "error", err, "chat_id", req.ChatID)
		if txn != nil {
			txn.NoticeError(err)
		}
		if err == services.ErrNotEnoughCards {
			httputil.RespondError(w, err.Error(), http.StatusBadRequest)
			return
		}
		httputil.RespondError(w, "failed to create match", http.StatusInternalServerError)
		return
	}

	if txn != nil {
		txn.AddAttribute("match_id", match.ID)
		txn.AddAttribute("chat_id", req.ChatID)
	}

	httputil.RespondJSON(w, match, http.StatusCreated)
}

// HandleBotGetMatch retrieves a match by ID (bot endpoint).
// GET /api/v1/arena/match/{id}
func (h *ArenaHandler) HandleBotGetMatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:bot-get-match")
	}

	vars := mux.Vars(r)
	matchID := vars["id"]
	if matchID == "" {
		httputil.RespondError(w, "match id is required", http.StatusBadRequest)
		return
	}

	match, err := h.service.GetMatch(ctx, matchID)
	if err != nil {
		slog.Error("failed to get match", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		if err == services.ErrMatchNotFound {
			httputil.RespondError(w, "match not found", http.StatusNotFound)
			return
		}
		httputil.RespondError(w, "failed to get match", http.StatusInternalServerError)
		return
	}

	httputil.RespondJSON(w, match, http.StatusOK)
}

// HandleBotJoinMatch joins a match (bot endpoint).
// POST /api/v1/arena/match/{id}/join
func (h *ArenaHandler) HandleBotJoinMatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:bot-join-match")
	}

	vars := mux.Vars(r)
	matchID := vars["id"]

	var req BotJoinMatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == 0 {
		httputil.RespondError(w, "user_id is required", http.StatusBadRequest)
		return
	}

	match, err := h.service.JoinMatch(ctx, matchID, req.UserID)
	if err != nil {
		slog.Error("failed to join match", "error", err, "user_id", req.UserID)
		if txn != nil {
			txn.NoticeError(err)
		}
		switch err {
		case services.ErrMatchNotFound:
			httputil.RespondError(w, "match not found", http.StatusNotFound)
		case services.ErrMatchNotOpen:
			httputil.RespondError(w, "match is not open for joining", http.StatusBadRequest)
		case services.ErrAlreadyJoined:
			httputil.RespondError(w, "already joined this match", http.StatusBadRequest)
		default:
			httputil.RespondError(w, "failed to join match", http.StatusInternalServerError)
		}
		return
	}

	httputil.RespondJSON(w, match, http.StatusOK)
}

// HandleBotLeaveMatch leaves a match (bot endpoint).
// POST /api/v1/arena/match/{id}/leave
func (h *ArenaHandler) HandleBotLeaveMatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:bot-leave-match")
	}

	vars := mux.Vars(r)
	matchID := vars["id"]

	var req BotJoinMatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == 0 {
		httputil.RespondError(w, "user_id is required", http.StatusBadRequest)
		return
	}

	err := h.service.LeaveMatch(ctx, matchID, req.UserID)
	if err != nil {
		slog.Error("failed to leave match", "error", err, "user_id", req.UserID)
		if txn != nil {
			txn.NoticeError(err)
		}
		switch err {
		case services.ErrMatchNotFound:
			httputil.RespondError(w, "match not found", http.StatusNotFound)
		case services.ErrMatchNotOpen:
			httputil.RespondError(w, "cannot leave after match has started", http.StatusBadRequest)
		default:
			httputil.RespondError(w, "failed to leave match", http.StatusInternalServerError)
		}
		return
	}

	httputil.RespondOK(w)
}

// HandleBotStartMatch starts a match (bot endpoint, creator only).
// POST /api/v1/arena/match/{id}/start
func (h *ArenaHandler) HandleBotStartMatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:bot-start-match")
	}

	vars := mux.Vars(r)
	matchID := vars["id"]

	var req BotJoinMatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == 0 {
		httputil.RespondError(w, "user_id is required", http.StatusBadRequest)
		return
	}

	match, err := h.service.StartMatch(ctx, matchID, req.UserID)
	if err != nil {
		slog.Error("failed to start match", "error", err, "user_id", req.UserID)
		if txn != nil {
			txn.NoticeError(err)
		}
		switch err {
		case services.ErrMatchNotFound:
			httputil.RespondError(w, "match not found", http.StatusNotFound)
		case services.ErrNotCreator:
			httputil.RespondError(w, "only the match creator can start the match", http.StatusForbidden)
		case services.ErrMatchNotOpen:
			httputil.RespondError(w, "match has already started", http.StatusBadRequest)
		default:
			httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		}
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

	result, err := h.service.AutoStartMatch(ctx, matchID)
	if err != nil {
		slog.Error("failed to auto-start match", "error", err, "match_id", matchID)
		if txn != nil {
			txn.NoticeError(err)
		}
		switch err {
		case services.ErrMatchNotFound:
			httputil.RespondError(w, "match not found", http.StatusNotFound)
		case services.ErrMatchNotOpen:
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

	result, err := h.service.ForceSubmitTeams(ctx, matchID)
	if err != nil {
		slog.Error("failed to force submit teams", "error", err, "match_id", matchID)
		if txn != nil {
			txn.NoticeError(err)
		}
		switch err {
		case services.ErrMatchNotFound:
			httputil.RespondError(w, "match not found", http.StatusNotFound)
		case services.ErrMatchNotInShopPhase:
			httputil.RespondError(w, "match is not in shop phase", http.StatusBadRequest)
		default:
			httputil.RespondError(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	httputil.RespondJSON(w, result, http.StatusOK)
}
