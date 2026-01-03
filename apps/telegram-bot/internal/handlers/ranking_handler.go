package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// RankingHandler handles the /ranking command
type RankingHandler struct {
	nrApp       *newrelic.Application
	botUsername string
}

// NewRankingHandler creates a new RankingHandler
func NewRankingHandler(nrApp *newrelic.Application) *RankingHandler {
	return &RankingHandler{
		nrApp:       nrApp,
		botUsername: os.Getenv("TELEGRAM_BOT_USERNAME"),
	}
}

// Handle processes the /ranking command
func (h *RankingHandler) Handle(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID
	chatType := update.Message.Chat.Type

	slog.Debug("received /ranking command", "chat_id", chatID, "user_id", userID, "chat_type", chatType)

	// Only allow in group or supergroup chats
	if chatType != "group" && chatType != "supergroup" {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "This command only works in group chats.",
		})
		return
	}

	// Start New Relic transaction if available
	var txn *newrelic.Transaction
	if h.nrApp != nil {
		txn = h.nrApp.StartTransaction("bot:ranking-command")
		defer txn.End()
		ctx = newrelic.NewContext(ctx, txn)
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("user_id", userID)
	}

	// Check if Mini App is configured
	if h.botUsername == "" {
		slog.Warn("TELEGRAM_BOT_USERNAME not configured, ranking Mini App unavailable", "chat_id", chatID)
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "The ranking feature is not configured. Please contact the administrator.",
		})
		if err != nil {
			slog.Error("failed to send ranking error message", "chat_id", chatID, "error", err)
		}
		return
	}

	// Build Mini App URL using t.me direct link format
	// Format: https://t.me/<bot_username>/<app_short_name>?startapp=<chat_id>
	miniAppURL := fmt.Sprintf("https://t.me/%s/leaderboard?startapp=%d", h.botUsername, chatID)

	// Create inline keyboard with URL button to Mini App
	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{{
			{
				Text: "🏆 Open Ranking",
				URL:  miniAppURL,
			},
		}},
	}

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        "Tap the button below to view the leaderboard for this group:",
		ReplyMarkup: keyboard,
	})
	if err != nil {
		slog.Error("failed to send ranking message", "chat_id", chatID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		return
	}

	slog.Debug("ranking message sent successfully", "chat_id", chatID, "mini_app_url", miniAppURL)
}
