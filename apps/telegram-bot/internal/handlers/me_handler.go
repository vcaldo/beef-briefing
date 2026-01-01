package handlers

import (
	"context"
	"errors"
	"log/slog"

	"beef-briefing/apps/telegram-bot/internal/client"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// MeHandler handles the /me command
type MeHandler struct {
	apiClient *client.APIClient
	nrApp     *newrelic.Application
}

// NewMeHandler creates a new MeHandler
func NewMeHandler(apiClient *client.APIClient, nrApp *newrelic.Application) *MeHandler {
	return &MeHandler{
		apiClient: apiClient,
		nrApp:     nrApp,
	}
}

// Handle processes the /me command
func (h *MeHandler) Handle(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID
	chatType := update.Message.Chat.Type

	slog.Debug("received /me command", "chat_id", chatID, "user_id", userID, "chat_type", chatType)

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
		txn = h.nrApp.StartTransaction("bot:me-command")
		defer txn.End()
		ctx = newrelic.NewContext(ctx, txn)
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("user_id", userID)
	}

	// Fetch card image URL from API
	cardImage, err := h.apiClient.GetCardImageURL(ctx, userID, chatID)
	if err != nil {
		if errors.Is(err, client.ErrCardNotFound) {
			slog.Debug("no card found for user", "user_id", userID, "chat_id", chatID)
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "No card available for you yet. Cards are generated weekly.",
			})
			return
		}

		slog.Error("failed to get card image", "user_id", userID, "chat_id", chatID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Failed to fetch your card. Please try again later.",
		})
		return
	}

	slog.Info("sending card image", "user_id", userID, "chat_id", chatID, "theme", cardImage.Theme, "week", cardImage.WeekStart)

	// Send the image using the presigned URL
	_, err = b.SendPhoto(ctx, &bot.SendPhotoParams{
		ChatID: chatID,
		Photo:  &models.InputFileString{Data: cardImage.URL},
	})
	if err != nil {
		slog.Error("failed to send photo", "user_id", userID, "chat_id", chatID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Failed to send the card image. Please try again later.",
		})
		return
	}

	slog.Debug("card image sent successfully", "user_id", userID, "chat_id", chatID)
}
