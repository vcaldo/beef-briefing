package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"beef-briefing/apps/telegram-bot/internal/storage"
	"beef-briefing/apps/telegram-bot/internal/store"
)

// Importer orchestrates the import process
type Importer struct {
	store       *store.PostgresStore
	minioClient *storage.MinIOClient
	chunkSize   int
	activeLocks sync.Map
}

// NewImporter creates a new importer instance
func NewImporter(store *store.PostgresStore, minioClient *storage.MinIOClient, chunkSize int) *Importer {
	return &Importer{
		store:       store,
		minioClient: minioClient,
		chunkSize:   chunkSize,
		activeLocks: sync.Map{},
	}
}

// Import imports messages from extracted Telegram export directory
func (im *Importer) Import(ctx context.Context, chatID int64, extractedDir string, progressChan chan<- ImportProgress) error {
	// Acquire lock for this chat
	if _, loaded := im.activeLocks.LoadOrStore(chatID, true); loaded {
		return fmt.Errorf("import already in progress for chat %d", chatID)
	}
	defer im.activeLocks.Delete(chatID)

	// Parse result.json
	resultPath := filepath.Join(extractedDir, "result.json")
	data, err := os.ReadFile(resultPath)
	if err != nil {
		return fmt.Errorf("failed to read result.json: %w", err)
	}

	var exportData ExportData
	if err := json.Unmarshal(data, &exportData); err != nil {
		return fmt.Errorf("failed to parse result.json: %w", err)
	}

	totalMessages := len(exportData.Messages)
	totalChunks := (totalMessages + im.chunkSize - 1) / im.chunkSize

	slog.Info("starting import", "chat_id", chatID, "total_messages", totalMessages, "chunks", totalChunks)

	// Create media processor
	mediaProc := NewMediaProcessor(im.minioClient, extractedDir)

	// Initialize progress
	progress := ImportProgress{
		Total:       totalMessages,
		TotalChunks: totalChunks,
	}

	// Process messages in chunks
	for chunkIdx := 0; chunkIdx < totalChunks; chunkIdx++ {
		start := chunkIdx * im.chunkSize
		end := start + im.chunkSize
		if end > totalMessages {
			end = totalMessages
		}

		progress.CurrentChunk = chunkIdx + 1

		// Process chunk with transaction
		if err := im.processChunk(ctx, &exportData, start, end, chatID, mediaProc, &progress); err != nil {
			slog.Error("chunk processing failed", "chunk", chunkIdx+1, "error", err)
			progress.ErrorCount++
		}

		// Send progress update
		select {
		case progressChan <- progress:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	slog.Info("import completed", "chat_id", chatID, "processed", progress.Processed, "inserted", progress.Inserted, "errors", progress.ErrorCount)
	return nil
}

// processChunk processes a chunk of messages within a transaction
func (im *Importer) processChunk(ctx context.Context, exportData *ExportData, start, end int, chatID int64, mediaProc *MediaProcessor, progress *ImportProgress) error {
	// Begin transaction
	tx, err := im.store.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Process messages in this chunk
	for i := start; i < end; i++ {
		msg := &exportData.Messages[i]
		progress.Processed++

		if err := im.processMessage(ctx, msg, chatID, exportData.Type, exportData.Name, mediaProc, progress); err != nil {
			slog.Error("failed to process message", "message_id", msg.ID, "error", err)
			progress.ErrorCount++
			// Continue processing despite errors
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// processMessage processes a single message
func (im *Importer) processMessage(ctx context.Context, exportMsg *ExportMessage, chatID int64, chatType, chatName string, mediaProc *MediaProcessor, progress *ImportProgress) error {
	// Handle service messages
	if exportMsg.Type == "service" {
		return im.processServiceMessage(ctx, exportMsg, chatID, chatType, chatName, progress)
	}

	// Handle regular messages
	return im.processRegularMessage(ctx, exportMsg, chatID, chatType, chatName, mediaProc, progress)
}

// processServiceMessage processes a service message
func (im *Importer) processServiceMessage(ctx context.Context, exportMsg *ExportMessage, chatID int64, chatType, chatName string, progress *ImportProgress) error {
	// Check if already exists
	exists, err := im.store.ServiceMessageExists(ctx, chatID, int64(exportMsg.ID))
	if err != nil {
		return fmt.Errorf("failed to check service message existence: %w", err)
	}
	if exists {
		progress.Skipped++
		return nil
	}

	// Upsert chat
	chat := &store.Chat{
		ID:        chatID,
		Type:      chatType,
		Name:      chatName,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := im.store.UpsertChat(ctx, chat); err != nil {
		return fmt.Errorf("failed to upsert chat: %w", err)
	}

	// Parse actor user ID
	var actorUserID *int64
	if exportMsg.ActorID != nil {
		id, err := ParseUserID(*exportMsg.ActorID)
		if err == nil {
			actorUserID = &id

			// Upsert actor user
			actorUser := &store.User{
				ID:        id,
				FirstName: *exportMsg.Actor,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			if err := im.store.UpsertUser(ctx, actorUser); err != nil {
				slog.Warn("failed to upsert actor user", "user_id", id, "error", err)
			}
		}
	}

	// Parse message date
	messageDate, err := parseMessageDate(exportMsg)
	if err != nil {
		return fmt.Errorf("failed to parse message date: %w", err)
	}

	// Build metadata
	metadata := make(map[string]interface{})
	if exportMsg.Members != nil && len(exportMsg.Members) > 0 {
		metadata["members"] = exportMsg.Members
	}
	metadataJSON, _ := json.Marshal(metadata)

	// Map action
	action := ""
	if exportMsg.Action != nil {
		action = MapServiceAction(*exportMsg.Action)
	}

	// Insert service message
	serviceMsg := &store.ServiceMessage{
		TelegramMessageID: int64(exportMsg.ID),
		ChatID:            chatID,
		ActorUserID:       actorUserID,
		MessageDate:       messageDate,
		Action:            action,
		Metadata:          metadataJSON,
	}

	if err := im.store.InsertServiceMessage(ctx, serviceMsg); err != nil {
		return fmt.Errorf("failed to insert service message: %w", err)
	}

	progress.Inserted++
	return nil
}

// processRegularMessage processes a regular message
func (im *Importer) processRegularMessage(ctx context.Context, exportMsg *ExportMessage, chatID int64, chatType, chatName string, mediaProc *MediaProcessor, progress *ImportProgress) error {
	// Check if already exists
	exists, err := im.store.MessageExists(ctx, chatID, int64(exportMsg.ID))
	if err != nil {
		return fmt.Errorf("failed to check message existence: %w", err)
	}
	if exists {
		progress.Skipped++
		return nil
	}

	// Upsert chat
	chat := &store.Chat{
		ID:        chatID,
		Type:      chatType,
		Name:      chatName,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := im.store.UpsertChat(ctx, chat); err != nil {
		return fmt.Errorf("failed to upsert chat: %w", err)
	}

	// Parse user ID
	var userID *int64
	if exportMsg.FromID != "" {
		id, err := ParseUserID(exportMsg.FromID)
		if err == nil {
			userID = &id

			// Upsert user
			user := &store.User{
				ID:        id,
				FirstName: stringValue(exportMsg.From),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			if err := im.store.UpsertUser(ctx, user); err != nil {
				slog.Warn("failed to upsert user", "user_id", id, "error", err)
			}
		}
	}

	// Parse message date
	messageDate, err := parseMessageDate(exportMsg)
	if err != nil {
		return fmt.Errorf("failed to parse message date: %w", err)
	}

	// Determine message type
	messageType := DetermineMediaType(exportMsg)

	// Process media if present
	var mediaSHA256 *string
	var mediaFileName *string
	var mediaMimeType *string
	if mediaPath := GetMediaPath(exportMsg); mediaPath != "" {
		mimeType := stringValue(exportMsg.MimeType)
		hash, err := mediaProc.ProcessMedia(ctx, mediaPath, mimeType)
		if err != nil {
			slog.Warn("failed to process media, continuing without it", "path", mediaPath, "error", err)
		} else {
			mediaSHA256 = &hash
			progress.MediaUploaded++
		}

		mediaFileName = exportMsg.FileName
		mediaMimeType = exportMsg.MimeType
	}

	// Parse text content
	textContent := exportMsg.GetTextContent()
	var text *string
	if textContent != "" {
		text = &textContent
	}

	// Marshal text entities
	var entities json.RawMessage
	if len(exportMsg.TextEntities) > 0 {
		entities, _ = json.Marshal(exportMsg.TextEntities)
	}

	// Parse forwarded user ID
	var forwardedFromUserID *int64
	if exportMsg.ForwardedFromID != nil {
		id, err := ParseUserID(*exportMsg.ForwardedFromID)
		if err == nil {
			forwardedFromUserID = &id
		}
	}

	// Parse reply to message ID
	var replyToMessageID *int64
	if exportMsg.ReplyToMsgID != nil {
		id := int64(*exportMsg.ReplyToMsgID)
		replyToMessageID = &id
	}

	// Convert file size to int64
	var fileSize *int64
	if exportMsg.FileSize != nil {
		size := int64(*exportMsg.FileSize)
		fileSize = &size
	} else if exportMsg.PhotoFileSize != nil {
		size := int64(*exportMsg.PhotoFileSize)
		fileSize = &size
	}

	// Insert message
	message := &store.Message{
		TelegramMessageID:   int64(exportMsg.ID),
		ChatID:              chatID,
		UserID:              userID,
		MessageDate:         messageDate,
		MessageType:         messageType,
		Text:                text,
		ReplyToMessageID:    replyToMessageID,
		ForwardedFromUserID: forwardedFromUserID,
		MediaSHA256:         mediaSHA256,
		MediaFileName:       mediaFileName,
		MediaFileSize:       fileSize,
		MediaMimeType:       mediaMimeType,
		MediaDuration:       exportMsg.DurationSec,
		MediaWidth:          exportMsg.Width,
		MediaHeight:         exportMsg.Height,
		Entities:            entities,
	}

	if _, err := im.store.InsertMessage(ctx, message); err != nil {
		return fmt.Errorf("failed to insert message: %w", err)
	}

	progress.Inserted++
	return nil
}

// parseMessageDate parses the message date from export format
func parseMessageDate(exportMsg *ExportMessage) (time.Time, error) {
	// Try Unix timestamp first
	if exportMsg.DateUnixtime != "" {
		timestamp, err := strconv.ParseInt(exportMsg.DateUnixtime, 10, 64)
		if err == nil {
			return time.Unix(timestamp, 0).UTC(), nil
		}
	}

	// Fall back to ISO 8601 format
	if exportMsg.Date != "" {
		t, err := time.Parse(time.RFC3339, exportMsg.Date)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to parse date %q: %w", exportMsg.Date, err)
		}
		return t.UTC(), nil
	}

	return time.Time{}, fmt.Errorf("no date found in message")
}

// stringValue safely dereferences a string pointer
func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
