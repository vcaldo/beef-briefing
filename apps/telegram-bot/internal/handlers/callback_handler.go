package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"beef-briefing/apps/telegram-bot/internal/client"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// CallbackHandler handles callback queries from inline keyboards
type CallbackHandler struct {
	apiClient    *client.APIClient
	nrApp        *newrelic.Application
	arenaBaseURL string
	botToken     string
}

// NewCallbackHandler creates a new CallbackHandler
func NewCallbackHandler(apiClient *client.APIClient, nrApp *newrelic.Application, arenaBaseURL, botToken string) *CallbackHandler {
	return &CallbackHandler{
		apiClient:    apiClient,
		nrApp:        nrApp,
		arenaBaseURL: arenaBaseURL,
		botToken:     botToken,
	}
}

// signGameURL generates an HMAC-SHA256 signature for Games API authentication.
// The signature format matches the API service validation:
// data = "chat_id=%d&match_id=%s&ts=%d&user_id=%d" (alphabetically sorted keys)
// Returns the hex-encoded signature.
func signGameURL(botToken string, chatID, userID int64, matchID string, ts int64) string {
	// Build data string with alphabetically sorted keys (must match API service!)
	dataString := fmt.Sprintf("chat_id=%d&match_id=%s&ts=%d&user_id=%d", chatID, matchID, ts, userID)

	// Calculate HMAC-SHA256 using bot token as key (raw, not hashed)
	mac := hmac.New(sha256.New, []byte(botToken))
	mac.Write([]byte(dataString))
	return hex.EncodeToString(mac.Sum(nil))
}

// Handle processes callback queries
func (h *CallbackHandler) Handle(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}

	// Check if this is a game callback
	if update.CallbackQuery.GameShortName != "" {
		h.handleGameCallback(ctx, b, update.CallbackQuery)
		return
	}

	callbackData := update.CallbackQuery.Data
	userID := update.CallbackQuery.From.ID
	chatID := update.CallbackQuery.Message.Message.Chat.ID
	messageID := update.CallbackQuery.Message.Message.ID

	slog.Debug("received callback query", "data", callbackData, "user_id", userID, "chat_id", chatID)

	// Parse callback data
	parts := strings.SplitN(callbackData, ":", 2)
	if len(parts) != 2 {
		h.answerCallback(ctx, b, update.CallbackQuery.ID, "Invalid action")
		return
	}

	action := parts[0]
	matchID := parts[1]

	// Start New Relic transaction if available
	var txn *newrelic.Transaction
	if h.nrApp != nil {
		txn = h.nrApp.StartTransaction("bot:callback-" + action)
		defer txn.End()
		ctx = newrelic.NewContext(ctx, txn)
		txn.AddAttribute("action", action)
		txn.AddAttribute("match_id", matchID)
		txn.AddAttribute("user_id", userID)
	}

	switch action {
	case "join_match":
		h.handleJoinMatch(ctx, b, update.CallbackQuery, matchID, userID, chatID, messageID, txn)
	case "leave_match":
		h.handleLeaveMatch(ctx, b, update.CallbackQuery, matchID, userID, chatID, messageID, txn)
	case "start_match":
		h.handleStartMatch(ctx, b, update.CallbackQuery, matchID, userID, chatID, messageID, txn)
	case "ranked_join":
		h.handleJoinTournament(ctx, b, update.CallbackQuery, matchID, userID, chatID, messageID, txn)
	case "ranked_leave":
		h.handleLeaveTournament(ctx, b, update.CallbackQuery, matchID, userID, chatID, messageID, txn)
	default:
		h.answerCallback(ctx, b, update.CallbackQuery.ID, "Unknown action")
	}
}

// handleJoinMatch handles the join_match callback
func (h *CallbackHandler) handleJoinMatch(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery, matchID string, userID, chatID int64, messageID int, txn *newrelic.Transaction) {
	match, err := h.apiClient.JoinArenaMatch(ctx, matchID, userID)
	if err != nil {
		if errors.Is(err, client.ErrAlreadyJoined) {
			h.answerCallback(ctx, b, callback.ID, "You've already joined this match!")
			return
		}
		if errors.Is(err, client.ErrMatchNotOpen) {
			h.answerCallback(ctx, b, callback.ID, "This match is no longer open for joining.")
			return
		}
		if errors.Is(err, client.ErrMatchNotFound) {
			h.answerCallback(ctx, b, callback.ID, "Match not found.")
			return
		}

		slog.Error("failed to join match", "match_id", matchID, "user_id", userID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		h.answerCallback(ctx, b, callback.ID, "Failed to join match. Please try again.")
		return
	}

	slog.Info("user joined match", "match_id", matchID, "user_id", userID)
	h.answerCallback(ctx, b, callback.ID, "You've joined the match!")

	// Update the message with new participant count
	h.updateMatchMessage(ctx, b, chatID, messageID, match)
}

// handleLeaveMatch handles the leave_match callback
func (h *CallbackHandler) handleLeaveMatch(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery, matchID string, userID, chatID int64, messageID int, txn *newrelic.Transaction) {
	err := h.apiClient.LeaveArenaMatch(ctx, matchID, userID)
	if err != nil {
		if errors.Is(err, client.ErrMatchNotOpen) {
			h.answerCallback(ctx, b, callback.ID, "Cannot leave - match has already started.")
			return
		}

		slog.Error("failed to leave match", "match_id", matchID, "user_id", userID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		h.answerCallback(ctx, b, callback.ID, "Failed to leave match. Please try again.")
		return
	}

	slog.Info("user left match", "match_id", matchID, "user_id", userID)
	h.answerCallback(ctx, b, callback.ID, "You've left the match.")

	// Get updated match info
	match, err := h.apiClient.GetArenaMatch(ctx, matchID)
	if err != nil {
		slog.Error("failed to get match after leave", "match_id", matchID, "error", err)
		return
	}

	// Update the message
	h.updateMatchMessage(ctx, b, chatID, messageID, match)
}

// handleStartMatch handles the start_match callback (creator only)
func (h *CallbackHandler) handleStartMatch(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery, matchID string, userID, chatID int64, messageID int, txn *newrelic.Transaction) {
	match, err := h.apiClient.StartArenaMatch(ctx, matchID, userID)
	if err != nil {
		// Check for specific errors
		errStr := err.Error()
		if strings.Contains(errStr, "only the match creator") {
			h.answerCallback(ctx, b, callback.ID, "Only the match creator can start the match!")
			return
		}
		if strings.Contains(errStr, "already started") {
			h.answerCallback(ctx, b, callback.ID, "Match has already started.")
			return
		}
		if strings.Contains(errStr, "at least 2 participants") {
			h.answerCallback(ctx, b, callback.ID, "Need at least 2 participants to start!")
			return
		}

		slog.Error("failed to start match", "match_id", matchID, "user_id", userID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		h.answerCallback(ctx, b, callback.ID, "Failed to start match. Please try again.")
		return
	}

	slog.Info("match started", "match_id", matchID, "user_id", userID, "participants", len(match.Participants))
	h.answerCallback(ctx, b, callback.ID, "Match started! Open the game to build your team.")

	// Update message to show match has started
	h.updateMatchStartedMessage(ctx, b, chatID, messageID, match)
}

// updateMatchMessage updates the match message buttons
// Note: Game messages (from SendGame) don't have captions, so we only update the reply markup
func (h *CallbackHandler) updateMatchMessage(ctx context.Context, b *bot.Bot, chatID int64, messageID int, match *client.ArenaMatch) {
	// Build participant count into button text so users see updated info
	joinText := fmt.Sprintf("➕ Join (%d)", len(match.Participants))

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "🎮 Play Arena", CallbackGame: &models.CallbackGame{}},
			},
			{
				{Text: joinText, CallbackData: fmt.Sprintf("join_match:%s", match.ID)},
				{Text: "🚪 Leave", CallbackData: fmt.Sprintf("leave_match:%s", match.ID)},
			},
			{
				{Text: "▶️ Start Match", CallbackData: fmt.Sprintf("start_match:%s", match.ID)},
			},
		},
	}

	_, err := b.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
		ChatID:      chatID,
		MessageID:   messageID,
		ReplyMarkup: keyboard,
	})
	if err != nil {
		slog.Error("failed to update match message", "error", err)
	}
}

// updateMatchStartedMessage updates the message when match starts
// Keeps the Play button but removes join/leave/start buttons since match has started
func (h *CallbackHandler) updateMatchStartedMessage(ctx context.Context, b *bot.Bot, chatID int64, messageID int, match *client.ArenaMatch) {
	// Keep only the Play button since match has started
	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: fmt.Sprintf("🎮 Play Arena (%d players)", len(match.Participants)), CallbackGame: &models.CallbackGame{}},
			},
		},
	}

	_, err := b.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
		ChatID:      chatID,
		MessageID:   messageID,
		ReplyMarkup: keyboard,
	})
	if err != nil {
		slog.Error("failed to update match started message", "error", err)
	}
}

// answerCallback sends a callback query answer
func (h *CallbackHandler) answerCallback(ctx context.Context, b *bot.Bot, callbackID, text string) {
	_, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackID,
		Text:            text,
		ShowAlert:       false,
	})
	if err != nil {
		slog.Error("failed to answer callback query", "error", err)
	}
}

// handleGameCallback processes game button clicks
func (h *CallbackHandler) handleGameCallback(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery) {
	if callback.GameShortName == "" {
		slog.Warn("game callback without game_short_name", "user_id", callback.From.ID)
		h.answerCallback(ctx, b, callback.ID, "Invalid game callback")
		return
	}

	userID := callback.From.ID
	chatID := callback.Message.Message.Chat.ID

	// Query user's active match from API
	match, err := h.apiClient.GetUserActiveMatch(ctx, chatID, userID)
	if err != nil {
		slog.Error("failed to get user active match", "user_id", userID, "chat_id", chatID, "error", err)
		h.answerCallback(ctx, b, callback.ID, "Failed to load game. Please try again.")
		return
	}

	// If no active match, still allow access to lobby (match_id will be empty)
	matchID := ""
	if match != nil {
		matchID = match.ID
	}

	// Generate timestamp and signature for Games API authentication
	ts := time.Now().Unix()
	sig := signGameURL(h.botToken, chatID, userID, matchID, ts)

	// Build signed game URL with all authentication parameters
	gameURL := fmt.Sprintf(
		"%s?chat_id=%d&user_id=%d&match_id=%s&ts=%d&sig=%s",
		h.arenaBaseURL,
		chatID,
		userID,
		matchID,
		ts,
		sig,
	)

	// Answer callback query with game URL
	_, err = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callback.ID,
		URL:             gameURL,
	})
	if err != nil {
		slog.Error("failed to answer game callback", "error", err)
		return
	}

	slog.Info("game callback answered", "user_id", userID, "chat_id", chatID, "match_id", matchID, "game_url", gameURL)
}

// =====================================================
// RANKED TOURNAMENT CALLBACKS
// =====================================================

// handleJoinTournament handles the ranked_join callback
func (h *CallbackHandler) handleJoinTournament(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery, _ string, userID, chatID int64, messageID int, txn *newrelic.Transaction) {
	// Join tournament via API
	tournament, err := h.apiClient.JoinTournament(ctx, chatID, userID)
	if err != nil {
		if errors.Is(err, client.ErrAlreadyRegistered) {
			h.answerCallback(ctx, b, callback.ID, "You're already registered for this tournament!")
			return
		}
		if errors.Is(err, client.ErrTournamentNotOpen) {
			h.answerCallback(ctx, b, callback.ID, "Tournament is not open for registration yet.")
			return
		}
		if errors.Is(err, client.ErrTournamentRegistrationClosed) {
			h.answerCallback(ctx, b, callback.ID, "Registration has closed.")
			return
		}
		if errors.Is(err, client.ErrTournamentNotFound) {
			h.answerCallback(ctx, b, callback.ID, "Tournament not found.")
			return
		}

		slog.Error("failed to join tournament", "chat_id", chatID, "user_id", userID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		h.answerCallback(ctx, b, callback.ID, "Failed to join tournament. Please try again.")
		return
	}

	slog.Info("user joined tournament via callback", "chat_id", chatID, "user_id", userID, "participant_count", tournament.ParticipantCount)
	h.answerCallback(ctx, b, callback.ID, fmt.Sprintf("Joined! (%d participants)", tournament.ParticipantCount))

	// Update announcement message with new count
	h.updateTournamentMessage(ctx, b, chatID, messageID, tournament)
}

// handleLeaveTournament handles the ranked_leave callback
func (h *CallbackHandler) handleLeaveTournament(ctx context.Context, b *bot.Bot, callback *models.CallbackQuery, _ string, userID, chatID int64, messageID int, txn *newrelic.Transaction) {
	// Leave tournament via API
	tournament, err := h.apiClient.LeaveTournament(ctx, chatID, userID)
	if err != nil {
		if errors.Is(err, client.ErrTournamentRegistrationClosed) {
			h.answerCallback(ctx, b, callback.ID, "Cannot leave - registration has closed.")
			return
		}
		if errors.Is(err, client.ErrNotRegistered) {
			h.answerCallback(ctx, b, callback.ID, "You're not registered for this tournament.")
			return
		}

		slog.Error("failed to leave tournament", "chat_id", chatID, "user_id", userID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		h.answerCallback(ctx, b, callback.ID, "Failed to leave tournament. Please try again.")
		return
	}

	slog.Info("user left tournament via callback", "chat_id", chatID, "user_id", userID, "participant_count", tournament.ParticipantCount)
	h.answerCallback(ctx, b, callback.ID, "You've left the tournament.")

	// Update announcement message
	h.updateTournamentMessage(ctx, b, chatID, messageID, tournament)
}

// updateTournamentMessage updates the tournament announcement message
func (h *CallbackHandler) updateTournamentMessage(ctx context.Context, b *bot.Bot, chatID int64, messageID int, tournament *client.RankedTournament) {
	text := fmt.Sprintf(
		"🏆 *Ranked Tournament Open!*\n\n"+
			"Today's arena battle is now accepting participants.\n\n"+
			"⏰ Registration closes at 18:00\n"+
			"👥 Participants: %d\n\n"+
			"Use /ranked to join!",
		tournament.ParticipantCount,
	)

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{
					Text:         fmt.Sprintf("Join Tournament (%d)", tournament.ParticipantCount),
					CallbackData: fmt.Sprintf("ranked_join:%d", tournament.ID),
				},
			},
		},
	}

	_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ParseMode:   models.ParseModeMarkdown,
		ReplyMarkup: keyboard,
	})
	if err != nil {
		slog.Error("failed to update tournament message", "error", err)
	}
}
