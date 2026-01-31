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

// HandleMatchOrJoin is the unified handler for both /match and /join commands.
// It checks if an open match exists in the chat and either joins it or creates a new one.
func (h *MatchHandler) HandleMatchOrJoin(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID
	chatType := update.Message.Chat.Type

	slog.Debug("received match or join command", "chat_id", chatID, "user_id", userID, "chat_type", chatType)

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
		txn = h.nrApp.StartTransaction("bot:match-or-join-command")
		defer txn.End()
		ctx = newrelic.NewContext(ctx, txn)
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("user_id", userID)
	}

	// Check for existing open match
	match, err := h.apiClient.GetChatOpenMatch(ctx, chatID)
	if err != nil {
		slog.Error("failed to check for open match", "chat_id", chatID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Failed to check for existing matches. Please try again later.",
		})
		return
	}

	// Branch based on whether match exists
	if match == nil {
		// No existing match - create new one
		h.createNewMatch(ctx, b, update, chatID, userID, txn)
	} else {
		// Match exists - join it
		h.joinExistingMatch(ctx, b, update, match, userID, txn)
	}
}

// Handle processes the /match command
func (h *MatchHandler) Handle(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Delegate to the unified HandleMatchOrJoin method
	h.HandleMatchOrJoin(ctx, b, update)
}

// createNewMatch creates a new match and sends the game modal.
// This is a helper function for HandleMatchOrJoin (task 6.2).
func (h *MatchHandler) createNewMatch(ctx context.Context, b *bot.Bot, update *models.Update, chatID int64, userID int64, txn *newrelic.Transaction) {
	// 1. Create match via API
	match, err := h.apiClient.CreateArenaMatch(ctx, chatID, userID)
	if err != nil {
		// 4. Handle ErrActiveMatchExists by falling back to joinExistingMatch
		if errors.Is(err, client.ErrActiveMatchExists) {
			slog.Debug("active match exists, fetching and joining", "chat_id", chatID, "user_id", userID)
			// Fetch the existing match and join it
			existingMatch, fetchErr := h.apiClient.GetChatOpenMatch(ctx, chatID)
			if fetchErr != nil {
				slog.Error("failed to fetch existing match after ErrActiveMatchExists", "chat_id", chatID, "error", fetchErr)
				if txn != nil {
					txn.NoticeError(fetchErr)
				}
				b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: chatID,
					Text:   "Failed to join existing match. Please try again later.",
				})
				return
			}
			if existingMatch == nil {
				slog.Error("got ErrActiveMatchExists but GetChatOpenMatch returned nil", "chat_id", chatID)
				b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: chatID,
					Text:   "Failed to find existing match. Please try again later.",
				})
				return
			}
			// Join the existing match
			h.joinExistingMatch(ctx, b, update, existingMatch, userID, txn)
			return
		}

		// Handle ErrNotEnoughCards
		if errors.Is(err, client.ErrNotEnoughCards) {
			slog.Debug("not enough cards for match", "chat_id", chatID)
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: chatID,
				Text:   "Not enough cards in this group. Need at least 10 cards to start a match.",
			})
			return
		}

		// Other errors
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

	// 2. Send game modal via Telegram
	// Check if Games API is configured
	if h.gameShortName == "" {
		slog.Warn("ARENA_GAME_SHORT_NAME not configured, Games API unavailable", "chat_id", chatID)
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "The arena feature is not configured. Please contact the administrator.",
		})
		return
	}

	// Build keyboard using shared function with match participants
	keyboard := BuildMatchKeyboard(match.ID, match.Participants)

	// Send game message using SendGame (Games API)
	msg, err := b.SendGame(ctx, &bot.SendGameParams{
		ChatID:       chatID,
		GameShorName: h.gameShortName,
		ReplyMarkup:  keyboard,
	})
	if err != nil {
		slog.Error("failed to send game message", "match_id", match.ID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		return
	}

	// 3. Save telegram message ID via SetMatchTelegramMessageID
	if msg != nil && msg.ID != 0 {
		err = h.apiClient.SetMatchTelegramMessageID(ctx, match.ID, int64(msg.ID))
		if err != nil {
			slog.Error("failed to save telegram message ID", "match_id", match.ID, "message_id", msg.ID, "error", err)
			if txn != nil {
				txn.NoticeError(err)
			}
			// Don't return - message was sent successfully, this is just metadata
		} else {
			slog.Debug("saved telegram message ID", "match_id", match.ID, "message_id", msg.ID)
		}
	}
}

// joinExistingMatch joins an existing match and updates the game modal.
// This is a helper function for HandleMatchOrJoin (task 6.3).
func (h *MatchHandler) joinExistingMatch(ctx context.Context, b *bot.Bot, update *models.Update, match *client.ArenaMatch, userID int64, txn *newrelic.Transaction) {
	// 1. Join the existing match via API
	updatedMatch, err := h.apiClient.JoinArenaMatch(ctx, match.ID, userID)
	if err != nil {
		// 2. If user already joined, send "You've already joined this match!" message
		if errors.Is(err, client.ErrAlreadyJoined) {
			slog.Debug("user already joined match", "match_id", match.ID, "user_id", userID)
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: match.ChatID,
				Text:   "You've already joined this match! Click 'Open Game' to continue.",
			})
			return
		}

		// Other errors
		slog.Error("failed to join match", "match_id", match.ID, "user_id", userID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: match.ChatID,
			Text:   "Failed to join match. Please try again later.",
		})
		return
	}

	slog.Info("user joined match", "match_id", match.ID, "user_id", userID, "participant_count", len(updatedMatch.Participants))
	if txn != nil {
		txn.AddAttribute("match_id", match.ID)
		txn.AddAttribute("participant_count", len(updatedMatch.Participants))
	}

	// 3. Call updateMatchMessage to edit the game modal (if we have a message ID)
	if updatedMatch.TelegramMessageID != nil && *updatedMatch.TelegramMessageID != 0 {
		h.updateMatchMessage(ctx, b, updatedMatch.ChatID, int(*updatedMatch.TelegramMessageID), updatedMatch.Participants, updatedMatch.ID)
	} else {
		slog.Warn("no telegram message ID to update", "match_id", match.ID)
	}

	// 4. Send notification message
	// Get the username from the update
	username := ""
	if update.Message != nil && update.Message.From != nil {
		if update.Message.From.Username != "" {
			username = "@" + update.Message.From.Username
		} else if update.Message.From.FirstName != "" {
			username = update.Message.From.FirstName
		} else {
			username = fmt.Sprintf("User %d", userID)
		}
	} else {
		username = fmt.Sprintf("User %d", userID)
	}

	notificationText := fmt.Sprintf("**%s** joined the match!\n\n👥 Participants: %d\n\nUse /join to enter the arena!", username, len(updatedMatch.Participants))
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    match.ChatID,
		Text:      notificationText,
		ParseMode: models.ParseModeMarkdown,
	})
	if err != nil {
		slog.Error("failed to send join notification", "match_id", match.ID, "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		// Don't return - join was successful, notification is just nice to have
	}
}

// updateMatchMessage edits the game modal to update the participant list.
// This is a helper function for joinExistingMatch (task 6.4).
func (h *MatchHandler) updateMatchMessage(ctx context.Context, b *bot.Bot, chatID int64, messageID int, participants []client.ArenaParticipant, matchID string) {
	// Build keyboard using shared function
	keyboard := BuildMatchKeyboard(matchID, participants)

	// Edit the message's reply markup (inline keyboard)
	_, err := b.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
		ChatID:      chatID,
		MessageID:   messageID,
		ReplyMarkup: keyboard,
	})
	if err != nil {
		// Log error but don't fail - the join notification provides the count anyway
		slog.Warn("failed to update match message", "chat_id", chatID, "message_id", messageID, "error", err)
	} else {
		slog.Debug("updated match message keyboard", "chat_id", chatID, "message_id", messageID, "participant_count", len(participants))
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

// BuildMatchKeyboard creates the inline keyboard for a match with 3 rows:
// Row 1: Open Game button
// Row 2: Join Game button with participant count
// Row 3: Participant list
func BuildMatchKeyboard(matchID string, participants []client.ArenaParticipant) *models.InlineKeyboardMarkup {
	participantList := FormatParticipantList(participants)

	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "🎮 Open", CallbackGame: &models.CallbackGame{}},
			},
			{
				{Text: fmt.Sprintf("➕ Join (%d)", len(participants)), CallbackData: fmt.Sprintf("join_match:%s", matchID)},
			},
			{
				{Text: fmt.Sprintf("👥 %s", participantList), CallbackData: "noop"},
			},
		},
	}
}
