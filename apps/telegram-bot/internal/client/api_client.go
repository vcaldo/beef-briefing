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
	"time"

	"beef-briefing/apps/telegram-bot/internal"
)

type APIClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewAPIClient(baseURL string) *APIClient {
	return &APIClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: internal.DefaultHTTPTimeout,
		},
	}
}

// SendUpdate sends an update with optional media files to the API service
func (c *APIClient) SendUpdate(ctx context.Context, update interface{}, files map[string][]byte) error {
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
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := c.sendUpdateOnce(ctx, update, files)
		if err == nil {
			if attempt > 0 {
				slog.Info("API request succeeded after retry", "attempt", attempt+1)
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
			return fmt.Errorf("client error, not retrying: %w", err)
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", internal.MaxRetryAttempts, lastErr)
}

func (c *APIClient) sendUpdateOnce(ctx context.Context, update interface{}, files map[string][]byte) error {
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
	url := fmt.Sprintf("%s/api/v1/ingest", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	resp, err := c.httpClient.Do(req)
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
