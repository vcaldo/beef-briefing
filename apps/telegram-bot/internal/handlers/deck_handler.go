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

// DeckHandler handles the /deck command
type DeckHandler struct {
	nrApp      *newrelic.Application
	domainName string
}

// NewDeckHandler creates a new DeckHandler
func NewDeckHandler(nrApp *newrelic.Application) *DeckHandler {
	return &DeckHandler{
		nrApp:      nrApp,
		domainName: os.Getenv("DOMAIN_NAME"),
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

	// Check if Mini App is configured
	if h.domainName == "" {
		slog.Warn("DOMAIN_NAME not configured, deck Mini App unavailable", "chat_id", chatID)
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "The deck feature is not configured. Please contact the administrator.",
		})
		if err != nil {
			slog.Error("failed to send deck error message", "chat_id", chatID, "error", err)
		}
		return
	}

	// Build Mini App URL with chat context passed via startapp parameter
	miniAppURL := fmt.Sprintf("https://deck.%s", h.domainName)

	// Create inline keyboard with Mini App button
	keyboard := &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{{
			{
				Text: "View Cards",
				WebApp: &models.WebAppInfo{
					URL: miniAppURL,
				},
			},
		}},
	}

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        "Tap the button below to browse all player cards for this group:",
		ReplyMarkup: keyboard,
	})
	if err != nil {
		slog.Error("failed to send deck message", "chat_id", chatID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		return
	}

	slog.Debug("deck message sent successfully", "chat_id", chatID, "mini_app_url", miniAppURL)
}
