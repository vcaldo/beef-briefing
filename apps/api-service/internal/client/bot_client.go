package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"beef-briefing/apps/api-service/internal/nrutil"

	"github.com/newrelic/go-agent/v3/newrelic"
)

// BotClient is a client for calling the telegram-bot internal webhook
type BotClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewBotClient creates a new BotClient
// baseURL should be like "http://telegram-bot:8081"
func NewBotClient(baseURL, apiKey string) *BotClient {
	return &BotClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// UpdateMatchMessageRequest is the request body for updating a match message
type UpdateMatchMessageRequest struct {
	MatchID           string `json:"match_id"`
	ChatID            int64  `json:"chat_id"`
	TelegramMessageID int64  `json:"telegram_message_id"`
}

// BattleResultNotification is the request body for battle result DM notifications
type BattleResultNotification struct {
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

// NotifyParticipantChange notifies the bot that a match participant has joined or left.
// This triggers the bot to update the Telegram message with the new participant list.
func (c *BotClient) NotifyParticipantChange(ctx context.Context, matchID string, chatID, telegramMessageID int64) error {
	// Start segment for overall operation
	defer nrutil.StartSegment(ctx, "bot-client:notify-participant-change")()

	if c.baseURL == "" {
		slog.Debug("bot client not configured, skipping participant change notification")
		return nil
	}

	if telegramMessageID == 0 {
		slog.Debug("no telegram message ID, skipping participant change notification", "match_id", matchID)
		return nil
	}

	req := UpdateMatchMessageRequest{
		MatchID:           matchID,
		ChatID:            chatID,
		TelegramMessageID: telegramMessageID,
	}

	body, err := json.Marshal(req)
	if err != nil {
		nrutil.NoticeError(ctx, err)
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := c.baseURL + "/internal/update-match-message"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		nrutil.NoticeError(ctx, err)
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	// Track external HTTP call with detailed segment
	txn := newrelic.FromContext(ctx)
	var externalSegment *newrelic.ExternalSegment
	if txn != nil {
		parsedURL, _ := url.Parse(apiURL)
		host := c.baseURL
		if parsedURL != nil {
			host = parsedURL.Host
		}
		externalSegment = &newrelic.ExternalSegment{
			StartTime: txn.StartSegmentNow(),
			URL:       apiURL,
			Host:      host,
			Procedure: "POST",
			Library:   "net/http",
		}
		txn.AddAttribute("webhook_match_id", matchID)
		txn.AddAttribute("webhook_chat_id", chatID)
	}

	resp, err := c.httpClient.Do(httpReq)

	if externalSegment != nil {
		if resp != nil {
			externalSegment.Response = resp
		}
		externalSegment.End()
	}

	if err != nil {
		nrutil.NoticeError(ctx, err)
		// Log but don't fail - this is a best-effort notification
		slog.Warn("failed to notify bot of participant change", "match_id", matchID, "error", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("bot returned non-OK status for participant change", "match_id", matchID, "status", resp.StatusCode)
		return nil
	}

	slog.Debug("notified bot of participant change", "match_id", matchID, "chat_id", chatID)
	return nil
}

// NotifyBattleComplete notifies the bot to send DMs to players with battle results.
// This is a best-effort notification - failures are logged but don't affect battle completion.
func (c *BotClient) NotifyBattleComplete(ctx context.Context, notification *BattleResultNotification) error {
	defer nrutil.StartSegment(ctx, "bot-client:notify-battle-complete")()

	if c.baseURL == "" {
		slog.Debug("bot client not configured, skipping battle result notification")
		return nil
	}

	body, err := json.Marshal(notification)
	if err != nil {
		nrutil.NoticeError(ctx, err)
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := c.baseURL + "/internal/notify-battle-result"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		nrutil.NoticeError(ctx, err)
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	// Track external HTTP call with detailed segment
	txn := newrelic.FromContext(ctx)
	var externalSegment *newrelic.ExternalSegment
	if txn != nil {
		parsedURL, _ := url.Parse(apiURL)
		host := c.baseURL
		if parsedURL != nil {
			host = parsedURL.Host
		}
		externalSegment = &newrelic.ExternalSegment{
			StartTime: txn.StartSegmentNow(),
			URL:       apiURL,
			Host:      host,
			Procedure: "POST",
			Library:   "net/http",
		}
		txn.AddAttribute("webhook_match_id", notification.MatchID)
		txn.AddAttribute("webhook_format", notification.Format)
	}

	resp, err := c.httpClient.Do(httpReq)

	if externalSegment != nil {
		if resp != nil {
			externalSegment.Response = resp
		}
		externalSegment.End()
	}

	if err != nil {
		nrutil.NoticeError(ctx, err)
		// Log but don't fail - this is a best-effort notification
		slog.Warn("failed to notify bot of battle result", "match_id", notification.MatchID, "error", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("bot returned non-OK status for battle result notification", "match_id", notification.MatchID, "status", resp.StatusCode)
		return nil
	}

	slog.Debug("notified bot of battle result", "match_id", notification.MatchID, "format", notification.Format)
	return nil
}
