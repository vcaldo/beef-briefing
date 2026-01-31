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
