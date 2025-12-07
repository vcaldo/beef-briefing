package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"beef-briefing/apps/telegram-bot/internal/client"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type UpdateHandler struct {
	bot       *bot.Bot
	apiClient *client.APIClient
}

func NewUpdateHandler(b *bot.Bot, apiClient *client.APIClient) *UpdateHandler {
	return &UpdateHandler{
		bot:       b,
		apiClient: apiClient,
	}
}

// Handle processes all incoming updates
func (h *UpdateHandler) Handle(ctx context.Context, b *bot.Bot, update *models.Update) {
	slog.Info("received update", "update_id", update.ID)

	// Extract file IDs from the update
	fileIDs := h.extractFileIDs(update)

	// Download all media files
	files := make(map[string][]byte)
	if len(fileIDs) > 0 {
		slog.Info("downloading media files", "count", len(fileIDs))

		for _, fileID := range fileIDs {
			// Create context with 2-minute timeout for file download
			downloadCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()

			fileData, err := h.downloadFile(downloadCtx, fileID)
			if err != nil {
				slog.Error("failed to download file",
					"file_id", fileID,
					"error", err,
				)
				// Continue processing other files
				continue
			}

			files[fileID] = fileData
			slog.Debug("downloaded file",
				"file_id", fileID,
				"size", len(fileData),
			)
		}
	}

	// Send update to API service
	if err := h.apiClient.SendUpdate(ctx, update, files); err != nil {
		slog.Error("failed to send update to API",
			"update_id", update.ID,
			"error", err,
		)
		return
	}

	// Log successful processing
	logFields := []interface{}{"update_id", update.ID}

	if update.Message != nil {
		logFields = append(logFields, "message_id", update.Message.ID)
		logFields = append(logFields, "chat_id", update.Message.Chat.ID)
	} else if update.EditedMessage != nil {
		logFields = append(logFields, "message_id", update.EditedMessage.ID)
		logFields = append(logFields, "chat_id", update.EditedMessage.Chat.ID)
		logFields = append(logFields, "type", "edit")
	} else if update.MessageReaction != nil {
		logFields = append(logFields, "message_id", update.MessageReaction.MessageID)
		logFields = append(logFields, "chat_id", update.MessageReaction.Chat.ID)
		logFields = append(logFields, "type", "reaction")
	} else if update.MessageReactionCount != nil {
		logFields = append(logFields, "message_id", update.MessageReactionCount.MessageID)
		logFields = append(logFields, "chat_id", update.MessageReactionCount.Chat.ID)
		logFields = append(logFields, "type", "reaction_count")
	}

	slog.Info("successfully processed update", logFields...)
}

// extractFileIDs extracts all file IDs from an update
func (h *UpdateHandler) extractFileIDs(update *models.Update) []string {
	var fileIDs []string

	// Helper function to process a message
	processMessage := func(msg *models.Message) {
		if msg == nil {
			return
		}

		// Photo (array of sizes)
		for _, photo := range msg.Photo {
			fileIDs = append(fileIDs, photo.FileID)
		}

		// Video
		if msg.Video != nil {
			fileIDs = append(fileIDs, msg.Video.FileID)
		}

		// Audio
		if msg.Audio != nil {
			fileIDs = append(fileIDs, msg.Audio.FileID)
		}

		// Voice
		if msg.Voice != nil {
			fileIDs = append(fileIDs, msg.Voice.FileID)
		}

		// Document
		if msg.Document != nil {
			fileIDs = append(fileIDs, msg.Document.FileID)
		}

		// Animation
		if msg.Animation != nil {
			fileIDs = append(fileIDs, msg.Animation.FileID)
		}

		// VideoNote
		if msg.VideoNote != nil {
			fileIDs = append(fileIDs, msg.VideoNote.FileID)
		}
	}

	// Process message or edited message
	processMessage(update.Message)
	processMessage(update.EditedMessage)

	return fileIDs
}

// downloadFile downloads a file from Telegram
func (h *UpdateHandler) downloadFile(ctx context.Context, fileID string) ([]byte, error) {
	// Get file info from Telegram
	file, err := h.bot.GetFile(ctx, &bot.GetFileParams{
		FileID: fileID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Build file URL
	fileURL := h.bot.FileDownloadLink(file)

	// Download file content
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	httpClient := &http.Client{
		Timeout: 2 * time.Minute,
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read file data
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read file data: %w", err)
	}

	return data, nil
}

// TelegramUpdate wraps the bot's Update to implement our Update interface
type TelegramUpdate struct {
	*models.Update
}

func (t *TelegramUpdate) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Update)
}

func (t *TelegramUpdate) GetUpdateID() int64 {
	return t.ID
}
