package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

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
	if claims.ChatID != nil && *claims.ChatID != chatID {
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

	_, chatID, err := parseChatIDWithAuth(r, false)
	if err != nil {
		handleChatAccessError(w, err)
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

// HandleGetHistory retrieves the user's match history.
// GET /api/v1/mini-app/arena/history?chat_id=X&limit=20&offset=0
func (h *ArenaHandler) HandleGetHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:history")
	}

	claims, chatID, err := parseChatIDWithAuth(r, true)
	if err != nil {
		handleChatAccessError(w, err)
		return
	}

	limit := httputil.ParseIntWithDefault(r, "limit", 20, 1, 50)
	offset := httputil.ParseIntWithDefault(r, "offset", 0, 0, 10000)

	entries, total, err := h.service.GetMatchHistory(ctx, chatID, claims.UserID, limit, offset)
	if err != nil {
		slog.Error("failed to get match history", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "failed to get match history", http.StatusInternalServerError)
		return
	}

	// Transform entries for response
	type MatchHistoryResponse struct {
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
		Opponent    OpponentInfo `json:"opponent"`
		Wins        int          `json:"wins"`
		Losses      int          `json:"losses"`
		LastMatchAt *string      `json:"last_match_at,omitempty"`
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
		response.Record = &H2HResponse{
			Opponent: OpponentInfo{
				UserID:    record.OpponentID,
				FirstName: record.OpponentName,
				Username:  record.OpponentUser,
			},
			Wins:   record.Wins,
			Losses: record.Losses,
		}
		if record.LastMatchAt != nil {
			t := record.LastMatchAt.Format(time.RFC3339)
			response.Record.LastMatchAt = &t
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
	if match.Status != "completed" {
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
		case services.ErrTournamentNotFound:
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
		case services.ErrTournamentNotOpen:
			httputil.RespondError(w, "tournament is not open for registration", http.StatusBadRequest)
		case services.ErrTournamentRegistrationClosed:
			httputil.RespondError(w, "tournament registration has closed", http.StatusBadRequest)
		case services.ErrAlreadyRegistered:
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
		case services.ErrTournamentRegistrationClosed:
			httputil.RespondError(w, "tournament registration has closed", http.StatusBadRequest)
		case services.ErrNotRegistered:
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
		case services.ErrTournamentNotFound:
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

	if match.Status != "completed" {
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

// timeNow is a function variable for testing purposes
var timeNow = func() time.Time {
	return time.Now()
}
