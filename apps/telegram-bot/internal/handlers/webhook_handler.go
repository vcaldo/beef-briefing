package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"beef-briefing/apps/telegram-bot/internal/client"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// winnerRevealDelay is the grace period before revealing the battle winner
// in the group message, allowing users to open the mini-app first.
const winnerRevealDelay = 45 * time.Second

// WebhookHandler handles internal webhook requests for updating match messages
type WebhookHandler struct {
	apiClient *client.APIClient
	bot       *bot.Bot
	nrApp     *newrelic.Application
}

// NewWebhookHandler creates a new WebhookHandler
func NewWebhookHandler(apiClient *client.APIClient, b *bot.Bot, nrApp *newrelic.Application) *WebhookHandler {
	return &WebhookHandler{
		apiClient: apiClient,
		bot:       b,
		nrApp:     nrApp,
	}
}

// UpdateMatchMessageRequest is the request body for updating a match message
type UpdateMatchMessageRequest struct {
	MatchID           string `json:"match_id"`
	ChatID            int64  `json:"chat_id"`
	TelegramMessageID int64  `json:"telegram_message_id"`
}

// HandleUpdateMatchMessage handles POST /internal/update-match-message
// This endpoint is called by the API service when a participant joins/leaves via Mini App
func (h *WebhookHandler) HandleUpdateMatchMessage(w http.ResponseWriter, r *http.Request) {
	// Start New Relic transaction
	var txn *newrelic.Transaction
	if h.nrApp != nil {
		txn = h.nrApp.StartTransaction("bot:webhook-update-match")
		defer txn.End()
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UpdateMatchMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("failed to decode webhook request", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.MatchID == "" || req.ChatID == 0 || req.TelegramMessageID == 0 {
		slog.Warn("webhook request missing required fields", "match_id", req.MatchID, "chat_id", req.ChatID, "telegram_message_id", req.TelegramMessageID)
		http.Error(w, "match_id, chat_id, and telegram_message_id are required", http.StatusBadRequest)
		return
	}

	// Add attributes after parsing request
	if txn != nil {
		txn.AddAttribute("match_id", req.MatchID)
		txn.AddAttribute("chat_id", req.ChatID)
		txn.AddAttribute("telegram_message_id", req.TelegramMessageID)
	}

	slog.Info("received update match message webhook", "match_id", req.MatchID, "chat_id", req.ChatID, "telegram_message_id", req.TelegramMessageID)

	// Create context with transaction for downstream calls
	ctx := r.Context()
	if txn != nil {
		ctx = newrelic.NewContext(ctx, txn)
	}

	// Track API call segment
	var apiSegment *newrelic.Segment
	if txn != nil {
		apiSegment = txn.StartSegment("webhook:get-match")
	}
	match, err := h.apiClient.GetArenaMatch(ctx, req.MatchID)
	if apiSegment != nil {
		apiSegment.End()
	}
	if err != nil {
		if txn != nil {
			txn.NoticeError(err)
		}
		slog.Error("failed to get match for webhook update", "match_id", req.MatchID, "error", err)
		http.Error(w, "failed to get match", http.StatusInternalServerError)
		return
	}

	// For completed matches, show a spoiler keyboard first, then reveal the winner after a delay
	if match.Status == "completed" {
		spoilerKeyboard := &models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{
					{Text: "📺 Watch Battle", CallbackGame: &models.CallbackGame{}},
				},
				{
					{Text: "⚔️ Battle in progress...", CallbackData: "noop"},
				},
			},
		}

		var botSegment *newrelic.Segment
		if txn != nil {
			botSegment = txn.StartSegment("webhook:edit-message-spoiler")
		}
		_, err = h.bot.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
			ChatID:      req.ChatID,
			MessageID:   int(req.TelegramMessageID),
			ReplyMarkup: spoilerKeyboard,
		})
		if botSegment != nil {
			botSegment.End()
		}
		if err != nil {
			if txn != nil {
				txn.NoticeError(err)
			}
			slog.Error("failed to update match message with spoiler", "match_id", req.MatchID, "chat_id", req.ChatID, "error", err)
		} else {
			slog.Info("updated match message with spoiler", "match_id", req.MatchID, "chat_id", req.ChatID)
		}

		// Reveal the real winner after a delay
		go h.revealWinner(match, req.ChatID, req.TelegramMessageID)
	} else {
		// Non-completed match: update keyboard normally
		keyboard := BuildMatchKeyboard(match.ID, match)

		var botSegment *newrelic.Segment
		if txn != nil {
			botSegment = txn.StartSegment("webhook:edit-message")
		}
		_, err = h.bot.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
			ChatID:      req.ChatID,
			MessageID:   int(req.TelegramMessageID),
			ReplyMarkup: keyboard,
		})
		if botSegment != nil {
			botSegment.End()
		}
		if err != nil {
			if txn != nil {
				txn.NoticeError(err)
			}
			slog.Error("failed to update match message via webhook", "match_id", req.MatchID, "chat_id", req.ChatID, "telegram_message_id", req.TelegramMessageID, "error", err)
		} else {
			slog.Info("updated match message via webhook", "match_id", req.MatchID, "chat_id", req.ChatID, "participant_count", len(match.Participants))
		}
	}

	// Add success attribute
	if txn != nil {
		txn.AddAttribute("participant_count", len(match.Participants))
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// revealWinner waits for the grace period then updates the match message
// with the actual winner, replacing the "???" spoiler.
func (h *WebhookHandler) revealWinner(match *client.ArenaMatch, chatID int64, telegramMessageID int64) {
	time.Sleep(winnerRevealDelay)

	keyboard := BuildMatchKeyboard(match.ID, match)

	_, err := h.bot.EditMessageReplyMarkup(context.Background(), &bot.EditMessageReplyMarkupParams{
		ChatID:      chatID,
		MessageID:   int(telegramMessageID),
		ReplyMarkup: keyboard,
	})
	if err != nil {
		slog.Error("failed to reveal winner on match message", "match_id", match.ID, "chat_id", chatID, "error", err)
	} else {
		slog.Info("revealed winner on match message", "match_id", match.ID, "chat_id", chatID)
	}
}

// BattleResultRequest is the request body for battle result DM notifications
type BattleResultRequest struct {
	MatchID   string `json:"match_id"`
	ChatID    int64  `json:"chat_id"`
	MatchType string `json:"match_type"` // "regular" or "ranked"
	Format    string `json:"format"`     // "1v1" or "arena"

	// 1v1 battle results
	PlayerAID   int64  `json:"player_a_id"`
	PlayerBID   int64  `json:"player_b_id"`
	PlayerAName string `json:"player_a_name"`
	PlayerBName string `json:"player_b_name"`
	WinnerID    *int64 `json:"winner_id,omitempty"`
	IsDraw      bool   `json:"is_draw"`
	TeamADamage int    `json:"team_a_damage"`
	TeamBDamage int    `json:"team_b_damage"`
	NumRounds   int    `json:"num_rounds"`

	// Arena tournament results (multi-player)
	Participants []BattleParticipantResult `json:"participants,omitempty"`
}

// BattleParticipantResult represents a participant's tournament results
type BattleParticipantResult struct {
	UserID      int64  `json:"user_id"`
	Name        string `json:"name"`
	Wins        int    `json:"wins"`
	TotalDamage int    `json:"total_damage"`
	Rank        int    `json:"rank"`
}

// HandleBattleResultNotification handles POST /internal/notify-battle-result
// This endpoint is called by the API service to send DMs to players after battle completion
func (h *WebhookHandler) HandleBattleResultNotification(w http.ResponseWriter, r *http.Request) {
	// Start New Relic transaction
	var txn *newrelic.Transaction
	if h.nrApp != nil {
		txn = h.nrApp.StartTransaction("bot:webhook-battle-result")
		defer txn.End()
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req BattleResultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("failed to decode battle result request", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.MatchID == "" {
		slog.Warn("battle result request missing match_id")
		http.Error(w, "match_id is required", http.StatusBadRequest)
		return
	}

	// Add attributes after parsing request
	if txn != nil {
		txn.AddAttribute("match_id", req.MatchID)
		txn.AddAttribute("format", req.Format)
	}

	slog.Info("received battle result notification", "match_id", req.MatchID, "format", req.Format)

	// Create context for sending DMs
	ctx := r.Context()
	if txn != nil {
		ctx = newrelic.NewContext(ctx, txn)
	}

	// Send DMs asynchronously based on format
	if req.Format == "1v1" {
		go h.send1v1ResultDMs(ctx, req)
	} else if req.Format == "arena" {
		go h.sendArenaResultDMs(ctx, req)
	}

	// Return success immediately (best-effort delivery)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// send1v1ResultDMs sends battle result DMs to both players in a 1v1 match
func (h *WebhookHandler) send1v1ResultDMs(ctx context.Context, req BattleResultRequest) {
	// Send to Player A
	go h.sendBattleResultDM(ctx, req.PlayerAID, h.build1v1Message(req, req.PlayerAID))

	// Send to Player B
	go h.sendBattleResultDM(ctx, req.PlayerBID, h.build1v1Message(req, req.PlayerBID))
}

// sendArenaResultDMs sends tournament result DMs to all participants
func (h *WebhookHandler) sendArenaResultDMs(ctx context.Context, req BattleResultRequest) {
	for _, p := range req.Participants {
		go h.sendBattleResultDM(ctx, p.UserID, h.buildArenaMessage(req, p))
	}
}

// sendBattleResultDM sends a single DM to a user with battle results
func (h *WebhookHandler) sendBattleResultDM(ctx context.Context, userID int64, text string) {
	_, err := h.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    userID,
		Text:      text,
		ParseMode: "Markdown",
	})

	if err != nil {
		// Expected if user hasn't started chat with bot - log at debug level
		slog.Debug("failed to send battle result DM", "user_id", userID, "error", err)
	} else {
		slog.Info("sent battle result DM", "user_id", userID)
	}
}

// build1v1Message builds a DM message for a 1v1 battle result
func (h *WebhookHandler) build1v1Message(req BattleResultRequest, userID int64) string {
	isPlayerA := userID == req.PlayerAID
	var opponentName string
	var damageDealt, damageTaken int

	if isPlayerA {
		opponentName = req.PlayerBName
		damageDealt = req.TeamADamage
		damageTaken = req.TeamBDamage
	} else {
		opponentName = req.PlayerAName
		damageDealt = req.TeamBDamage
		damageTaken = req.TeamADamage
	}

	if req.IsDraw {
		return fmt.Sprintf(`🤝 *Draw!*

The battle with %s ended in a stalemate.

📊 Battle Stats:
• Your Damage: %d
• Opponent Damage: %d
• Combat Rounds: %d

🎫 Match: %s`, opponentName, damageDealt, damageTaken, req.NumRounds, req.MatchID[:8])
	}

	isWinner := req.WinnerID != nil && *req.WinnerID == userID

	if isWinner {
		return fmt.Sprintf(`🏆 *Victory!*

You defeated %s!

📊 Battle Stats:
• Damage Dealt: %d
• Damage Taken: %d
• Combat Rounds: %d

🎫 Match: %s`, opponentName, damageDealt, damageTaken, req.NumRounds, req.MatchID[:8])
	}

	return fmt.Sprintf(`💀 *Defeat*

%s emerged victorious.

📊 Battle Stats:
• Damage Dealt: %d
• Damage Taken: %d
• Combat Rounds: %d

Better luck next time!

🎫 Match: %s`, opponentName, damageDealt, damageTaken, req.NumRounds, req.MatchID[:8])
}

// buildArenaMessage builds a DM message for an arena tournament result
func (h *WebhookHandler) buildArenaMessage(req BattleResultRequest, participant BattleParticipantResult) string {
	isWinner := req.WinnerID != nil && *req.WinnerID == participant.UserID

	if isWinner {
		return fmt.Sprintf(`🏆 *Tournament Champion!*

You won the arena tournament!

🎖️ Your Record: %d wins, %d damage
⚔️ Total Rounds: %d

🎫 Match: %s`, participant.Wins, participant.TotalDamage, req.NumRounds, req.MatchID[:8])
	}

	// Find winner name
	var winnerName string
	for _, p := range req.Participants {
		if req.WinnerID != nil && p.UserID == *req.WinnerID {
			winnerName = p.Name
			break
		}
	}

	return fmt.Sprintf(`⚔️ *Tournament Complete*

%s won the tournament!

🎖️ Your Record: %d wins, %d damage (#%d)
⚔️ Total Rounds: %d

🎫 Match: %s`, winnerName, participant.Wins, participant.TotalDamage, participant.Rank, req.NumRounds, req.MatchID[:8])
}
