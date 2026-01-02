package handlers

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// DeckHandler handles the /deck command
type DeckHandler struct {
	nrApp *newrelic.Application
}

// NewDeckHandler creates a new DeckHandler
func NewDeckHandler(nrApp *newrelic.Application) *DeckHandler {
	return &DeckHandler{
		nrApp: nrApp,
	}
}

// Handle processes the /deck command
func (h *DeckHandler) Handle(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID
	chatType := update.Message.Chat.Type

	slog.Debug("received /deck command", "chat_id", chatID, "user_id", userID, "chat_type", chatType)

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
		txn = h.nrApp.StartTransaction("bot:deck-command")
		defer txn.End()
		ctx = newrelic.NewContext(ctx, txn)
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("user_id", userID)
	}

	// Send coming soon message
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "The /deck command is coming soon! Stay tuned for the ability to browse all player cards.",
	})
	if err != nil {
		slog.Error("failed to send deck message", "chat_id", chatID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		return
	}

	slog.Debug("deck message sent successfully", "chat_id", chatID)
}
