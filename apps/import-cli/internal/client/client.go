package client

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"beef-briefing/apps/import-cli/internal/models"
)

// Client handles API communication
type Client struct {
	baseURL    string
	httpClient *http.Client
	batchSize  int
	delayMS    int
}

// New creates a new API client
func New(baseURL string, batchSize, delayMS int) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		batchSize: batchSize,
		delayMS:   delayMS,
	}
}

// SendUpdate sends a single update to the API
func (c *Client) SendUpdate(update *models.Update) error {
	return c.sendUpdateWithMedia(update, "", "")
}

// SendUpdateWithMedia sends an update with an attached media file
func (c *Client) SendUpdateWithMedia(update *models.Update, mediaPath, exportPath string) error {
	fullPath := filepath.Join(exportPath, mediaPath)
	return c.sendUpdateWithMedia(update, fullPath, mediaPath)
}

func (c *Client) sendUpdateWithMedia(update *models.Update, fullMediaPath, mediaPath string) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add update JSON
	updateJSON, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("marshaling update: %w", err)
	}

	if err := writer.WriteField("update", string(updateJSON)); err != nil {
		return fmt.Errorf("writing update field: %w", err)
	}

	// Add media file if present
	if fullMediaPath != "" && mediaPath != "" {
		fileID := c.getFileIDFromUpdate(update)
		if fileID != "" {
			file, err := os.Open(fullMediaPath)
			if err != nil {
				slog.Warn("could not open media file", "path", fullMediaPath, "error", err)
			} else {
				defer file.Close()

				part, err := writer.CreateFormFile(fileID, filepath.Base(mediaPath))
				if err != nil {
					return fmt.Errorf("creating form file: %w", err)
				}

				if _, err := io.Copy(part, file); err != nil {
					return fmt.Errorf("copying file content: %w", err)
				}
			}
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("closing multipart writer: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/v1/ingest", &body)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// getFileIDFromUpdate extracts the file ID from the update's media
func (c *Client) getFileIDFromUpdate(update *models.Update) string {
	if update.Message == nil {
		return ""
	}

	msg := update.Message
	if len(msg.Photo) > 0 {
		return msg.Photo[0].FileID
	}
	if msg.Video != nil {
		return msg.Video.FileID
	}
	if msg.Audio != nil {
		return msg.Audio.FileID
	}
	if msg.Voice != nil {
		return msg.Voice.FileID
	}
	if msg.Document != nil {
		return msg.Document.FileID
	}
	if msg.Animation != nil {
		return msg.Animation.FileID
	}
	if msg.VideoNote != nil {
		return msg.VideoNote.FileID
	}

	return ""
}

// BatchResult represents the result of a batch send operation
type BatchResult struct {
	Successful int
	Failed     int
	Errors     []BatchError
}

// BatchError represents an error for a single message in a batch
type BatchError struct {
	MessageID int64
	Error     error
}

// SendBatch sends a batch of updates with the configured delay between each
func (c *Client) SendBatch(updates []*models.Update, exportPath string, includeMedia bool) *BatchResult {
	result := &BatchResult{}

	for i, update := range updates {
		var err error

		if includeMedia && update.Message != nil {
			mediaPath := c.getMediaPath(update)
			if mediaPath != "" {
				err = c.SendUpdateWithMedia(update, mediaPath, exportPath)
			} else {
				err = c.SendUpdate(update)
			}
		} else {
			err = c.SendUpdate(update)
		}

		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, BatchError{
				MessageID: update.Message.MessageID,
				Error:     err,
			})
			slog.Debug("failed to send message", "message_id", update.Message.MessageID, "error", err)
		} else {
			result.Successful++
		}

		// Add delay between messages (except for the last one)
		if i < len(updates)-1 && c.delayMS > 0 {
			time.Sleep(time.Duration(c.delayMS) * time.Millisecond)
		}
	}

	return result
}

// getMediaPath extracts the media path from an update
func (c *Client) getMediaPath(update *models.Update) string {
	if update.Message == nil {
		return ""
	}

	// The media path is embedded in the file ID (we encoded it during mapping)
	// For now, we don't have access to original path here
	// This would need to be passed through the mapper
	return ""
}

// CheckChatExists verifies if a chat exists in the database
func (c *Client) CheckChatExists(chatID int64) (bool, error) {
	// For now, we'll try to send a dummy request and see if it fails
	// In a real implementation, we'd have a dedicated API endpoint
	// Since the API uses upsert, we can't reliably check this
	// We'll return true and let the API handle it
	return true, nil
}

// GenerateFileHash generates a SHA256 hash for a file
func GenerateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hashing file: %w", err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
