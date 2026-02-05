package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"beef-briefing/apps/api-service/internal/apperror"
	"beef-briefing/apps/api-service/internal/httputil"
	"beef-briefing/apps/api-service/internal/middleware"
	"beef-briefing/apps/api-service/internal/repository"

	"github.com/gorilla/mux"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// =============================================================================
// Mini-App Match Endpoints (JWT authenticated)
// =============================================================================

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

// HandleGetUserActiveMatch retrieves a user's active match in a specific chat.
// GET /api/v1/arena/matches/active?chat_id=X&user_id=Y
func (h *ArenaHandler) HandleGetUserActiveMatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setTransactionName(ctx, "api:arena:get-user-active-match")

	claims, chatID, err := parseChatIDWithAuth(r, false)
	if err != nil {
		handleChatAccessError(w, err)
		return
	}

	// Parse user_id from query params
	userID, err := httputil.ParseInt64(r, "user_id")
	if err != nil || userID == 0 {
		httputil.RespondError(w, "user_id is required", http.StatusBadRequest)
		return
	}

	// Verify the requesting user is querying their own active match
	if claims.UserID != userID {
		httputil.RespondError(w, "can only query your own active match", http.StatusForbidden)
		return
	}

	addTransactionAttribute(ctx, "chat_id", chatID)
	addTransactionAttribute(ctx, "user_id", userID)

	match, err := h.service.GetUserActiveMatch(ctx, chatID, userID)
	if err != nil {
		logAndNoticeError(ctx, "failed to get user active match", err)
		httputil.RespondError(w, "failed to get user active match", http.StatusInternalServerError)
		return
	}

	// Return 404 if no active match found
	if match == nil {
		httputil.RespondError(w, "no active match found", http.StatusNotFound)
		return
	}

	httputil.RespondJSON(w, match, http.StatusOK)
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

	// Validate match belongs to user's chat
	if h.validateMatchChatAccess(ctx, w, matchID, claims) == nil {
		return
	}

	addTransactionAttribute(ctx, "match_id", matchID)
	addTransactionAttribute(ctx, "user_id", claims.UserID)

	match, err := h.service.JoinMatch(ctx, matchID, claims.UserID)
	if err != nil {
		handleServiceError(ctx, w, err, "join match")
		return
	}

	// Notify bot to update Telegram message with new participant list
	if h.botClient != nil && match.TelegramMessageID != nil {
		go h.botClient.NotifyParticipantChange(context.Background(), match.ID, match.ChatID, *match.TelegramMessageID)
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

	// Validate match belongs to user's chat and get match info for notification
	match := h.validateMatchChatAccess(ctx, w, matchID, claims)
	if match == nil {
		return
	}

	addTransactionAttribute(ctx, "match_id", matchID)
	addTransactionAttribute(ctx, "user_id", claims.UserID)

	err = h.service.LeaveMatch(ctx, matchID, claims.UserID)
	if err != nil {
		handleServiceError(ctx, w, err, "leave match")
		return
	}

	// Notify bot to update Telegram message with new participant list
	if h.botClient != nil && match.TelegramMessageID != nil {
		go h.botClient.NotifyParticipantChange(context.Background(), match.ID, match.ChatID, *match.TelegramMessageID)
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

	// Validate match belongs to user's chat
	if h.validateMatchChatAccess(ctx, w, matchID, claims) == nil {
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

	// Validate match belongs to user's chat
	if h.validateMatchChatAccess(ctx, w, matchID, claims) == nil {
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

// HandleGetRoundBattle retrieves battle results for a specific round within a multi-round match.
// GET /api/v1/mini-app/arena/match/{id}/battle/round/{round_number}
func (h *ArenaHandler) HandleGetRoundBattle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	setTransactionName(ctx, "api:arena:get-round-battle")

	claims := requireJWTClaims(ctx, w)
	if claims == nil {
		return
	}

	matchID, err := extractMatchIDFromURL(r)
	if err != nil {
		httputil.RespondError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Extract round_number from URL path
	vars := mux.Vars(r)
	roundNumberStr := vars["round_number"]
	if roundNumberStr == "" {
		httputil.RespondError(w, "round_number is required", http.StatusBadRequest)
		return
	}
	roundNumber, err := strconv.Atoi(roundNumberStr)
	if err != nil {
		httputil.RespondError(w, "round_number must be an integer", http.StatusBadRequest)
		return
	}

	// Validate match belongs to user's chat
	if h.validateMatchChatAccess(ctx, w, matchID, claims) == nil {
		return
	}

	addTransactionAttribute(ctx, "match_id", matchID)
	addTransactionAttribute(ctx, "user_id", claims.UserID)
	addTransactionAttribute(ctx, "round_number", roundNumber)

	battle, err := h.service.GetRoundBattle(ctx, matchID, roundNumber, claims.UserID)
	if err != nil {
		handleServiceError(ctx, w, err, "get round battle")
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
	addTransactionAttribute(ctx, "offset", offset)

	entries, total, err := h.service.GetLeaderboard(ctx, chatID, matchType, limit, offset)
	if err != nil {
		logAndNoticeError(ctx, "failed to get leaderboard", err)
		httputil.RespondError(w, "failed to get leaderboard", http.StatusInternalServerError)
		return
	}

	httputil.RespondJSON(w, map[string]interface{}{
		"entries":  entries,
		"type":     matchType,
		"total":    total,
		"page":     offset / limit,
		"limit":    limit,
		"has_more": offset+len(entries) < total,
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
		Format       string  `json:"format"`
		PlayerCount  int     `json:"player_count"`
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
			Format:       e.Format,
			PlayerCount:  e.PlayerCount,
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
		PhotoURL                *string `json:"photo_url,omitempty"`
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
		PhotoURL:                profile.PhotoURL,
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

	// Validate match belongs to user's chat
	match := h.validateMatchChatAccess(ctx, w, matchID, claims)
	if match == nil {
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
// Bot Match Endpoints (API key authenticated)
// =============================================================================

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

// HandleBotGetUserActiveMatch retrieves a user's active match in a specific chat.
// GET /api/v1/arena/matches/active?chat_id=X&user_id=Y
func (h *ArenaHandler) HandleBotGetUserActiveMatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:bot-get-user-active-match")
	}

	chatID, err := httputil.ParseInt64(r, "chat_id")
	if err != nil || chatID == 0 {
		httputil.RespondError(w, "chat_id is required", http.StatusBadRequest)
		return
	}

	userID, err := httputil.ParseInt64(r, "user_id")
	if err != nil || userID == 0 {
		httputil.RespondError(w, "user_id is required", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("user_id", userID)
	}

	match, err := h.service.GetUserActiveMatch(ctx, chatID, userID)
	if err != nil {
		slog.Error("failed to get user active match", "error", err, "chat_id", chatID, "user_id", userID)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "failed to get user active match", http.StatusInternalServerError)
		return
	}

	if match == nil {
		httputil.RespondError(w, "no active match found", http.StatusNotFound)
		return
	}

	httputil.RespondJSON(w, match, http.StatusOK)
}

// HandleGetChatOpenMatch retrieves an open match for a specific chat.
// GET /api/v1/arena/matches/open?chat_id=X
func (h *ArenaHandler) HandleGetChatOpenMatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:get-chat-open-match")
	}

	chatID, err := httputil.ParseInt64(r, "chat_id")
	if err != nil || chatID == 0 {
		httputil.RespondError(w, "chat_id is required", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
	}

	match, err := h.service.GetChatOpenMatch(ctx, chatID)
	if err != nil {
		slog.Error("failed to get chat open match", "error", err, "chat_id", chatID)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "failed to get chat open match", http.StatusInternalServerError)
		return
	}

	if match == nil {
		httputil.RespondError(w, "no open match found", http.StatusNotFound)
		return
	}

	httputil.RespondJSON(w, match, http.StatusOK)
}

// HandleSetTelegramMessageID updates the telegram_message_id for a match.
// PATCH /api/v1/arena/match/{id}/message-id
func (h *ArenaHandler) HandleSetTelegramMessageID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		txn.SetName("api:arena:set-telegram-message-id")
	}

	vars := mux.Vars(r)
	matchID := vars["id"]
	if matchID == "" {
		httputil.RespondError(w, "match ID is required", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("match_id", matchID)
	}

	var req struct {
		TelegramMessageID int64 `json:"telegram_message_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.TelegramMessageID == 0 {
		httputil.RespondError(w, "telegram_message_id is required", http.StatusBadRequest)
		return
	}

	if txn != nil {
		txn.AddAttribute("telegram_message_id", req.TelegramMessageID)
	}

	err := h.service.SetTelegramMessageID(ctx, matchID, req.TelegramMessageID)
	if err != nil {
		slog.Error("failed to set telegram message ID", "error", err, "match_id", matchID)
		if txn != nil {
			txn.NoticeError(err)
		}
		httputil.RespondError(w, "failed to set telegram message ID", http.StatusInternalServerError)
		return
	}

	httputil.RespondJSON(w, map[string]string{"status": "ok"}, http.StatusOK)
}
