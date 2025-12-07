package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"beef-briefing/apps/api-service/internal/models"
	"beef-briefing/apps/api-service/internal/repository"
	"beef-briefing/apps/api-service/internal/storage"
	"beef-briefing/apps/api-service/internal/telegram"
)

type WebhookHandler struct {
	db            *sql.DB
	fileClient    *telegram.FileClient
	storageClient *storage.MinIOClient
	updateRepo    *repository.UpdateRepository
	chatRepo      *repository.ChatRepository
	userRepo      *repository.UserRepository
	messageRepo   *repository.MessageRepository
	reactionRepo  *repository.ReactionRepository
	mediaRepo     *repository.MediaRepository
}

func NewWebhookHandler(
	db *sql.DB,
	fileClient *telegram.FileClient,
	storageClient *storage.MinIOClient,
) *WebhookHandler {
	return &WebhookHandler{
		db:            db,
		fileClient:    fileClient,
		storageClient: storageClient,
		updateRepo:    repository.NewUpdateRepository(db),
		chatRepo:      repository.NewChatRepository(db),
		userRepo:      repository.NewUserRepository(db),
		messageRepo:   repository.NewMessageRepository(db),
		reactionRepo:  repository.NewReactionRepository(db),
		mediaRepo:     repository.NewMediaRepository(db),
	}
}

// HandleUpdate processes incoming Telegram updates
func (h *WebhookHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Decode update
	var update models.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		slog.Error("failed to decode update", "error", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	slog.Info("received update", "update_id", update.UpdateID)

	// Process update based on type
	if err := h.processUpdate(ctx, &update); err != nil {
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

func (h *WebhookHandler) processUpdate(ctx context.Context, update *models.Update) error {
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
		if err := h.processMessage(ctx, tx, update.Message, false); err != nil {
			return fmt.Errorf("failed to process message: %w", err)
		}

	case update.EditedMessage != nil:
		if err := h.processMessage(ctx, tx, update.EditedMessage, true); err != nil {
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

func (h *WebhookHandler) processMessage(ctx context.Context, tx *sql.Tx, msg *models.Message, isEdit bool) error {
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

	// Process media - download and upload to MinIO before committing transaction
	if err := h.processMedia(ctx, tx, messageID, msg); err != nil {
		return fmt.Errorf("failed to process media: %w", err)
	}

	// Process location
	if msg.Location != nil {
		if err := h.mediaRepo.InsertLocation(ctx, tx, messageID, msg.Location); err != nil {
			slog.Warn("failed to insert location", "error", err)
		}
	}

	return nil
}

func (h *WebhookHandler) processMedia(ctx context.Context, tx *sql.Tx, messageID int64, msg *models.Message) error {
	// Process photos (multiple sizes)
	for _, photo := range msg.Photo {
		data, mimeType, err := h.fileClient.DownloadFile(ctx, photo.FileID)
		if err != nil {
			slog.Error("failed to download photo", "file_id", photo.FileID, "error", err)
			continue
		}

		objectKey, err := h.storageClient.UploadMedia(ctx, photo.FileID, data, mimeType, "photo")
		if err != nil {
			slog.Error("failed to upload photo to MinIO", "file_id", photo.FileID, "error", err)
			continue
		}

		if err := h.mediaRepo.InsertPhoto(ctx, tx, messageID, &photo, objectKey); err != nil {
			slog.Warn("failed to insert photo", "error", err)
		}
	}

	// Process video
	if msg.Video != nil {
		if err := h.processMediaFile(ctx, tx, messageID, "video", msg.Video.FileID, msg.Video.FileUniqueID,
			msg.Video.FileSize, msg.Video.MimeType, msg.Video.FileName,
			&msg.Video.Duration, &msg.Video.Width, &msg.Video.Height, "", ""); err != nil {
			slog.Error("failed to process video", "error", err)
		}
	}

	// Process audio
	if msg.Audio != nil {
		if err := h.processMediaFile(ctx, tx, messageID, "audio", msg.Audio.FileID, msg.Audio.FileUniqueID,
			msg.Audio.FileSize, msg.Audio.MimeType, msg.Audio.FileName,
			&msg.Audio.Duration, nil, nil, msg.Audio.Performer, msg.Audio.Title); err != nil {
			slog.Error("failed to process audio", "error", err)
		}
	}

	// Process voice
	if msg.Voice != nil {
		if err := h.processMediaFile(ctx, tx, messageID, "voice", msg.Voice.FileID, msg.Voice.FileUniqueID,
			msg.Voice.FileSize, msg.Voice.MimeType, "",
			&msg.Voice.Duration, nil, nil, "", ""); err != nil {
			slog.Error("failed to process voice", "error", err)
		}
	}

	// Process document
	if msg.Document != nil {
		if err := h.processMediaFile(ctx, tx, messageID, "document", msg.Document.FileID, msg.Document.FileUniqueID,
			msg.Document.FileSize, msg.Document.MimeType, msg.Document.FileName,
			nil, nil, nil, "", ""); err != nil {
			slog.Error("failed to process document", "error", err)
		}
	}

	// Process animation
	if msg.Animation != nil {
		if err := h.processMediaFile(ctx, tx, messageID, "animation", msg.Animation.FileID, msg.Animation.FileUniqueID,
			msg.Animation.FileSize, msg.Animation.MimeType, msg.Animation.FileName,
			&msg.Animation.Duration, &msg.Animation.Width, &msg.Animation.Height, "", ""); err != nil {
			slog.Error("failed to process animation", "error", err)
		}
	}

	// Process video note
	if msg.VideoNote != nil {
		if err := h.processMediaFile(ctx, tx, messageID, "video_note", msg.VideoNote.FileID, msg.VideoNote.FileUniqueID,
			msg.VideoNote.FileSize, "", "",
			&msg.VideoNote.Duration, &msg.VideoNote.Length, &msg.VideoNote.Length, "", ""); err != nil {
			slog.Error("failed to process video note", "error", err)
		}
	}

	return nil
}

func (h *WebhookHandler) processMediaFile(ctx context.Context, tx *sql.Tx, messageID int64, mediaType, fileID, fileUniqueID string, fileSize *int64, mimeType, fileName string, duration, width, height *int, performer, title string) error {
	// Download file from Telegram
	data, detectedMimeType, err := h.fileClient.DownloadFile(ctx, fileID)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}

	// Use detected mime type if not provided
	if mimeType == "" {
		mimeType = detectedMimeType
	}

	// Upload to MinIO
	objectKey, err := h.storageClient.UploadMedia(ctx, fileID, data, mimeType, mediaType)
	if err != nil {
		return fmt.Errorf("failed to upload to MinIO: %w", err)
	}

	// Insert media file record
	return h.mediaRepo.InsertMediaFile(ctx, tx, messageID, mediaType, fileID, fileUniqueID, objectKey,
		fileSize, mimeType, fileName, duration, width, height, performer, title)
}

func (h *WebhookHandler) processMessageReaction(ctx context.Context, tx *sql.Tx, reaction *models.MessageReactionUpdated) error {
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

func (h *WebhookHandler) processReactionCount(ctx context.Context, tx *sql.Tx, countUpdate *models.MessageReactionCountUpdate) error {
	// Upsert chat
	if err := h.chatRepo.UpsertChat(ctx, tx, &countUpdate.Chat); err != nil {
		return fmt.Errorf("failed to upsert chat: %w", err)
	}

	// Upsert reaction counts
	return h.reactionRepo.UpsertReactionCount(ctx, tx, countUpdate)
}

func (h *WebhookHandler) getUpdateType(update *models.Update) string {
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
