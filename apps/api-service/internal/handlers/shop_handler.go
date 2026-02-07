package handlers

import (
	"encoding/json"
	"net/http"

	"beef-briefing/apps/api-service/internal/game/shop"
	"beef-briefing/apps/api-service/internal/httputil"
)

// =============================================================================
// SHOP ENDPOINTS (Mini-App JWT authenticated)
// =============================================================================

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

	// Validate match belongs to user's chat
	if h.validateMatchChatAccess(ctx, w, matchID, claims) == nil {
		return
	}

	addTransactionAttribute(ctx, "match_id", matchID)
	addTransactionAttribute(ctx, "user_id", claims.UserID)

	shopState, err := h.service.GetShop(ctx, matchID, claims.UserID)
	if err != nil {
		handleServiceError(ctx, w, err, "get shop")
		return
	}

	httputil.RespondJSON(w, shopState, http.StatusOK)
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

	// Validate match belongs to user's chat
	if h.validateMatchChatAccess(ctx, w, matchID, claims) == nil {
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

	// Validate match belongs to user's chat
	if h.validateMatchChatAccess(ctx, w, matchID, claims) == nil {
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

	// Validate match belongs to user's chat
	if h.validateMatchChatAccess(ctx, w, matchID, claims) == nil {
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

	// Validate match belongs to user's chat
	if h.validateMatchChatAccess(ctx, w, matchID, claims) == nil {
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

	// Validate match belongs to user's chat
	if h.validateMatchChatAccess(ctx, w, matchID, claims) == nil {
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
