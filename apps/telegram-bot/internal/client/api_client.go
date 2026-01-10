package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"

	"beef-briefing/apps/telegram-bot/internal"

	"github.com/newrelic/go-agent/v3/newrelic"
)

type APIClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	nrApp      *newrelic.Application
}

func NewAPIClient(baseURL string, apiKey string, nrApp *newrelic.Application) *APIClient {
	return &APIClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: internal.DefaultHTTPTimeout,
		},
		nrApp: nrApp,
	}
}

// addAuthHeader adds the Authorization header to the request
func (c *APIClient) addAuthHeader(req *http.Request) {
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
}

// SendUpdate sends an update with optional media files to the API service
func (c *APIClient) SendUpdate(ctx context.Context, update interface{}, files map[string][]byte) error {
	// Get transaction from context
	txn := newrelic.FromContext(ctx)

	// Start segment for the overall send operation
	var sendSegment *newrelic.Segment
	if txn != nil {
		sendSegment = txn.StartSegment("api:send-update")
		defer sendSegment.End()
		txn.AddAttribute("file_attachments", len(files))
	}

	var lastErr error

	for attempt := 0; attempt < internal.MaxRetryAttempts; attempt++ {
		if attempt > 0 {
			delay := internal.RetryDelays[attempt-1]
			slog.Info("retrying API request",
				"attempt", attempt+1,
				"delay", delay,
			)
			select {
			case <-ctx.Done():
				if txn != nil {
					txn.NoticeError(ctx.Err())
				}
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := c.sendUpdateOnce(ctx, update, files)
		if err == nil {
			if attempt > 0 {
				slog.Info("API request succeeded after retry", "attempt", attempt+1)
				if txn != nil {
					txn.AddAttribute("retry_count", attempt)
				}
			}
			return nil
		}

		lastErr = err

		// Log the error with structured fields
		slog.Warn("API request failed",
			"attempt", attempt+1,
			"error", err,
		)

		// Don't retry on 4xx errors (client errors)
		if isClientError(err) {
			if txn != nil {
				txn.AddAttribute("error_type", "client_error")
				txn.NoticeError(err)
			}
			return fmt.Errorf("client error, not retrying: %w", err)
		}
	}

	if txn != nil {
		txn.AddAttribute("retry_count", internal.MaxRetryAttempts)
		txn.AddAttribute("error_type", "max_retries_exceeded")
		txn.NoticeError(lastErr)
	}
	return fmt.Errorf("failed after %d attempts: %w", internal.MaxRetryAttempts, lastErr)
}

func (c *APIClient) sendUpdateOnce(ctx context.Context, update interface{}, files map[string][]byte) error {
	txn := newrelic.FromContext(ctx)

	// Marshal update to JSON
	updateJSON, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("failed to marshal update: %w", err)
	}

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add update field
	if err := writer.WriteField("update", string(updateJSON)); err != nil {
		return fmt.Errorf("failed to write update field: %w", err)
	}

	// Add file attachments
	for fileID, fileData := range files {
		part, err := writer.CreateFormFile(fileID, fileID)
		if err != nil {
			return fmt.Errorf("failed to create form file for %s: %w", fileID, err)
		}

		if _, err := part.Write(fileData); err != nil {
			return fmt.Errorf("failed to write file data for %s: %w", fileID, err)
		}

		slog.Debug("added file to multipart form",
			"file_id", fileID,
			"size", len(fileData),
		)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create HTTP request
	apiURL := fmt.Sprintf("%s/api/v1/ingest", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, &buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.addAuthHeader(req)

	// External segment for API service call
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
	}

	// Send request
	resp, err := c.httpClient.Do(req)

	if externalSegment != nil {
		if resp != nil {
			externalSegment.Response = resp
		}
		externalSegment.End()
	}

	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for error details
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Warn("failed to read response body", "error", err)
		body = []byte("<unreadable>")
	}

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	slog.Debug("successfully sent update to API", "status", resp.StatusCode)
	return nil
}

// HTTPError represents an HTTP error response
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}

func isClientError(err error) bool {
	var httpErr *HTTPError
	if err == nil {
		return false
	}

	// Check if it's an HTTPError
	if e, ok := err.(*HTTPError); ok {
		httpErr = e
	} else {
		// Try to unwrap
		for err != nil {
			if e, ok := err.(*HTTPError); ok {
				httpErr = e
				break
			}
			// Try to unwrap using Unwrap interface
			if unwrapper, ok := err.(interface{ Unwrap() error }); ok {
				err = unwrapper.Unwrap()
			} else {
				break
			}
		}
	}

	if httpErr != nil && httpErr.StatusCode >= 400 && httpErr.StatusCode < 500 {
		return true
	}

	return false
}

// ProfilePhotoMeta represents metadata for a profile photo
type ProfilePhotoMeta struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	FileSize     *int64 `json:"file_size,omitempty"`
	PhotoSize    string `json:"photo_size,omitempty"` // For chat photos: "small" or "big"
}

// GetAllUsers returns all user IDs from the database
func (c *APIClient) GetAllUsers(ctx context.Context) ([]int64, error) {
	txn := newrelic.FromContext(ctx)

	apiURL := fmt.Sprintf("%s/api/v1/users", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.addAuthHeader(req)

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
			Procedure: "GET",
			Library:   "net/http",
		}
	}

	resp, err := c.httpClient.Do(req)

	if externalSegment != nil {
		if resp != nil {
			externalSegment.Response = resp
		}
		externalSegment.End()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	var result struct {
		UserIDs []int64 `json:"user_ids"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.UserIDs, nil
}

// GetAllChats returns all chat IDs from the database
func (c *APIClient) GetAllChats(ctx context.Context) ([]int64, error) {
	txn := newrelic.FromContext(ctx)

	apiURL := fmt.Sprintf("%s/api/v1/chats", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.addAuthHeader(req)

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
			Procedure: "GET",
			Library:   "net/http",
		}
	}

	resp, err := c.httpClient.Do(req)

	if externalSegment != nil {
		if resp != nil {
			externalSegment.Response = resp
		}
		externalSegment.End()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	var result struct {
		ChatIDs []int64 `json:"chat_ids"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.ChatIDs, nil
}

// SendUserProfilePhotos uploads user profile photos to the API
func (c *APIClient) SendUserProfilePhotos(ctx context.Context, userID int64, photos []ProfilePhotoMeta, files map[string][]byte) error {
	txn := newrelic.FromContext(ctx)

	metadata := struct {
		UserID int64              `json:"user_id"`
		Photos []ProfilePhotoMeta `json:"photos"`
	}{
		UserID: userID,
		Photos: photos,
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	if err := writer.WriteField("metadata", string(metadataJSON)); err != nil {
		return fmt.Errorf("failed to write metadata field: %w", err)
	}

	for fileID, fileData := range files {
		part, err := writer.CreateFormFile(fileID, fileID)
		if err != nil {
			return fmt.Errorf("failed to create form file for %s: %w", fileID, err)
		}
		if _, err := part.Write(fileData); err != nil {
			return fmt.Errorf("failed to write file data for %s: %w", fileID, err)
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	apiURL := fmt.Sprintf("%s/api/v1/profile-photos/user", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, &buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.addAuthHeader(req)

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
	}

	resp, err := c.httpClient.Do(req)

	if externalSegment != nil {
		if resp != nil {
			externalSegment.Response = resp
		}
		externalSegment.End()
	}

	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return &HTTPError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	return nil
}

// CardImageResponse represents the response from the card image endpoint
type CardImageResponse struct {
	ImageID     int64  `json:"image_id"`
	URL         string `json:"url"`
	ExpiresIn   int    `json:"expires_in"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Theme       string `json:"theme"`
	WeekStart   string `json:"week_start"`
	GeneratedAt string `json:"generated_at"`
}

// ErrCardNotFound is returned when no card exists for the user
var ErrCardNotFound = fmt.Errorf("card not found")

// GetCardImageURL fetches the presigned URL for a user's card image
func (c *APIClient) GetCardImageURL(ctx context.Context, userID, chatID int64) (*CardImageResponse, error) {
	txn := newrelic.FromContext(ctx)

	apiURL := fmt.Sprintf("%s/api/v1/cards/%d/image?chat_id=%d", c.baseURL, userID, chatID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.addAuthHeader(req)

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
			Procedure: "GET",
			Library:   "net/http",
		}
	}

	resp, err := c.httpClient.Do(req)

	if externalSegment != nil {
		if resp != nil {
			externalSegment.Response = resp
		}
		externalSegment.End()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrCardNotFound
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	var result CardImageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// SendChatProfilePhotos uploads chat profile photos to the API
func (c *APIClient) SendChatProfilePhotos(ctx context.Context, chatID int64, photos []ProfilePhotoMeta, files map[string][]byte) error {
	txn := newrelic.FromContext(ctx)

	metadata := struct {
		ChatID int64              `json:"chat_id"`
		Photos []ProfilePhotoMeta `json:"photos"`
	}{
		ChatID: chatID,
		Photos: photos,
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	if err := writer.WriteField("metadata", string(metadataJSON)); err != nil {
		return fmt.Errorf("failed to write metadata field: %w", err)
	}

	for fileID, fileData := range files {
		part, err := writer.CreateFormFile(fileID, fileID)
		if err != nil {
			return fmt.Errorf("failed to create form file for %s: %w", fileID, err)
		}
		if _, err := part.Write(fileData); err != nil {
			return fmt.Errorf("failed to write file data for %s: %w", fileID, err)
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	apiURL := fmt.Sprintf("%s/api/v1/profile-photos/chat", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, &buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.addAuthHeader(req)

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
	}

	resp, err := c.httpClient.Do(req)

	if externalSegment != nil {
		if resp != nil {
			externalSegment.Response = resp
		}
		externalSegment.End()
	}

	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return &HTTPError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	return nil
}

// =============================================================================
// Arena Game API Methods
// =============================================================================

// ArenaMatch represents a match from the arena API
type ArenaMatch struct {
	ID                string             `json:"id"`
	ChatID            int64              `json:"chat_id"`
	MatchType         string             `json:"match_type"`
	Format            string             `json:"format"`
	Status            string             `json:"status"`
	CreatorUserID     *int64             `json:"creator_user_id,omitempty"`
	JoinDeadline      *string            `json:"join_deadline,omitempty"`
	ShopPhaseDeadline *string            `json:"shop_phase_deadline,omitempty"`
	CreatedAt         string             `json:"created_at"`
	Participants      []ArenaParticipant `json:"participants"`
	CardCount         int                `json:"card_count"`
}

// ArenaParticipant represents a participant in a match
type ArenaParticipant struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username,omitempty"`
	Name     string `json:"name,omitempty"`
	Status   string `json:"status"`
}

// PendingMatch represents a match needing action
type PendingMatch struct {
	ArenaMatch
	Action           string `json:"action"`
	ParticipantCount int    `json:"participant_count"`
}

// AutoStartResult represents the result of auto-starting a match
type AutoStartResult struct {
	MatchID      string `json:"match_id"`
	ChatID       int64  `json:"chat_id"`
	Action       string `json:"action"`
	Reason       string `json:"reason"`
	Participants int    `json:"participants"`
}

// ForceSubmitResult represents the result of force-submitting teams
type ForceSubmitResult struct {
	MatchID       string  `json:"match_id"`
	ChatID        int64   `json:"chat_id"`
	ForcedUsers   []int64 `json:"forced_users"`
	BattleStarted bool    `json:"battle_started"`
}

// Arena-specific errors
var (
	ErrMatchNotFound  = fmt.Errorf("match not found")
	ErrMatchNotOpen   = fmt.Errorf("match is not open")
	ErrNotEnoughCards = fmt.Errorf("not enough cards")
	ErrAlreadyJoined  = fmt.Errorf("already joined")
)

// CreateArenaMatch creates a new arena match
func (c *APIClient) CreateArenaMatch(ctx context.Context, chatID, creatorUserID int64) (*ArenaMatch, error) {
	txn := newrelic.FromContext(ctx)

	reqBody := struct {
		ChatID        int64 `json:"chat_id"`
		CreatorUserID int64 `json:"creator_user_id"`
	}{
		ChatID:        chatID,
		CreatorUserID: creatorUserID,
	}

	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := fmt.Sprintf("%s/api/v1/arena/match", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.addAuthHeader(req)

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
	}

	resp, err := c.httpClient.Do(req)

	if externalSegment != nil {
		if resp != nil {
			externalSegment.Response = resp
		}
		externalSegment.End()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		if bytes.Contains(body, []byte("not enough cards")) {
			return nil, ErrNotEnoughCards
		}
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	var result ArenaMatch
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetArenaMatch retrieves a match by ID
func (c *APIClient) GetArenaMatch(ctx context.Context, matchID string) (*ArenaMatch, error) {
	txn := newrelic.FromContext(ctx)

	apiURL := fmt.Sprintf("%s/api/v1/arena/match/%s", c.baseURL, matchID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.addAuthHeader(req)

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
			Procedure: "GET",
			Library:   "net/http",
		}
	}

	resp, err := c.httpClient.Do(req)

	if externalSegment != nil {
		if resp != nil {
			externalSegment.Response = resp
		}
		externalSegment.End()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrMatchNotFound
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	var result ArenaMatch
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// JoinArenaMatch joins a match
func (c *APIClient) JoinArenaMatch(ctx context.Context, matchID string, userID int64) (*ArenaMatch, error) {
	txn := newrelic.FromContext(ctx)

	reqBody := struct {
		UserID int64 `json:"user_id"`
	}{UserID: userID}

	bodyJSON, _ := json.Marshal(reqBody)

	apiURL := fmt.Sprintf("%s/api/v1/arena/match/%s/join", c.baseURL, matchID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.addAuthHeader(req)

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
	}

	resp, err := c.httpClient.Do(req)

	if externalSegment != nil {
		if resp != nil {
			externalSegment.Response = resp
		}
		externalSegment.End()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		if bytes.Contains(body, []byte("already joined")) {
			return nil, ErrAlreadyJoined
		}
		if bytes.Contains(body, []byte("not open")) {
			return nil, ErrMatchNotOpen
		}
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	var result ArenaMatch
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// LeaveArenaMatch leaves a match
func (c *APIClient) LeaveArenaMatch(ctx context.Context, matchID string, userID int64) error {
	txn := newrelic.FromContext(ctx)

	reqBody := struct {
		UserID int64 `json:"user_id"`
	}{UserID: userID}

	bodyJSON, _ := json.Marshal(reqBody)

	apiURL := fmt.Sprintf("%s/api/v1/arena/match/%s/leave", c.baseURL, matchID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.addAuthHeader(req)

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
	}

	resp, err := c.httpClient.Do(req)

	if externalSegment != nil {
		if resp != nil {
			externalSegment.Response = resp
		}
		externalSegment.End()
	}

	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &HTTPError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	return nil
}

// StartArenaMatch starts a match (creator only)
func (c *APIClient) StartArenaMatch(ctx context.Context, matchID string, userID int64) (*ArenaMatch, error) {
	txn := newrelic.FromContext(ctx)

	reqBody := struct {
		UserID int64 `json:"user_id"`
	}{UserID: userID}

	bodyJSON, _ := json.Marshal(reqBody)

	apiURL := fmt.Sprintf("%s/api/v1/arena/match/%s/start", c.baseURL, matchID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.addAuthHeader(req)

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
	}

	resp, err := c.httpClient.Do(req)

	if externalSegment != nil {
		if resp != nil {
			externalSegment.Response = resp
		}
		externalSegment.End()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	var result ArenaMatch
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetPendingMatches retrieves matches needing action
func (c *APIClient) GetPendingMatches(ctx context.Context) ([]PendingMatch, error) {
	txn := newrelic.FromContext(ctx)

	apiURL := fmt.Sprintf("%s/api/v1/arena/matches/pending", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.addAuthHeader(req)

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
			Procedure: "GET",
			Library:   "net/http",
		}
	}

	resp, err := c.httpClient.Do(req)

	if externalSegment != nil {
		if resp != nil {
			externalSegment.Response = resp
		}
		externalSegment.End()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	var result struct {
		Matches []PendingMatch `json:"matches"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Matches, nil
}

// AutoStartMatch auto-starts a match when deadline expires
func (c *APIClient) AutoStartMatch(ctx context.Context, matchID string) (*AutoStartResult, error) {
	txn := newrelic.FromContext(ctx)

	apiURL := fmt.Sprintf("%s/api/v1/arena/match/%s/auto-start", c.baseURL, matchID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.addAuthHeader(req)

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
	}

	resp, err := c.httpClient.Do(req)

	if externalSegment != nil {
		if resp != nil {
			externalSegment.Response = resp
		}
		externalSegment.End()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	var result AutoStartResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ForceSubmitTeams forces team submission for unready participants
func (c *APIClient) ForceSubmitTeams(ctx context.Context, matchID string) (*ForceSubmitResult, error) {
	txn := newrelic.FromContext(ctx)

	apiURL := fmt.Sprintf("%s/api/v1/arena/match/%s/force-submit", c.baseURL, matchID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.addAuthHeader(req)

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
	}

	resp, err := c.httpClient.Do(req)

	if externalSegment != nil {
		if resp != nil {
			externalSegment.Response = resp
		}
		externalSegment.End()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	var result ForceSubmitResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
