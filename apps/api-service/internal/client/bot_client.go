package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
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
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + "/internal/update-match-message"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
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
