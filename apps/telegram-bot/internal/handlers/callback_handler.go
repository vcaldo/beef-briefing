package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"beef-briefing/apps/telegram-bot/internal/client"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// CallbackHandler handles callback queries from inline keyboards
type CallbackHandler struct {
	apiClient *client.APIClient
	nrApp     *newrelic.Application
}

// NewCallbackHandler creates a new CallbackHandler
func NewCallbackHandler(apiClient *client.APIClient, nrApp *newrelic.Application) *CallbackHandler {
	return &CallbackHandler{
		apiClient: apiClient,
		nrApp:     nrApp,
	}
}

// Handle processes callback queries
func (h *CallbackHandler) Handle(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
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

// updateMatchMessage updates the match message with current participant info
func (h *CallbackHandler) updateMatchMessage(ctx context.Context, b *bot.Bot, chatID int64, messageID int, match *client.ArenaMatch) {
	var creatorID int64
	if match.CreatorUserID != nil {
		creatorID = *match.CreatorUserID
	}

	messageText := fmt.Sprintf(
		"⚔️ *Arena Match*\n\n"+
			"👤 Creator: [user](tg://user?id=%d)\n"+
			"⏰ Join window: 5 minutes\n"+
			"👥 Participants: %d\n\n"+
			"Click *Join Match* to participate!",
		creatorID,
		len(match.Participants),
	)

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "🎮 Join Match", CallbackData: fmt.Sprintf("join_match:%s", match.ID)},
				{Text: "🚪 Leave", CallbackData: fmt.Sprintf("leave_match:%s", match.ID)},
			},
			{
				{Text: "▶️ Start Match", CallbackData: fmt.Sprintf("start_match:%s", match.ID)},
			},
		},
	}

	_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        messageText,
		ParseMode:   models.ParseModeMarkdown,
		ReplyMarkup: keyboard,
	})
	if err != nil {
		slog.Error("failed to update match message", "error", err)
	}
}

// updateMatchStartedMessage updates the message when match starts
func (h *CallbackHandler) updateMatchStartedMessage(ctx context.Context, b *bot.Bot, chatID int64, messageID int, match *client.ArenaMatch) {
	participantMentions := ""
	for _, p := range match.Participants {
		if participantMentions != "" {
			participantMentions += ", "
		}
		participantMentions += fmt.Sprintf("[user](tg://user?id=%d)", p.UserID)
	}

	messageText := fmt.Sprintf(
		"⚔️ *Arena Match Started!*\n\n"+
			"🎮 Format: %s\n"+
			"👥 Participants: %s\n\n"+
			"⏰ You have 3 minutes to build your team!\n"+
			"Open the game to select and arrange your cards.",
		match.Format,
		participantMentions,
	)

	// Remove action buttons since match has started
	_, err := b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      messageText,
		ParseMode: models.ParseModeMarkdown,
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
