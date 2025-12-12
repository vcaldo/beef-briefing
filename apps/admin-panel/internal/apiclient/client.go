package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
)

// Config holds API client configuration
type Config struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

// Client handles communication with the analytics API
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	nrApp      *newrelic.Application
}

// NewClient creates a new analytics API client
func NewClient(cfg Config, nrApp *newrelic.Application) *Client {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		baseURL:    strings.TrimSuffix(cfg.BaseURL, "/"),
		apiKey:     cfg.APIKey,
		nrApp:      nrApp,
	}
}

// APIError represents an error from the API
type APIError struct {
	StatusCode int
	Message    string
	Endpoint   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d on %s: %s", e.StatusCode, e.Endpoint, e.Message)
}

// doRequest performs an HTTP request and decodes the response
func (c *Client) doRequest(ctx context.Context, endpoint string, params url.Values, result interface{}) error {
	u := c.baseURL + endpoint
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	// Create external segment for New Relic if transaction exists in context
	txn := newrelic.FromContext(ctx)
	var segment *newrelic.ExternalSegment
	if txn != nil {
		segment = newrelic.StartExternalSegment(txn, req)
	}

	resp, err := c.httpClient.Do(req)

	// End segment after request completes
	if segment != nil {
		segment.Response = resp
		segment.End()
	}

	if err != nil {
		if txn != nil {
			txn.NoticeError(err)
		}
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.Unmarshal(body, &errResp)
		apiErr := &APIError{
			StatusCode: resp.StatusCode,
			Message:    errResp.Error,
			Endpoint:   endpoint,
		}
		if txn != nil {
			txn.NoticeError(apiErr)
		}
		return apiErr
	}

	// Parse response wrapper
	var wrapper struct {
		Data     json.RawMessage `json:"data"`
		Metadata Metadata        `json:"metadata"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return fmt.Errorf("decoding response wrapper: %w", err)
	}

	// Decode data into result
	if err := json.Unmarshal(wrapper.Data, result); err != nil {
		return fmt.Errorf("decoding response data: %w", err)
	}

	return nil
}

// formatTime formats a time.Time for API query parameters (RFC3339)
func formatTime(t time.Time) string {
	return t.Format(time.RFC3339)
}

// ============================================
// CHAT LISTING (no time range)
// ============================================

// ListChats returns all chats with summary statistics
func (c *Client) ListChats(ctx context.Context) ([]ChatSummary, error) {
	var chats []ChatSummary
	err := c.doRequest(ctx, "/api/v1/analytics/chats", nil, &chats)
	if err != nil {
		return nil, fmt.Errorf("listing chats: %w", err)
	}
	return chats, nil
}

// GetChat returns detailed information about a single chat
func (c *Client) GetChat(ctx context.Context, chatID int64) (*ChatDetail, error) {
	endpoint := fmt.Sprintf("/api/v1/analytics/chats/%d/info", chatID)
	var chat ChatDetail
	err := c.doRequest(ctx, endpoint, nil, &chat)
	if err != nil {
		return nil, fmt.Errorf("getting chat %d: %w", chatID, err)
	}
	return &chat, nil
}

// ============================================
// ANALYTICS (require time range)
// ============================================

// GetOverview returns chat overview statistics
func (c *Client) GetOverview(ctx context.Context, chatID int64, startDate, endDate time.Time) (*OverviewResponse, error) {
	endpoint := fmt.Sprintf("/api/v1/analytics/chats/%d/overview", chatID)
	params := url.Values{
		"start_date": {formatTime(startDate)},
		"end_date":   {formatTime(endDate)},
	}

	var overview OverviewResponse
	err := c.doRequest(ctx, endpoint, params, &overview)
	if err != nil {
		return nil, fmt.Errorf("getting overview: %w", err)
	}
	return &overview, nil
}

// GetLeaderboard returns user rankings
func (c *Client) GetLeaderboard(ctx context.Context, chatID int64, startDate, endDate time.Time, metric string, limit int) ([]LeaderboardEntry, error) {
	endpoint := fmt.Sprintf("/api/v1/analytics/chats/%d/leaderboard", chatID)
	params := url.Values{
		"start_date": {formatTime(startDate)},
		"end_date":   {formatTime(endDate)},
		"metric":     {metric},
		"limit":      {strconv.Itoa(limit)},
	}

	var entries []LeaderboardEntry
	err := c.doRequest(ctx, endpoint, params, &entries)
	if err != nil {
		return nil, fmt.Errorf("getting leaderboard: %w", err)
	}
	return entries, nil
}

// GetUserDetail returns detailed user statistics
func (c *Client) GetUserDetail(ctx context.Context, chatID, userID int64, startDate, endDate time.Time) (*UserDetailResponse, error) {
	endpoint := fmt.Sprintf("/api/v1/analytics/chats/%d/users/%d", chatID, userID)
	params := url.Values{
		"start_date": {formatTime(startDate)},
		"end_date":   {formatTime(endDate)},
	}

	var user UserDetailResponse
	err := c.doRequest(ctx, endpoint, params, &user)
	if err != nil {
		return nil, fmt.Errorf("getting user detail: %w", err)
	}
	return &user, nil
}

// GetTimeline returns activity timeline
func (c *Client) GetTimeline(ctx context.Context, chatID int64, startDate, endDate time.Time, granularity string) ([]TimelinePoint, error) {
	endpoint := fmt.Sprintf("/api/v1/analytics/chats/%d/timeline", chatID)
	params := url.Values{
		"start_date":  {formatTime(startDate)},
		"end_date":    {formatTime(endDate)},
		"granularity": {granularity},
	}

	var points []TimelinePoint
	err := c.doRequest(ctx, endpoint, params, &points)
	if err != nil {
		return nil, fmt.Errorf("getting timeline: %w", err)
	}
	return points, nil
}

// GetHeatmap returns daily activity heatmap
func (c *Client) GetHeatmap(ctx context.Context, chatID int64, startDate, endDate time.Time) ([]HeatmapDay, error) {
	endpoint := fmt.Sprintf("/api/v1/analytics/chats/%d/heatmap", chatID)
	params := url.Values{
		"start_date": {formatTime(startDate)},
		"end_date":   {formatTime(endDate)},
	}

	var days []HeatmapDay
	err := c.doRequest(ctx, endpoint, params, &days)
	if err != nil {
		return nil, fmt.Errorf("getting heatmap: %w", err)
	}
	return days, nil
}

// GetTopContent returns most reacted/replied messages
func (c *Client) GetTopContent(ctx context.Context, chatID int64, startDate, endDate time.Time, metric string, limit int) ([]TopMessage, error) {
	endpoint := fmt.Sprintf("/api/v1/analytics/chats/%d/top-content", chatID)
	params := url.Values{
		"start_date": {formatTime(startDate)},
		"end_date":   {formatTime(endDate)},
		"metric":     {metric},
		"limit":      {strconv.Itoa(limit)},
	}

	var messages []TopMessage
	err := c.doRequest(ctx, endpoint, params, &messages)
	if err != nil {
		return nil, fmt.Errorf("getting top content: %w", err)
	}
	return messages, nil
}

// CompareUsers returns comparison of multiple users
func (c *Client) CompareUsers(ctx context.Context, chatID int64, userIDs []int64, startDate, endDate time.Time) ([]UserComparison, error) {
	endpoint := fmt.Sprintf("/api/v1/analytics/chats/%d/compare", chatID)

	// Convert user IDs to comma-separated string
	userIDStrs := make([]string, len(userIDs))
	for i, id := range userIDs {
		userIDStrs[i] = strconv.FormatInt(id, 10)
	}

	params := url.Values{
		"start_date": {formatTime(startDate)},
		"end_date":   {formatTime(endDate)},
		"user_ids":   {strings.Join(userIDStrs, ",")},
	}

	var comparisons []UserComparison
	err := c.doRequest(ctx, endpoint, params, &comparisons)
	if err != nil {
		return nil, fmt.Errorf("comparing users: %w", err)
	}
	return comparisons, nil
}
