package handlers

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"

	"beef-briefing/apps/telegram-bot/internal"
	"beef-briefing/apps/telegram-bot/internal/client"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type UpdateHandler struct {
	apiClient  *client.APIClient
	httpClient *http.Client
}

func NewUpdateHandler(apiClient *client.APIClient) *UpdateHandler {
	return &UpdateHandler{
		apiClient: apiClient,
		httpClient: &http.Client{
			Timeout: internal.FileDownloadTimeout,
		},
	}
}

// Handle processes all incoming updates
func (h *UpdateHandler) Handle(ctx context.Context, b *bot.Bot, update *models.Update) {
	slog.Info("received update", "update_id", update.ID)

	// Log reaction updates for debugging
	if update.MessageReaction != nil {
		slog.Info("received message reaction",
			"update_id", update.ID,
			"chat_id", update.MessageReaction.Chat.ID,
			"message_id", update.MessageReaction.MessageID,
			"user_id", update.MessageReaction.User.ID,
			"new_reaction_count", len(update.MessageReaction.NewReaction),
			"old_reaction_count", len(update.MessageReaction.OldReaction),
		)
	}
	if update.MessageReactionCount != nil {
		slog.Info("received message reaction count",
			"update_id", update.ID,
			"chat_id", update.MessageReactionCount.Chat.ID,
			"message_id", update.MessageReactionCount.MessageID,
			"reaction_count", len(update.MessageReactionCount.Reactions),
		)
	}

	// Extract file IDs from the update
	fileIDs := h.extractFileIDs(update)

	// Download all media files concurrently
	files := h.downloadFiles(ctx, b, fileIDs)

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

		// Photo - get largest size only (last in array)
		if len(msg.Photo) > 0 {
			largest := msg.Photo[len(msg.Photo)-1]
			fileIDs = append(fileIDs, largest.FileID)
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

		// Sticker
		if msg.Sticker != nil {
			fileIDs = append(fileIDs, msg.Sticker.FileID)
		}

		// Game photos - get largest size only
		if msg.Game != nil && len(msg.Game.Photo) > 0 {
			largest := msg.Game.Photo[len(msg.Game.Photo)-1]
			fileIDs = append(fileIDs, largest.FileID)
		}
	}

	// Process message or edited message
	processMessage(update.Message)
	processMessage(update.EditedMessage)

	return fileIDs
}

// downloadFiles downloads multiple files concurrently with a semaphore limit
func (h *UpdateHandler) downloadFiles(ctx context.Context, b *bot.Bot, fileIDs []string) map[string][]byte {
	if len(fileIDs) == 0 {
		return nil
	}

	slog.Info("downloading media files", "count", len(fileIDs))

	files := make(map[string][]byte)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Semaphore to limit concurrent downloads
	sem := make(chan struct{}, internal.MaxConcurrentDownloads)

	for _, fileID := range fileIDs {
		wg.Add(1)
		go func(fid string) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				slog.Warn("download cancelled", "file_id", fid, "error", ctx.Err())
				return
			}

			downloadCtx, cancel := context.WithTimeout(ctx, internal.FileDownloadTimeout)
			fileData, err := h.downloadFile(downloadCtx, b, fid)
			cancel()

			if err != nil {
				slog.Error("failed to download file", "file_id", fid, "error", err)
				return
			}

			mu.Lock()
			files[fid] = fileData
			mu.Unlock()

			slog.Debug("downloaded file", "file_id", fid, "size", len(fileData))
		}(fileID)
	}

	wg.Wait()
	return files
}

// downloadFile downloads a file from Telegram
func (h *UpdateHandler) downloadFile(ctx context.Context, b *bot.Bot, fileID string) ([]byte, error) {
	file, err := b.GetFile(ctx, &bot.GetFileParams{
		FileID: fileID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	fileURL := b.FileDownloadLink(file)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	limitedReader := io.LimitReader(resp.Body, internal.MaxFileSize)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read file data: %w", err)
	}

	if int64(len(data)) >= internal.MaxFileSize {
		return nil, fmt.Errorf("file exceeds maximum size limit of %d bytes", internal.MaxFileSize)
	}

	return data, nil
}
