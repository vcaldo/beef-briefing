package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"beef-briefing/apps/api-service/internal/apperror"
	"beef-briefing/apps/api-service/internal/httputil"

	"github.com/gorilla/mux"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// =============================================================================
// TOURNAMENT ENDPOINTS (Bot API Key Auth)
// =============================================================================

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
