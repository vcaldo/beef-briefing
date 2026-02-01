package handlers

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync"

	"beef-briefing/apps/telegram-bot/internal"
	"beef-briefing/apps/telegram-bot/internal/client"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/newrelic/go-agent/v3/newrelic"
)

type UpdateHandler struct {
	apiClient          *client.APIClient
	httpClient         *http.Client
	nrApp              *newrelic.Application
	mediaUploadEnabled bool
}

func NewUpdateHandler(apiClient *client.APIClient, nrApp *newrelic.Application, mediaUploadEnabled bool) *UpdateHandler {
	return &UpdateHandler{
		apiClient: apiClient,
		httpClient: &http.Client{
			Timeout: internal.FileDownloadTimeout,
		},
		nrApp:              nrApp,
		mediaUploadEnabled: mediaUploadEnabled,
	}
}

// Handle processes all incoming updates
func (h *UpdateHandler) Handle(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Start New Relic transaction for this update
	txn := h.startTransaction("telegram:handle-update")
	if txn != nil {
		defer txn.End()
		ctx = newrelic.NewContext(ctx, txn)

		// Add custom attributes
		txn.AddAttribute("update_id", update.ID)
		txn.AddAttribute("update_type", h.getUpdateType(update))
	}

	slog.Info("received update", "update_id", update.ID)

	// Log reaction updates for debugging and add attributes
	if update.MessageReaction != nil {
		if txn != nil {
			txn.AddAttribute("chat_id", update.MessageReaction.Chat.ID)
			txn.AddAttribute("message_id", update.MessageReaction.MessageID)
			txn.AddAttribute("user_id", update.MessageReaction.User.ID)
		}
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
		if txn != nil {
			txn.AddAttribute("chat_id", update.MessageReactionCount.Chat.ID)
			txn.AddAttribute("message_id", update.MessageReactionCount.MessageID)
		}
		slog.Info("received message reaction count",
			"update_id", update.ID,
			"chat_id", update.MessageReactionCount.Chat.ID,
			"message_id", update.MessageReactionCount.MessageID,
			"reaction_count", len(update.MessageReactionCount.Reactions),
		)
	}

	// Add message attributes
	if update.Message != nil {
		if txn != nil {
			txn.AddAttribute("chat_id", update.Message.Chat.ID)
			txn.AddAttribute("message_id", update.Message.ID)
			if update.Message.From != nil {
				txn.AddAttribute("user_id", update.Message.From.ID)
			}
		}
	} else if update.EditedMessage != nil {
		if txn != nil {
			txn.AddAttribute("chat_id", update.EditedMessage.Chat.ID)
			txn.AddAttribute("message_id", update.EditedMessage.ID)
			if update.EditedMessage.From != nil {
				txn.AddAttribute("user_id", update.EditedMessage.From.ID)
			}
		}
	}

	// Extract file IDs and download media files (if enabled)
	var files map[string][]byte
	if h.mediaUploadEnabled {
		var fileIDs []string
		if txn != nil {
			segment := txn.StartSegment("extract-file-ids")
			fileIDs = h.extractFileIDs(update)
			segment.End()
			txn.AddAttribute("file_count", len(fileIDs))
		} else {
			fileIDs = h.extractFileIDs(update)
		}

		// Download all media files concurrently
		files = h.downloadFiles(ctx, b, fileIDs)
		if txn != nil {
			txn.AddAttribute("files_downloaded", len(files))
		}
	} else {
		if txn != nil {
			txn.AddAttribute("media_upload_enabled", false)
		}
		slog.Debug("media upload disabled, skipping file downloads", "update_id", update.ID)
	}

	// Send update to API service
	if err := h.apiClient.SendUpdate(ctx, update, files); err != nil {
		if txn != nil {
			txn.NoticeError(err)
		}
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

// startTransaction starts a New Relic transaction if nrApp is configured
func (h *UpdateHandler) startTransaction(name string) *newrelic.Transaction {
	if h.nrApp == nil {
		return nil
	}
	return h.nrApp.StartTransaction(name)
}

// getUpdateType returns the type of update for transaction attributes
func (h *UpdateHandler) getUpdateType(update *models.Update) string {
	switch {
	case update.Message != nil:
		return "message"
	case update.EditedMessage != nil:
		return "edited_message"
	case update.MessageReaction != nil:
		return "message_reaction"
	case update.MessageReactionCount != nil:
		return "message_reaction_count"
	case update.ChannelPost != nil:
		return "channel_post"
	case update.EditedChannelPost != nil:
		return "edited_channel_post"
	case update.CallbackQuery != nil:
		return "callback_query"
	case update.InlineQuery != nil:
		return "inline_query"
	case update.Poll != nil:
		return "poll"
	case update.PollAnswer != nil:
		return "poll_answer"
	default:
		return "unknown"
	}
}

// extractFileIDs extracts all file IDs from an update
func (h *UpdateHandler) extractFileIDs(update *models.Update) []string {
	var fileIDs []string
	seenFileIDs := make(map[string]bool)

	// Helper function to add file ID only if not already seen
	addFileID := func(fileID string) {
		if fileID != "" && !seenFileIDs[fileID] {
			fileIDs = append(fileIDs, fileID)
			seenFileIDs[fileID] = true
		}
	}

	// Helper function to process a message
	processMessage := func(msg *models.Message) {
		if msg == nil {
			return
		}

		// Photo - get largest size only (last in array)
		if len(msg.Photo) > 0 {
			largest := msg.Photo[len(msg.Photo)-1]
			addFileID(largest.FileID)
		}

		// Video
		if msg.Video != nil {
			addFileID(msg.Video.FileID)
		}

		// Audio
		if msg.Audio != nil {
			addFileID(msg.Audio.FileID)
		}

		// Voice
		if msg.Voice != nil {
			addFileID(msg.Voice.FileID)
		}

		// Animation - process before Document (has richer metadata for GIFs)
		if msg.Animation != nil {
			addFileID(msg.Animation.FileID)
		}

		// Document - may have same file_id as Animation for GIFs, deduplication handles it
		if msg.Document != nil {
			addFileID(msg.Document.FileID)
		}

		// VideoNote
		if msg.VideoNote != nil {
			addFileID(msg.VideoNote.FileID)
		}

		// Sticker
		if msg.Sticker != nil {
			addFileID(msg.Sticker.FileID)
		}

		// Game photos - get largest size only
		if msg.Game != nil && len(msg.Game.Photo) > 0 {
			largest := msg.Game.Photo[len(msg.Game.Photo)-1]
			addFileID(largest.FileID)
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

	// Get transaction from context for segment tracking
	txn := newrelic.FromContext(ctx)

	// Start segment for the overall download operation
	var downloadAllSegment *newrelic.Segment
	if txn != nil {
		downloadAllSegment = txn.StartSegment("download-files")
		defer downloadAllSegment.End()
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
				if txn != nil {
					txn.NoticeError(fmt.Errorf("download cancelled for file %s: %w", fid, ctx.Err()))
				}
				slog.Warn("download cancelled", "file_id", fid, "error", ctx.Err())
				return
			}

			downloadCtx, cancel := context.WithTimeout(ctx, internal.FileDownloadTimeout)
			fileData, err := h.downloadFile(downloadCtx, b, fid)
			cancel()

			if err != nil {
				if txn != nil {
					txn.NoticeError(fmt.Errorf("failed to download file %s: %w", fid, err))
				}
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
	txn := newrelic.FromContext(ctx)

	// Segment for getting file info from Telegram API
	var getFileSegment *newrelic.Segment
	if txn != nil {
		getFileSegment = txn.StartSegment("telegram-api:get-file")
	}

	file, err := b.GetFile(ctx, &bot.GetFileParams{
		FileID: fileID,
	})

	if getFileSegment != nil {
		getFileSegment.End()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	fileURL := b.FileDownloadLink(file)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	// External segment for downloading from Telegram CDN
	var externalSegment *newrelic.ExternalSegment
	if txn != nil {
		parsedURL, _ := url.Parse(fileURL)
		host := "api.telegram.org"
		if parsedURL != nil {
			host = parsedURL.Host
		}
		externalSegment = &newrelic.ExternalSegment{
			StartTime: txn.StartSegmentNow(),
			URL:       fileURL,
			Host:      host,
			Procedure: "GET",
			Library:   "net/http",
		}
	}

	resp, err := h.httpClient.Do(req)

	if externalSegment != nil {
		if resp != nil {
			externalSegment.Response = resp
		}
		externalSegment.End()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Segment for reading file data
	var readSegment *newrelic.Segment
	if txn != nil {
		readSegment = txn.StartSegment("read-file-data")
	}

	limitedReader := io.LimitReader(resp.Body, internal.MaxFileSize)
	data, err := io.ReadAll(limitedReader)

	if readSegment != nil {
		readSegment.End()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read file data: %w", err)
	}

	if int64(len(data)) >= internal.MaxFileSize {
		return nil, fmt.Errorf("file exceeds maximum size limit of %d bytes", internal.MaxFileSize)
	}

	return data, nil
}
