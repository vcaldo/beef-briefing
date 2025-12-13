// Package services contains the business logic layer for the api-service.
//
// # Error Handling Strategy
//
// The IngestService implements a two-tier error handling approach:
//
// Hard Failures (transaction rollback):
//   - Update record insertion
//   - Message insertion
//   - Chat upsert
//   - User upsert
//
// These operations are critical for data integrity. If they fail, the entire
// transaction is rolled back to prevent partial/inconsistent state.
//
// Soft Failures (log and continue):
//   - Message entities (text formatting metadata)
//   - Location/Venue data
//   - Contact information
//   - Poll options
//   - Dice results
//   - Individual media files
//
// These operations are supplementary. Failing to insert an entity or media file
// should not prevent the core message from being saved. This ensures we capture
// as much data as possible even when some components fail (e.g., file download
// issues, temporary storage problems).
package services

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"beef-briefing/apps/api-service/internal/models"
	"beef-briefing/apps/api-service/internal/repository"
	"beef-briefing/apps/api-service/internal/storage"

	"github.com/newrelic/go-agent/v3/newrelic"
)

// IngestService handles the business logic for processing Telegram updates.
type IngestService struct {
	db            *sql.DB
	storageClient *storage.MinIOClient
	nrApp         *newrelic.Application
	updateRepo    *repository.UpdateRepository
	chatRepo      *repository.ChatRepository
	userRepo      *repository.UserRepository
	messageRepo   *repository.MessageRepository
	reactionRepo  *repository.ReactionRepository
	mediaRepo     *repository.MediaRepository
}

// NewIngestService creates a new IngestService with all required dependencies.
func NewIngestService(db *sql.DB, storageClient *storage.MinIOClient, nrApp *newrelic.Application) *IngestService {
	return &IngestService{
		db:            db,
		storageClient: storageClient,
		nrApp:         nrApp,
		updateRepo:    repository.NewUpdateRepository(db, nrApp),
		chatRepo:      repository.NewChatRepository(db, nrApp),
		userRepo:      repository.NewUserRepository(db, nrApp),
		messageRepo:   repository.NewMessageRepository(db, nrApp),
		reactionRepo:  repository.NewReactionRepository(db, nrApp),
		mediaRepo:     repository.NewMediaRepository(db, nrApp),
	}
}

// ProcessUpdate handles a Telegram update within a database transaction.
func (s *IngestService) ProcessUpdate(ctx context.Context, update *models.Update, files map[string][]byte) error {
	txn := newrelic.FromContext(ctx)

	// Start segment for transaction management
	if txn != nil {
		segment := txn.StartSegment("service:process-update")
		defer segment.End()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	updateType := s.getUpdateType(update)
	if err := s.updateRepo.InsertUpdate(ctx, tx, update.UpdateID, updateType, update); err != nil {
		return fmt.Errorf("failed to insert update: %w", err)
	}

	switch {
	case update.Message != nil:
		if err := s.processMessage(ctx, tx, update.Message, files, false); err != nil {
			return fmt.Errorf("failed to process message: %w", err)
		}

	case update.EditedMessage != nil:
		if err := s.processMessage(ctx, tx, update.EditedMessage, files, true); err != nil {
			return fmt.Errorf("failed to process edited message: %w", err)
		}

	case update.MessageReaction != nil:
		if err := s.processMessageReaction(ctx, tx, update.MessageReaction); err != nil {
			return fmt.Errorf("failed to process message reaction: %w", err)
		}

	case update.MessageReactionCount != nil:
		if err := s.processReactionCount(ctx, tx, update.MessageReactionCount); err != nil {
			return fmt.Errorf("failed to process reaction count: %w", err)
		}

	case update.MyChatMember != nil:
		if err := s.processMyChatMember(ctx, tx, update.MyChatMember); err != nil {
			return fmt.Errorf("failed to process my_chat_member: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *IngestService) processMessage(ctx context.Context, tx *sql.Tx, msg *models.Message, files map[string][]byte, isEdit bool) error {
	// Hard failure: chat must be upserted
	if err := s.chatRepo.UpsertChat(ctx, tx, &msg.Chat); err != nil {
		return fmt.Errorf("failed to upsert chat: %w", err)
	}

	// Hard failure: user must be upserted if present
	if msg.From != nil {
		if err := s.userRepo.UpsertUser(ctx, tx, msg.From); err != nil {
			return fmt.Errorf("failed to upsert user: %w", err)
		}
	}

	// Soft failure: edit tracking
	if isEdit && msg.EditDate != nil {
		prevMsg, err := s.messageRepo.GetMessageByID(ctx, tx, msg.Chat.ID, msg.MessageID)
		if err == nil {
			editDate := time.Unix(*msg.EditDate, 0)
			if err := s.messageRepo.InsertMessageEdit(ctx, tx, prevMsg.ID, editDate,
				prevMsg.Text.String, msg.Text,
				prevMsg.Caption.String, msg.Caption); err != nil {
				slog.Warn("failed to insert message edit", "error", err)
			}
		}
	}

	// Hard failure: message must be inserted
	messageID, err := s.messageRepo.InsertMessage(ctx, tx, msg)
	if err != nil {
		return fmt.Errorf("failed to insert message: %w", err)
	}

	// Soft failures: entities
	for _, entity := range msg.Entities {
		if err := s.messageRepo.InsertEntity(ctx, tx, messageID, &entity); err != nil {
			slog.Warn("failed to insert entity", "error", err)
		}
	}

	for _, entity := range msg.CaptionEntities {
		if err := s.messageRepo.InsertEntity(ctx, tx, messageID, &entity); err != nil {
			slog.Warn("failed to insert caption entity", "error", err)
		}
	}

	// Soft failure: media processing
	if err := s.processMedia(ctx, tx, messageID, msg, files); err != nil {
		return fmt.Errorf("failed to process media: %w", err)
	}

	// Soft failures: location, venue, contact, poll, dice
	if msg.Location != nil && msg.Venue == nil {
		if err := s.mediaRepo.InsertLocation(ctx, tx, messageID, msg.Location); err != nil {
			slog.Warn("failed to insert location", "error", err)
		}
	}

	if msg.Venue != nil {
		locationID, err := s.mediaRepo.InsertLocationReturningID(ctx, tx, messageID, &msg.Venue.Location)
		if err != nil {
			slog.Warn("failed to insert venue location", "error", err)
		} else {
			if err := s.mediaRepo.InsertVenue(ctx, tx, messageID, locationID, msg.Venue); err != nil {
				slog.Warn("failed to insert venue", "error", err)
			}
		}
	}

	if msg.Contact != nil {
		if err := s.mediaRepo.InsertContact(ctx, tx, messageID, msg.Contact); err != nil {
			slog.Warn("failed to insert contact", "error", err)
		}
	}

	if msg.Poll != nil {
		pollID, err := s.mediaRepo.InsertPoll(ctx, tx, messageID, msg.Poll)
		if err != nil {
			slog.Warn("failed to insert poll", "error", err)
		} else {
			for i, option := range msg.Poll.Options {
				if err := s.mediaRepo.InsertPollOption(ctx, tx, pollID, i, &option); err != nil {
					slog.Warn("failed to insert poll option", "error", err, "index", i)
				}
			}
		}
	}

	if msg.Dice != nil {
		if err := s.mediaRepo.InsertDice(ctx, tx, messageID, msg.Dice); err != nil {
			slog.Warn("failed to insert dice", "error", err)
		}
	}

	return nil
}

func (s *IngestService) processMedia(ctx context.Context, tx *sql.Tx, messageID int64, msg *models.Message, files map[string][]byte) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:process-media")
		defer segment.End()
	}

	// Track processed file IDs to prevent duplicates (e.g., GIFs with both Document and Animation fields)
	processedFileIDs := make(map[string]bool)

	// Process photos
	for _, photo := range msg.Photo {
		if processedFileIDs[photo.FileID] {
			slog.Debug("skipping duplicate photo file_id", "file_id", photo.FileID)
			continue
		}

		data, ok := files[photo.FileID]
		if !ok {
			slog.Warn("photo file not found in request, skipping", "file_id", photo.FileID)
			continue
		}

		objectKey, fileHash, err := s.uploadOrReuseMedia(ctx, tx, photo.FileID, data, "image/jpeg", "photo")
		if err != nil {
			slog.Error("failed to upload photo", "file_id", photo.FileID, "error", err)
			continue
		}

		if err := s.mediaRepo.InsertPhoto(ctx, tx, messageID, &photo, objectKey, fileHash); err != nil {
			slog.Warn("failed to insert photo", "error", err)
		} else {
			processedFileIDs[photo.FileID] = true
		}
	}

	// Process video
	if msg.Video != nil && msg.Video.FileID != "" && !processedFileIDs[msg.Video.FileID] {
		if err := s.processMediaFile(ctx, tx, messageID, "video", msg.Video.FileID, msg.Video.FileUniqueID,
			msg.Video.FileSize, msg.Video.MimeType, msg.Video.FileName,
			&msg.Video.Duration, &msg.Video.Width, &msg.Video.Height, "", "", files); err != nil {
			slog.Error("failed to process video", "error", err)
		} else {
			processedFileIDs[msg.Video.FileID] = true
		}
	}

	// Process audio
	if msg.Audio != nil && msg.Audio.FileID != "" && !processedFileIDs[msg.Audio.FileID] {
		if err := s.processMediaFile(ctx, tx, messageID, "audio", msg.Audio.FileID, msg.Audio.FileUniqueID,
			msg.Audio.FileSize, msg.Audio.MimeType, msg.Audio.FileName,
			&msg.Audio.Duration, nil, nil, msg.Audio.Performer, msg.Audio.Title, files); err != nil {
			slog.Error("failed to process audio", "error", err)
		} else {
			processedFileIDs[msg.Audio.FileID] = true
		}
	}

	// Process voice
	if msg.Voice != nil && msg.Voice.FileID != "" && !processedFileIDs[msg.Voice.FileID] {
		if err := s.processMediaFile(ctx, tx, messageID, "voice", msg.Voice.FileID, msg.Voice.FileUniqueID,
			msg.Voice.FileSize, msg.Voice.MimeType, "",
			&msg.Voice.Duration, nil, nil, "", "", files); err != nil {
			slog.Error("failed to process voice", "error", err)
		} else {
			processedFileIDs[msg.Voice.FileID] = true
		}
	}

	// Process animation BEFORE document (has richer metadata for GIFs: width, height, duration)
	if msg.Animation != nil && msg.Animation.FileID != "" && !processedFileIDs[msg.Animation.FileID] {
		if err := s.processMediaFile(ctx, tx, messageID, "animation", msg.Animation.FileID, msg.Animation.FileUniqueID,
			msg.Animation.FileSize, msg.Animation.MimeType, msg.Animation.FileName,
			&msg.Animation.Duration, &msg.Animation.Width, &msg.Animation.Height, "", "", files); err != nil {
			slog.Error("failed to process animation", "error", err)
		} else {
			processedFileIDs[msg.Animation.FileID] = true
		}
	}

	// Process document AFTER animation (GIFs may have both fields with same file_id)
	if msg.Document != nil && msg.Document.FileID != "" && !processedFileIDs[msg.Document.FileID] {
		if err := s.processMediaFile(ctx, tx, messageID, "document", msg.Document.FileID, msg.Document.FileUniqueID,
			msg.Document.FileSize, msg.Document.MimeType, msg.Document.FileName,
			nil, nil, nil, "", "", files); err != nil {
			slog.Error("failed to process document", "error", err)
		} else {
			processedFileIDs[msg.Document.FileID] = true
		}
	}

	// Process video note
	if msg.VideoNote != nil && msg.VideoNote.FileID != "" && !processedFileIDs[msg.VideoNote.FileID] {
		if err := s.processMediaFile(ctx, tx, messageID, "video_note", msg.VideoNote.FileID, msg.VideoNote.FileUniqueID,
			msg.VideoNote.FileSize, "", "",
			&msg.VideoNote.Duration, &msg.VideoNote.Length, &msg.VideoNote.Length, "", "", files); err != nil {
			slog.Error("failed to process video note", "error", err)
		} else {
			processedFileIDs[msg.VideoNote.FileID] = true
		}
	}

	// Process sticker
	if msg.Sticker != nil {
		if err := s.processSticker(ctx, tx, messageID, msg.Sticker, files); err != nil {
			slog.Error("failed to process sticker", "error", err)
		}
	}

	// Process game
	if msg.Game != nil {
		if err := s.processGame(ctx, tx, messageID, msg.Game, files); err != nil {
			slog.Error("failed to process game", "error", err)
		}
	}

	return nil
}

// uploadOrReuseMedia handles file deduplication by checking hash before upload.
// Returns the object key and file hash for database storage.
func (s *IngestService) uploadOrReuseMedia(ctx context.Context, tx *sql.Tx, fileID string, data []byte, mimeType, mediaType string) (objectKey, fileHash string, err error) {
	fileHash = storage.ComputeFileHash(data)

	existingKey, err := s.mediaRepo.GetObjectKeyByHash(ctx, tx, fileHash)
	if err != nil {
		return "", "", fmt.Errorf("failed to check file hash: %w", err)
	}

	if existingKey != "" {
		slog.Debug("media already exists in storage, reusing",
			"file_id", fileID,
			"media_type", mediaType,
			"object_key", existingKey,
		)
		return existingKey, fileHash, nil
	}

	objectKey, _, err = s.storageClient.UploadMedia(ctx, fileID, data, mimeType, mediaType)
	if err != nil {
		return "", "", fmt.Errorf("failed to upload to MinIO: %w", err)
	}

	return objectKey, fileHash, nil
}

func (s *IngestService) processMediaFile(ctx context.Context, tx *sql.Tx, messageID int64, mediaType, fileID, fileUniqueID string, fileSize *int64, mimeType, fileName string, duration, width, height *int, performer, title string, files map[string][]byte) error {
	var objectKey, fileHash string

	// Try to download and store the file if available
	data, ok := files[fileID]
	if ok {
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}

		var err error
		objectKey, fileHash, err = s.uploadOrReuseMedia(ctx, tx, fileID, data, mimeType, mediaType)
		if err != nil {
			slog.Error("failed to upload media, saving metadata only",
				"file_id", fileID,
				"media_type", mediaType,
				"error", err,
			)
			// Continue to insert metadata even if upload fails
			objectKey = ""
			fileHash = ""
		}
	} else {
		slog.Info("media file not downloaded, saving metadata only",
			"file_id", fileID,
			"media_type", mediaType,
			"file_size", fileSize,
		)
	}

	// Insert metadata (objectKey and fileHash will be empty strings if file wasn't downloaded)
	return s.mediaRepo.InsertMediaFile(ctx, tx, messageID, mediaType, fileID, fileUniqueID, objectKey, fileHash,
		fileSize, mimeType, fileName, duration, width, height, performer, title)
}

func (s *IngestService) processSticker(ctx context.Context, tx *sql.Tx, messageID int64, sticker *models.Sticker, files map[string][]byte) error {
	data, ok := files[sticker.FileID]
	if !ok {
		slog.Warn("sticker file not found in request, skipping", "file_id", sticker.FileID)
		return nil
	}

	mimeType := "image/webp"
	if sticker.IsAnimated {
		mimeType = "application/x-tgsticker"
	} else if sticker.IsVideo {
		mimeType = "video/webm"
	}

	objectKey, fileHash, err := s.uploadOrReuseMedia(ctx, tx, sticker.FileID, data, mimeType, "sticker")
	if err != nil {
		return err
	}

	mediaFileID, err := s.mediaRepo.InsertMediaFileReturningID(ctx, tx, messageID, "sticker",
		sticker.FileID, sticker.FileUniqueID, objectKey, fileHash,
		sticker.FileSize, mimeType, "",
		nil, &sticker.Width, &sticker.Height, "", "")
	if err != nil {
		return fmt.Errorf("failed to insert sticker media file: %w", err)
	}

	if err := s.mediaRepo.InsertSticker(ctx, tx, messageID, mediaFileID, sticker); err != nil {
		return fmt.Errorf("failed to insert sticker metadata: %w", err)
	}

	return nil
}

func (s *IngestService) processGame(ctx context.Context, tx *sql.Tx, messageID int64, game *models.Game, files map[string][]byte) error {
	gameID, err := s.mediaRepo.InsertGame(ctx, tx, messageID, game)
	if err != nil {
		return fmt.Errorf("failed to insert game: %w", err)
	}

	for _, photo := range game.Photo {
		data, ok := files[photo.FileID]
		if !ok {
			slog.Warn("game photo file not found in request, skipping", "file_id", photo.FileID)
			continue
		}

		objectKey, fileHash, err := s.uploadOrReuseMedia(ctx, tx, photo.FileID, data, "image/jpeg", "game_photo")
		if err != nil {
			slog.Error("failed to upload game photo", "file_id", photo.FileID, "error", err)
			continue
		}

		if err := s.mediaRepo.InsertGamePhoto(ctx, tx, gameID, &photo, objectKey, fileHash); err != nil {
			slog.Warn("failed to insert game photo", "error", err)
		}
	}

	return nil
}

func (s *IngestService) processMessageReaction(ctx context.Context, tx *sql.Tx, reaction *models.MessageReactionUpdated) error {
	if err := s.chatRepo.UpsertChat(ctx, tx, &reaction.Chat); err != nil {
		return fmt.Errorf("failed to upsert chat: %w", err)
	}

	if reaction.User != nil {
		if err := s.userRepo.UpsertUser(ctx, tx, reaction.User); err != nil {
			return fmt.Errorf("failed to upsert user: %w", err)
		}
	}

	if reaction.ActorChat != nil {
		if err := s.chatRepo.UpsertChat(ctx, tx, reaction.ActorChat); err != nil {
			return fmt.Errorf("failed to upsert actor chat: %w", err)
		}
	}

	return s.reactionRepo.InsertMessageReaction(ctx, tx, reaction)
}

func (s *IngestService) processReactionCount(ctx context.Context, tx *sql.Tx, countUpdate *models.MessageReactionCountUpdate) error {
	if err := s.chatRepo.UpsertChat(ctx, tx, &countUpdate.Chat); err != nil {
		return fmt.Errorf("failed to upsert chat: %w", err)
	}

	return s.reactionRepo.UpsertReactionCount(ctx, tx, countUpdate)
}

func (s *IngestService) processMyChatMember(ctx context.Context, tx *sql.Tx, member *models.ChatMemberUpdated) error {
	// Upsert the chat first
	if err := s.chatRepo.UpsertChat(ctx, tx, &member.Chat); err != nil {
		return fmt.Errorf("failed to upsert chat: %w", err)
	}

	// Upsert the user who performed the action
	if err := s.userRepo.UpsertUser(ctx, tx, &member.From); err != nil {
		return fmt.Errorf("failed to upsert user: %w", err)
	}

	// Handle group-to-supergroup migration
	if member.Chat.MigrateFromChatID != nil {
		slog.Info("detected group migration",
			"old_chat_id", *member.Chat.MigrateFromChatID,
			"new_chat_id", member.Chat.ID,
		)
		if err := s.chatRepo.LinkMigratedChat(ctx, tx, member.Chat.ID, *member.Chat.MigrateFromChatID); err != nil {
			return fmt.Errorf("failed to link migrated chat: %w", err)
		}
	}

	return nil
}

func (s *IngestService) getUpdateType(update *models.Update) string {
	switch {
	case update.Message != nil:
		return "message"
	case update.EditedMessage != nil:
		return "edited_message"
	case update.MessageReaction != nil:
		return "message_reaction"
	case update.MessageReactionCount != nil:
		return "message_reaction_count"
	case update.MyChatMember != nil:
		return "my_chat_member"
	default:
		return "unknown"
	}
}
