package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"beef-briefing/apps/telegram-bot/internal/client"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// MatchHandler handles the /match command
type MatchHandler struct {
	apiClient     *client.APIClient
	nrApp         *newrelic.Application
	botUsername   string
	gameShortName string
	arenaBaseURL  string
}

// NewMatchHandler creates a new MatchHandler
func NewMatchHandler(apiClient *client.APIClient, nrApp *newrelic.Application, gameShortName, arenaBaseURL string) *MatchHandler {
	return &MatchHandler{
		apiClient:     apiClient,
		nrApp:         nrApp,
		botUsername:   os.Getenv("TELEGRAM_BOT_USERNAME"),
		gameShortName: gameShortName,
		arenaBaseURL:  arenaBaseURL,
	}
}

// Handle processes the /match command
func (h *MatchHandler) Handle(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID
	chatType := update.Message.Chat.Type

	slog.Debug("received /match command", "chat_id", chatID, "user_id", userID, "chat_type", chatType)

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
		txn = h.nrApp.StartTransaction("bot:match-command")
		defer txn.End()
		ctx = newrelic.NewContext(ctx, txn)
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("user_id", userID)
	}

	// Create match via API
	match, err := h.apiClient.CreateArenaMatch(ctx, chatID, userID)
	if err != nil {
		if errors.Is(err, client.ErrNotEnoughCards) {
			slog.Debug("not enough cards for match", "chat_id", chatID)
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "Not enough cards in this group. Need at least 10 cards to start a match.",
			})
			return
		}

		slog.Error("failed to create match", "chat_id", chatID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Failed to create match. Please try again later.",
		})
		return
	}

	slog.Info("match created", "match_id", match.ID, "chat_id", chatID, "creator", userID)
	if txn != nil {
		txn.AddAttribute("match_id", match.ID)
	}

	// Check if Games API is configured
	if h.gameShortName == "" {
		slog.Warn("ARENA_GAME_SHORT_NAME not configured, Games API unavailable", "chat_id", chatID)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "The arena feature is not configured. Please contact the administrator.",
		})
		return
	}

	// Create inline keyboard with game callback buttons
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

	// Send game message using SendGame (Games API)
	// Note: The library has a typo - it's GameShorName not GameShortName
	_, err = b.SendGame(ctx, &bot.SendGameParams{
		ChatID:       chatID,
		GameShorName: h.gameShortName, // Note: typo in library field name
		ReplyMarkup:  keyboard,
	})
	if err != nil {
		slog.Error("failed to send game message", "match_id", match.ID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
	}
}

// FormatParticipantList formats the list of participants for display
func FormatParticipantList(participants []client.ArenaParticipant) string {
	if len(participants) == 0 {
		return "None"
	}

	result := ""
	for i, p := range participants {
		name := p.Name
		if p.Username != "" {
			name = "@" + p.Username
		}
		if name == "" {
			name = fmt.Sprintf("User %d", p.UserID)
		}
		if i > 0 {
			result += ", "
		}
		result += name
	}
	return result
}
