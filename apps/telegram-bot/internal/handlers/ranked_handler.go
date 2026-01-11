package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"beef-briefing/apps/telegram-bot/internal/client"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/newrelic/go-agent/v3/newrelic"
)

const (
	// DefaultTimezone is the fallback timezone (UTC-3 São Paulo)
	DefaultTimezone = "America/Sao_Paulo"
)

// RankedHandler handles the /ranked command
type RankedHandler struct {
	apiClient *client.APIClient
	nrApp     *newrelic.Application
}

// NewRankedHandler creates a new RankedHandler
func NewRankedHandler(apiClient *client.APIClient, nrApp *newrelic.Application) *RankedHandler {
	return &RankedHandler{
		apiClient: apiClient,
		nrApp:     nrApp,
	}
}

// Handle processes the /ranked command
func (h *RankedHandler) Handle(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Start New Relic transaction
	var txn *newrelic.Transaction
	if h.nrApp != nil {
		txn = h.nrApp.StartTransaction("handler:ranked-command")
		defer txn.End()
		ctx = newrelic.NewContext(ctx, txn)
	}

	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("user_id", userID)
	}

	// Check if in group/supergroup
	if update.Message.Chat.Type != "group" && update.Message.Chat.Type != "supergroup" {
		h.sendMessage(ctx, b, chatID, "⚠️ The /ranked command can only be used in groups.")
		return
	}

	// Check if ranked tournaments are enabled for this chat
	chatInfo, err := h.apiClient.GetChatInfo(ctx, chatID)
	if err != nil {
		slog.Error("failed to get chat info", "chat_id", chatID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		h.sendMessage(ctx, b, chatID, "⚠️ Failed to check tournament settings. Please try again.")
		return
	}

	if chatInfo != nil && !chatInfo.RankedTournamentsEnabled {
		slog.Info("ranked tournaments disabled for chat", "chat_id", chatID)
		h.sendMessage(ctx, b, chatID, "⚠️ Ranked tournaments are disabled for this group.")
		return
	}

	slog.Info("processing /ranked command", "chat_id", chatID, "user_id", userID)

	// Get today's date in default timezone
	loc, _ := time.LoadLocation(DefaultTimezone)
	today := time.Now().In(loc).Format("2006-01-02")

	// Get today's tournament
	tournament, err := h.apiClient.GetTodayTournament(ctx, chatID, today)
	if err != nil {
		slog.Error("failed to get today's tournament", "error", err, "chat_id", chatID)
		if txn != nil {
			txn.NoticeError(err)
		}
		h.sendMessage(ctx, b, chatID, "❌ Failed to check tournament status. Please try again.")
		return
	}

	// No tournament exists
	if tournament == nil {
		h.sendMessage(ctx, b, chatID,
			"🏆 No tournament scheduled today.\n\n"+
				"Daily ranked tournaments are announced at 00:01 local time.")
		return
	}

	// Handle based on tournament status
	switch tournament.Status {
	case "scheduled":
		h.sendMessage(ctx, b, chatID,
			"🏆 *Tournament Scheduled*\n\n"+
				"Today's ranked tournament hasn't been announced yet.\n"+
				"It will open for registration at 00:01 local time.")

	case "open":
		// Try to join the tournament
		h.handleJoin(ctx, b, chatID, userID, tournament, txn)

	case "in_progress":
		h.sendMessage(ctx, b, chatID,
			"⚔️ *Tournament In Progress*\n\n"+
				"Registration has closed.\n"+
				"The tournament is currently running.\n\n"+
				fmt.Sprintf("👥 %d participants competing", tournament.ParticipantCount))

	case "completed":
		msg := "🏆 *Tournament Complete*\n\nToday's tournament has ended."
		if tournament.WinnerUserID != nil {
			msg += fmt.Sprintf("\n\n🥇 Winner: User ID %d", *tournament.WinnerUserID)
		}
		msg += "\n\nUse /ranked tomorrow for the next battle!"
		h.sendMessage(ctx, b, chatID, msg)

	case "skipped":
		h.sendMessage(ctx, b, chatID,
			"🏆 *Tournament Skipped*\n\n"+
				"Today's tournament was cancelled due to insufficient participants.\n\n"+
				"Use /ranked tomorrow for the next battle!")

	default:
		h.sendMessage(ctx, b, chatID,
			fmt.Sprintf("🏆 Tournament status: %s", tournament.Status))
	}
}

// handleJoin tries to join the user to the tournament
func (h *RankedHandler) handleJoin(ctx context.Context, b *bot.Bot, chatID, userID int64, tournament *client.RankedTournament, txn *newrelic.Transaction) {
	// Check if already registered
	for _, p := range tournament.Participants {
		if p.UserID == userID {
			h.sendAlreadyRegisteredMessage(ctx, b, chatID, tournament)
			return
		}
	}

	// Join tournament
	updated, err := h.apiClient.JoinTournament(ctx, chatID, userID)
	if err != nil {
		slog.Error("failed to join tournament", "error", err, "chat_id", chatID, "user_id", userID)
		if txn != nil {
			txn.NoticeError(err)
		}

		switch err {
		case client.ErrTournamentNotOpen:
			h.sendMessage(ctx, b, chatID, "⚠️ Tournament is not open for registration yet.")
		case client.ErrTournamentRegistrationClosed:
			h.sendMessage(ctx, b, chatID, "⚠️ Tournament registration has closed.")
		case client.ErrAlreadyRegistered:
			h.sendAlreadyRegisteredMessage(ctx, b, chatID, tournament)
		default:
			h.sendMessage(ctx, b, chatID, "❌ Failed to join tournament. Please try again.")
		}
		return
	}

	slog.Info("user joined tournament", "chat_id", chatID, "user_id", userID, "participant_count", updated.ParticipantCount)

	// Success message
	text := fmt.Sprintf(
		"✅ *Joined Ranked Tournament!*\n\n"+
			"You are registered for today's tournament.\n\n"+
			"👥 Participants: %d\n"+
			"⏰ Registration closes at 18:00\n\n"+
			"Use /ranked again to check your status.",
		updated.ParticipantCount,
	)

	// Create keyboard with leave option
	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{
					Text:         "Leave Tournament",
					CallbackData: fmt.Sprintf("ranked_leave:%d", updated.ID),
				},
			},
		},
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   models.ParseModeMarkdown,
		ReplyMarkup: keyboard,
	})
	if err != nil {
		slog.Error("failed to send join confirmation", "error", err, "chat_id", chatID)
	}
}

// sendAlreadyRegisteredMessage sends a message for already registered users
func (h *RankedHandler) sendAlreadyRegisteredMessage(ctx context.Context, b *bot.Bot, chatID int64, tournament *client.RankedTournament) {
	text := fmt.Sprintf(
		"✅ *Already Registered*\n\n"+
			"You're already registered for today's tournament.\n\n"+
			"👥 Participants: %d\n"+
			"⏰ Registration closes at 18:00",
		tournament.ParticipantCount,
	)

	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{
					Text:         "Leave Tournament",
					CallbackData: fmt.Sprintf("ranked_leave:%d", tournament.ID),
				},
			},
		},
	}

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   models.ParseModeMarkdown,
		ReplyMarkup: keyboard,
	})
	if err != nil {
		slog.Error("failed to send already registered message", "error", err, "chat_id", chatID)
	}
}

// sendMessage is a helper to send a simple message
func (h *RankedHandler) sendMessage(ctx context.Context, b *bot.Bot, chatID int64, text string) {
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeMarkdown,
	})
	if err != nil {
		slog.Error("failed to send message", "error", err, "chat_id", chatID)
	}
}
