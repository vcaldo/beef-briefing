package telegram

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/go-telegram/bot"
)

type FileClient struct {
	bot *bot.Bot
}

func NewFileClient(botToken string) (*FileClient, error) {
	b, err := bot.New(botToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot client: %w", err)
	}

	return &FileClient{
		bot: b,
	}, nil
}

// DownloadFile downloads a file from Telegram with exponential backoff retry
func (fc *FileClient) DownloadFile(ctx context.Context, fileID string) ([]byte, string, error) {
	var data []byte
	var mimeType string
	var err error

	maxRetries := 3
	baseDelay := time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			slog.Warn("retrying file download",
				"file_id", fileID,
				"attempt", attempt,
				"delay", delay,
			)
			select {
			case <-ctx.Done():
				return nil, "", ctx.Err()
			case <-time.After(delay):
			}
		}

		data, mimeType, err = fc.downloadFileAttempt(ctx, fileID)
		if err == nil {
			return data, mimeType, nil
		}

		// Check if error is retryable (rate limit or temporary error)
		if !isRetryableError(err) {
			return nil, "", fmt.Errorf("non-retryable error downloading file: %w", err)
		}

		slog.Warn("file download failed, will retry",
			"file_id", fileID,
			"attempt", attempt,
			"error", err,
		)
	}

	return nil, "", fmt.Errorf("failed to download file after %d attempts: %w", maxRetries+1, err)
}

func (fc *FileClient) downloadFileAttempt(ctx context.Context, fileID string) ([]byte, string, error) {
	// Get file info from Telegram
	file, err := fc.bot.GetFile(ctx, &bot.GetFileParams{
		FileID: fileID,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get file info: %w", err)
	}

	// Download file content
	fileURL := fc.bot.FileDownloadLink(file)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read file data: %w", err)
	}

	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	return data, mimeType, nil
}

func isRetryableError(err error) bool {
	// Add logic to detect rate limiting (429) or temporary errors
	// For now, retry all errors
	return true
}
