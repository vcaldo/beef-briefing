package telegram

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const telegramAPIBase = "https://api.telegram.org/bot"

// UserInfo contains information about a Telegram user
type UserInfo struct {
	ID        int64
	IsBot     bool
	FirstName string
	LastName  string
	Username  string
}

// Client handles Telegram Bot API requests with caching
type Client struct {
	token      string
	httpClient *http.Client
	cache      map[int64]*UserInfo // userID -> UserInfo
	cacheMu    sync.RWMutex
	chatID     int64 // The chat to query for members
	enabled    bool  // Whether API lookups are enabled
}

// NewClient creates a new Telegram API client
func NewClient(token string, chatID int64) *Client {
	enabled := token != ""
	if enabled {
		slog.Info("telegram API client initialized", "chat_id", chatID)
	} else {
		slog.Info("telegram API client disabled (no token provided)")
	}

	return &Client{
		token: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		cache:   make(map[int64]*UserInfo),
		chatID:  chatID,
		enabled: enabled,
	}
}

// IsEnabled returns whether the client is enabled for API lookups
func (c *Client) IsEnabled() bool {
	return c.enabled
}

// GetUser retrieves user info, using cache if available
func (c *Client) GetUser(userID int64) (*UserInfo, error) {
	if !c.enabled {
		return nil, nil
	}

	// Check cache first
	c.cacheMu.RLock()
	if info, ok := c.cache[userID]; ok {
		c.cacheMu.RUnlock()
		return info, nil
	}
	c.cacheMu.RUnlock()

	// Query Telegram API
	info, err := c.fetchChatMember(userID)
	if err != nil {
		return nil, err
	}

	// Cache the result
	c.cacheMu.Lock()
	c.cache[userID] = info
	c.cacheMu.Unlock()

	return info, nil
}

// IsBot checks if a user is a bot, returns false if lookup fails or client disabled
func (c *Client) IsBot(userID int64) bool {
	info, err := c.GetUser(userID)
	if err != nil {
		slog.Debug("failed to check if user is bot", "user_id", userID, "error", err)
		return false
	}
	if info == nil {
		return false
	}
	return info.IsBot
}

// PreloadUser adds user info to cache without API call (for known non-bots)
func (c *Client) PreloadUser(userID int64, isBot bool, firstName string) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	c.cache[userID] = &UserInfo{
		ID:        userID,
		IsBot:     isBot,
		FirstName: firstName,
	}
}

// GetCacheStats returns cache statistics
func (c *Client) GetCacheStats() (total int, bots int) {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()

	total = len(c.cache)
	for _, info := range c.cache {
		if info.IsBot {
			bots++
		}
	}
	return
}

// getChatMemberResponse represents the Telegram API response for getChatMember
type getChatMemberResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
	Result      *struct {
		User struct {
			ID        int64  `json:"id"`
			IsBot     bool   `json:"is_bot"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name,omitempty"`
			Username  string `json:"username,omitempty"`
		} `json:"user"`
		Status string `json:"status"`
	} `json:"result,omitempty"`
}

// fetchChatMember queries the Telegram API for chat member info
func (c *Client) fetchChatMember(userID int64) (*UserInfo, error) {
	url := fmt.Sprintf("%s%s/getChatMember?chat_id=%d&user_id=%d",
		telegramAPIBase, c.token, c.chatID, userID)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var result getChatMemberResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if !result.OK {
		// User might not be a member anymore, return nil without error
		// This is common for users who left the group
		slog.Debug("getChatMember returned not OK",
			"user_id", userID,
			"description", result.Description,
		)
		return nil, nil
	}

	if result.Result == nil {
		return nil, nil
	}

	return &UserInfo{
		ID:        result.Result.User.ID,
		IsBot:     result.Result.User.IsBot,
		FirstName: result.Result.User.FirstName,
		LastName:  result.Result.User.LastName,
		Username:  result.Result.User.Username,
	}, nil
}
