package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"beef-briefing/apps/api-service/internal/models"
	"beef-briefing/apps/api-service/internal/repository"
	"beef-briefing/apps/api-service/internal/storage"
	"beef-briefing/pkg/config"
)

type IngestHandler struct {
	db            *sql.DB
	storageClient *storage.MinIOClient
	config        *config.Config
	updateRepo    *repository.UpdateRepository
	chatRepo      *repository.ChatRepository
	userRepo      *repository.UserRepository
	messageRepo   *repository.MessageRepository
	reactionRepo  *repository.ReactionRepository
	mediaRepo     *repository.MediaRepository
}

func NewIngestHandler(
	db *sql.DB,
	storageClient *storage.MinIOClient,
	cfg *config.Config,
) *IngestHandler {
	return &IngestHandler{
		db:            db,
		storageClient: storageClient,
		config:        cfg,
		updateRepo:    repository.NewUpdateRepository(db),
		chatRepo:      repository.NewChatRepository(db),
		userRepo:      repository.NewUserRepository(db),
		messageRepo:   repository.NewMessageRepository(db),
		reactionRepo:  repository.NewReactionRepository(db),
		mediaRepo:     repository.NewMediaRepository(db),
	}
}

// HandleIngest processes incoming multipart ingestion requests
func (h *IngestHandler) HandleIngest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse multipart form with size limit
	if err := r.ParseMultipartForm(h.config.MaxUploadSizeBytes()); err != nil {
		slog.Error("failed to parse multipart form", "error", err)
		http.Error(w, "invalid multipart request", http.StatusBadRequest)
		return
	}

	// Extract update JSON from form field
	updateJSON := r.FormValue("update")
	if updateJSON == "" {
		slog.Error("missing update field in request")
		http.Error(w, "missing update field", http.StatusBadRequest)
		return
	}

	var update models.Update
	if err := json.Unmarshal([]byte(updateJSON), &update); err != nil {
		slog.Error("failed to decode update", "error", err)
		http.Error(w, "invalid update JSON", http.StatusBadRequest)
		return
	}

	slog.Info("received update", "update_id", update.UpdateID)

	// Extract files from multipart form
	files := make(map[string][]byte)
	if r.MultipartForm != nil && r.MultipartForm.File != nil {
		for fileID, fileHeaders := range r.MultipartForm.File {
			if len(fileHeaders) == 0 {
				continue
			}

			file, err := fileHeaders[0].Open()
			if err != nil {
				slog.Warn("failed to open uploaded file", "file_id", fileID, "error", err)
				continue
			}

			data, err := io.ReadAll(file)
			file.Close()
			if err != nil {
				slog.Warn("failed to read uploaded file", "file_id", fileID, "error", err)
				continue
			}

			files[fileID] = data
			slog.Debug("extracted file from request", "file_id", fileID, "size", len(data))
		}
	}

	// Process update with files
	if err := h.processUpdate(ctx, &update, files); err != nil {
		slog.Error("failed to process update",
			"update_id", update.UpdateID,
			"error", err,
		)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *IngestHandler) processUpdate(ctx context.Context, update *models.Update, files map[string][]byte) error {
	// Start transaction
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Determine update type and insert update record
	updateType := h.getUpdateType(update)
	if err := h.updateRepo.InsertUpdate(ctx, tx, update.UpdateID, updateType, update); err != nil {
		return fmt.Errorf("failed to insert update: %w", err)
	}

	// Process based on update type
	switch {
	case update.Message != nil:
		if err := h.processMessage(ctx, tx, update.Message, files, false); err != nil {
			return fmt.Errorf("failed to process message: %w", err)
		}

	case update.EditedMessage != nil:
		if err := h.processMessage(ctx, tx, update.EditedMessage, files, true); err != nil {
			return fmt.Errorf("failed to process edited message: %w", err)
		}

	case update.MessageReaction != nil:
		if err := h.processMessageReaction(ctx, tx, update.MessageReaction); err != nil {
			return fmt.Errorf("failed to process message reaction: %w", err)
		}

	case update.MessageReactionCount != nil:
		if err := h.processReactionCount(ctx, tx, update.MessageReactionCount); err != nil {
			return fmt.Errorf("failed to process reaction count: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (h *IngestHandler) processMessage(ctx context.Context, tx *sql.Tx, msg *models.Message, files map[string][]byte, isEdit bool) error {
	// Upsert chat
	if err := h.chatRepo.UpsertChat(ctx, tx, &msg.Chat); err != nil {
		return fmt.Errorf("failed to upsert chat: %w", err)
	}

	// Upsert user if present
	if msg.From != nil {
		if err := h.userRepo.UpsertUser(ctx, tx, msg.From); err != nil {
			return fmt.Errorf("failed to upsert user: %w", err)
		}
	}

	// Track edit if this is an edited message
	if isEdit && msg.EditDate != nil {
		// Get previous message content
		prevMsg, err := h.messageRepo.GetMessageByID(ctx, tx, msg.Chat.ID, msg.MessageID)
		if err == nil {
			// Insert edit record
			editDate := time.Unix(*msg.EditDate, 0)
			if err := h.messageRepo.InsertMessageEdit(ctx, tx, prevMsg.ID, editDate,
				prevMsg.Text.String, msg.Text,
				prevMsg.Caption.String, msg.Caption); err != nil {
				slog.Warn("failed to insert message edit", "error", err)
			}
		}
	}

	// Insert/update message
	messageID, err := h.messageRepo.InsertMessage(ctx, tx, msg)
	if err != nil {
		return fmt.Errorf("failed to insert message: %w", err)
	}

	// Insert entities
	for _, entity := range msg.Entities {
		if err := h.messageRepo.InsertEntity(ctx, tx, messageID, &entity); err != nil {
			slog.Warn("failed to insert entity", "error", err)
		}
	}

	for _, entity := range msg.CaptionEntities {
		if err := h.messageRepo.InsertEntity(ctx, tx, messageID, &entity); err != nil {
			slog.Warn("failed to insert caption entity", "error", err)
		}
	}

	// Process media with provided files
	if err := h.processMedia(ctx, tx, messageID, msg, files); err != nil {
		return fmt.Errorf("failed to process media: %w", err)
	}

	// Process location (standalone, not part of venue)
	if msg.Location != nil && msg.Venue == nil {
		if err := h.mediaRepo.InsertLocation(ctx, tx, messageID, msg.Location); err != nil {
			slog.Warn("failed to insert location", "error", err)
		}
	}

	// Process venue (includes location)
	if msg.Venue != nil {
		locationID, err := h.mediaRepo.InsertLocationReturningID(ctx, tx, messageID, &msg.Venue.Location)
		if err != nil {
			slog.Warn("failed to insert venue location", "error", err)
		} else {
			if err := h.mediaRepo.InsertVenue(ctx, tx, messageID, locationID, msg.Venue); err != nil {
				slog.Warn("failed to insert venue", "error", err)
			}
		}
	}

	// Process contact
	if msg.Contact != nil {
		if err := h.mediaRepo.InsertContact(ctx, tx, messageID, msg.Contact); err != nil {
			slog.Warn("failed to insert contact", "error", err)
		}
	}

	// Process poll
	if msg.Poll != nil {
		pollID, err := h.mediaRepo.InsertPoll(ctx, tx, messageID, msg.Poll)
		if err != nil {
			slog.Warn("failed to insert poll", "error", err)
		} else {
			for i, option := range msg.Poll.Options {
				if err := h.mediaRepo.InsertPollOption(ctx, tx, pollID, i, &option); err != nil {
					slog.Warn("failed to insert poll option", "error", err, "index", i)
				}
			}
		}
	}

	// Process dice
	if msg.Dice != nil {
		if err := h.mediaRepo.InsertDice(ctx, tx, messageID, msg.Dice); err != nil {
			slog.Warn("failed to insert dice", "error", err)
		}
	}

	return nil
}

func (h *IngestHandler) processMedia(ctx context.Context, tx *sql.Tx, messageID int64, msg *models.Message, files map[string][]byte) error {
	// Process photos (multiple sizes)
	for _, photo := range msg.Photo {
		data, ok := files[photo.FileID]
		if !ok {
			slog.Warn("photo file not found in request, skipping", "file_id", photo.FileID)
			continue
		}

		// Compute hash for deduplication
		fileHash := storage.ComputeFileHash(data)

		// Check if file already exists in database
		existingKey, err := h.mediaRepo.GetObjectKeyByHash(ctx, tx, fileHash)
		if err != nil {
			slog.Error("failed to check file hash", "file_id", photo.FileID, "error", err)
			continue
		}

		var objectKey string
		if existingKey != "" {
			// File already exists, reuse existing object key
			objectKey = existingKey
			slog.Debug("photo already exists in storage, reusing", "file_id", photo.FileID, "object_key", objectKey)
		} else {
			// Upload new file to MinIO
			mimeType := "image/jpeg"
			objectKey, _, err = h.storageClient.UploadMedia(ctx, photo.FileID, data, mimeType, "photo")
			if err != nil {
				slog.Error("failed to upload photo to MinIO", "file_id", photo.FileID, "error", err)
				continue
			}
		}

		if err := h.mediaRepo.InsertPhoto(ctx, tx, messageID, &photo, objectKey, fileHash); err != nil {
			slog.Warn("failed to insert photo", "error", err)
		}
	}

	// Process video
	if msg.Video != nil {
		if err := h.processMediaFile(ctx, tx, messageID, "video", msg.Video.FileID, msg.Video.FileUniqueID,
			msg.Video.FileSize, msg.Video.MimeType, msg.Video.FileName,
			&msg.Video.Duration, &msg.Video.Width, &msg.Video.Height, "", "", files); err != nil {
			slog.Error("failed to process video", "error", err)
		}
	}

	// Process audio
	if msg.Audio != nil {
		if err := h.processMediaFile(ctx, tx, messageID, "audio", msg.Audio.FileID, msg.Audio.FileUniqueID,
			msg.Audio.FileSize, msg.Audio.MimeType, msg.Audio.FileName,
			&msg.Audio.Duration, nil, nil, msg.Audio.Performer, msg.Audio.Title, files); err != nil {
			slog.Error("failed to process audio", "error", err)
		}
	}

	// Process voice
	if msg.Voice != nil {
		if err := h.processMediaFile(ctx, tx, messageID, "voice", msg.Voice.FileID, msg.Voice.FileUniqueID,
			msg.Voice.FileSize, msg.Voice.MimeType, "",
			&msg.Voice.Duration, nil, nil, "", "", files); err != nil {
			slog.Error("failed to process voice", "error", err)
		}
	}

	// Process document
	if msg.Document != nil {
		if err := h.processMediaFile(ctx, tx, messageID, "document", msg.Document.FileID, msg.Document.FileUniqueID,
			msg.Document.FileSize, msg.Document.MimeType, msg.Document.FileName,
			nil, nil, nil, "", "", files); err != nil {
			slog.Error("failed to process document", "error", err)
		}
	}

	// Process animation
	if msg.Animation != nil {
		if err := h.processMediaFile(ctx, tx, messageID, "animation", msg.Animation.FileID, msg.Animation.FileUniqueID,
			msg.Animation.FileSize, msg.Animation.MimeType, msg.Animation.FileName,
			&msg.Animation.Duration, &msg.Animation.Width, &msg.Animation.Height, "", "", files); err != nil {
			slog.Error("failed to process animation", "error", err)
		}
	}

	// Process video note
	if msg.VideoNote != nil {
		if err := h.processMediaFile(ctx, tx, messageID, "video_note", msg.VideoNote.FileID, msg.VideoNote.FileUniqueID,
			msg.VideoNote.FileSize, "", "",
			&msg.VideoNote.Duration, &msg.VideoNote.Length, &msg.VideoNote.Length, "", "", files); err != nil {
			slog.Error("failed to process video note", "error", err)
		}
	}

	// Process sticker (stored as 'sticker' type in media_files with additional metadata in stickers table)
	if msg.Sticker != nil {
		if err := h.processSticker(ctx, tx, messageID, msg.Sticker, files); err != nil {
			slog.Error("failed to process sticker", "error", err)
		}
	}

	// Process game
	if msg.Game != nil {
		if err := h.processGame(ctx, tx, messageID, msg.Game, files); err != nil {
			slog.Error("failed to process game", "error", err)
		}
	}

	return nil
}

func (h *IngestHandler) processMediaFile(ctx context.Context, tx *sql.Tx, messageID int64, mediaType, fileID, fileUniqueID string, fileSize *int64, mimeType, fileName string, duration, width, height *int, performer, title string, files map[string][]byte) error {
	// Get file from provided files map
	data, ok := files[fileID]
	if !ok {
		slog.Warn("media file not found in request, skipping",
			"file_id", fileID,
			"media_type", mediaType,
		)
		return nil // Skip missing file, don't error
	}

	// Use provided mime type or default
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Compute hash for deduplication
	fileHash := storage.ComputeFileHash(data)

	// Check if file already exists in database
	existingKey, err := h.mediaRepo.GetObjectKeyByHash(ctx, tx, fileHash)
	if err != nil {
		return fmt.Errorf("failed to check file hash: %w", err)
	}

	var objectKey string
	if existingKey != "" {
		// File already exists, reuse existing object key
		objectKey = existingKey
		slog.Debug("media already exists in storage, reusing",
			"file_id", fileID,
			"media_type", mediaType,
			"object_key", objectKey,
		)
	} else {
		// Upload new file to MinIO
		objectKey, _, err = h.storageClient.UploadMedia(ctx, fileID, data, mimeType, mediaType)
		if err != nil {
			return fmt.Errorf("failed to upload to MinIO: %w", err)
		}
	}

	// Insert media file record
	return h.mediaRepo.InsertMediaFile(ctx, tx, messageID, mediaType, fileID, fileUniqueID, objectKey, fileHash,
		fileSize, mimeType, fileName, duration, width, height, performer, title)
}

// processSticker handles sticker media - stores file in media_files as 'sticker' type and metadata in stickers table
func (h *IngestHandler) processSticker(ctx context.Context, tx *sql.Tx, messageID int64, sticker *models.Sticker, files map[string][]byte) error {
	data, ok := files[sticker.FileID]
	if !ok {
		slog.Warn("sticker file not found in request, skipping", "file_id", sticker.FileID)
		return nil
	}

	// Determine MIME type based on sticker type
	mimeType := "image/webp"
	if sticker.IsAnimated {
		mimeType = "application/x-tgsticker"
	} else if sticker.IsVideo {
		mimeType = "video/webm"
	}

	// Compute hash for deduplication
	fileHash := storage.ComputeFileHash(data)

	// Check if file already exists in database
	existingKey, err := h.mediaRepo.GetObjectKeyByHash(ctx, tx, fileHash)
	if err != nil {
		return fmt.Errorf("failed to check file hash: %w", err)
	}

	var objectKey string
	if existingKey != "" {
		objectKey = existingKey
		slog.Debug("sticker already exists in storage, reusing", "file_id", sticker.FileID, "object_key", objectKey)
	} else {
		objectKey, _, err = h.storageClient.UploadMedia(ctx, sticker.FileID, data, mimeType, "sticker")
		if err != nil {
			return fmt.Errorf("failed to upload sticker to MinIO: %w", err)
		}
	}

	// Insert sticker into media_files table with type 'sticker'
	mediaFileID, err := h.mediaRepo.InsertMediaFileReturningID(ctx, tx, messageID, "sticker",
		sticker.FileID, sticker.FileUniqueID, objectKey, fileHash,
		sticker.FileSize, mimeType, "",
		nil, &sticker.Width, &sticker.Height, "", "")
	if err != nil {
		return fmt.Errorf("failed to insert sticker media file: %w", err)
	}

	// Insert sticker-specific metadata
	if err := h.mediaRepo.InsertSticker(ctx, tx, messageID, mediaFileID, sticker); err != nil {
		return fmt.Errorf("failed to insert sticker metadata: %w", err)
	}

	return nil
}

// processGame handles game messages - stores game info and photos
func (h *IngestHandler) processGame(ctx context.Context, tx *sql.Tx, messageID int64, game *models.Game, files map[string][]byte) error {
	// Insert game record
	gameID, err := h.mediaRepo.InsertGame(ctx, tx, messageID, game)
	if err != nil {
		return fmt.Errorf("failed to insert game: %w", err)
	}

	// Process game photos
	for _, photo := range game.Photo {
		data, ok := files[photo.FileID]
		if !ok {
			slog.Warn("game photo file not found in request, skipping", "file_id", photo.FileID)
			continue
		}

		// Compute hash for deduplication
		fileHash := storage.ComputeFileHash(data)

		// Check if file already exists in database
		existingKey, err := h.mediaRepo.GetObjectKeyByHash(ctx, tx, fileHash)
		if err != nil {
			slog.Error("failed to check file hash", "file_id", photo.FileID, "error", err)
			continue
		}

		var objectKey string
		if existingKey != "" {
			objectKey = existingKey
			slog.Debug("game photo already exists in storage, reusing", "file_id", photo.FileID, "object_key", objectKey)
		} else {
			mimeType := "image/jpeg"
			objectKey, _, err = h.storageClient.UploadMedia(ctx, photo.FileID, data, mimeType, "game_photo")
			if err != nil {
				slog.Error("failed to upload game photo to MinIO", "file_id", photo.FileID, "error", err)
				continue
			}
		}

		if err := h.mediaRepo.InsertGamePhoto(ctx, tx, gameID, &photo, objectKey, fileHash); err != nil {
			slog.Warn("failed to insert game photo", "error", err)
		}
	}

	return nil
}

func (h *IngestHandler) processMessageReaction(ctx context.Context, tx *sql.Tx, reaction *models.MessageReactionUpdated) error {
	// Upsert chat
	if err := h.chatRepo.UpsertChat(ctx, tx, &reaction.Chat); err != nil {
		return fmt.Errorf("failed to upsert chat: %w", err)
	}

	// Upsert user if present
	if reaction.User != nil {
		if err := h.userRepo.UpsertUser(ctx, tx, reaction.User); err != nil {
			return fmt.Errorf("failed to upsert user: %w", err)
		}
	}

	// Upsert actor chat if present
	if reaction.ActorChat != nil {
		if err := h.chatRepo.UpsertChat(ctx, tx, reaction.ActorChat); err != nil {
			return fmt.Errorf("failed to upsert actor chat: %w", err)
		}
	}

	// Insert reaction
	return h.reactionRepo.InsertMessageReaction(ctx, tx, reaction)
}

func (h *IngestHandler) processReactionCount(ctx context.Context, tx *sql.Tx, countUpdate *models.MessageReactionCountUpdate) error {
	// Upsert chat
	if err := h.chatRepo.UpsertChat(ctx, tx, &countUpdate.Chat); err != nil {
		return fmt.Errorf("failed to upsert chat: %w", err)
	}

	// Upsert reaction counts
	return h.reactionRepo.UpsertReactionCount(ctx, tx, countUpdate)
}

func (h *IngestHandler) getUpdateType(update *models.Update) string {
	switch {
	case update.Message != nil:
		return "message"
	case update.EditedMessage != nil:
		return "edited_message"
	case update.MessageReaction != nil:
		return "message_reaction"
	case update.MessageReactionCount != nil:
		return "message_reaction_count"
	default:
		return "unknown"
	}
}
