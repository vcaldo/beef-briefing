package handlers

import (
	"context"
	"log/slog"

	"beef-briefing/apps/telegram-bot/content"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// TechnicalHandler handles the /technical command
type TechnicalHandler struct {
	nrApp *newrelic.Application
}

// NewTechnicalHandler creates a new TechnicalHandler
func NewTechnicalHandler(nrApp *newrelic.Application) *TechnicalHandler {
	return &TechnicalHandler{
		nrApp: nrApp,
	}
}

// Handle processes the /technical command
func (h *TechnicalHandler) Handle(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID
	chatType := update.Message.Chat.Type

	slog.Debug("received /technical command", "chat_id", chatID, "user_id", userID, "chat_type", chatType)

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
		txn = h.nrApp.StartTransaction("bot:technical-command")
		defer txn.End()
		ctx = newrelic.NewContext(ctx, txn)
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("user_id", userID)
	}

	// Send the technical content
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      content.TechnicalContent,
		ParseMode: models.ParseModeMarkdown,
	})
	if err != nil {
		slog.Error("failed to send technical message", "chat_id", chatID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		return
	}

	slog.Debug("technical message sent successfully", "chat_id", chatID)
}
