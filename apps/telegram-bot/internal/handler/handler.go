package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"beef-briefing/apps/telegram-bot/internal/store"

	tele "gopkg.in/telebot.v4"
)

type Handler struct {
	store *store.PostgresStore
}

func NewHandler(store *store.PostgresStore) *Handler {
	return &Handler{
		store: store,
	}
}

// HandleMessage processes incoming messages
func (h *Handler) HandleMessage(c tele.Context) error {
	msg := c.Message()
	ctx := context.Background()

	// Upsert chat
	chat := &store.Chat{
		ID:        msg.Chat.ID,
		Type:      string(msg.Chat.Type),
		Name:      msg.Chat.Title,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := h.store.UpsertChat(ctx, chat); err != nil {
		slog.Error("failed to upsert chat", "error", err, "chat_id", msg.Chat.ID)
		return err
	}

	// Upsert user (sender)
	if msg.Sender != nil {
		user := &store.User{
			ID:        msg.Sender.ID,
			Username:  msg.Sender.Username,
			FirstName: msg.Sender.FirstName,
			LastName:  msg.Sender.LastName,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := h.store.UpsertUser(ctx, user); err != nil {
			slog.Error("failed to upsert user", "error", err, "user_id", msg.Sender.ID)
			return err
		}
	}

	// Determine message type and handle media
	messageType := "text"
	var mediaFileName *string
	var mediaFileSize *int64
	var mediaMimeType *string
	var mediaDuration *int
	var mediaWidth *int
	var mediaHeight *int

	// Handle different media types
	if msg.Photo != nil {
		messageType = "photo"
		h.handlePhoto(msg.Photo, &mediaFileName, &mediaFileSize, &mediaMimeType, &mediaWidth, &mediaHeight)
	} else if msg.Video != nil {
		messageType = "video"
		h.handleVideo(msg.Video, &mediaFileName, &mediaFileSize, &mediaMimeType, &mediaDuration, &mediaWidth, &mediaHeight)
	} else if msg.Voice != nil {
		messageType = "voice"
		h.handleVoice(msg.Voice, &mediaFileName, &mediaFileSize, &mediaMimeType, &mediaDuration)
	} else if msg.Document != nil {
		messageType = "document"
		h.handleDocument(msg.Document, &mediaFileName, &mediaFileSize, &mediaMimeType)
	} else if msg.Sticker != nil {
		messageType = "sticker"
		h.handleSticker(msg.Sticker, &mediaFileName, &mediaFileSize, &mediaMimeType, &mediaWidth, &mediaHeight)
	} else if msg.Animation != nil {
		messageType = "animation"
		h.handleAnimation(msg.Animation, &mediaFileName, &mediaFileSize, &mediaMimeType, &mediaDuration, &mediaWidth, &mediaHeight)
	} else if msg.VideoNote != nil {
		messageType = "video_note"
		h.handleVideoNote(msg.VideoNote, &mediaFileName, &mediaFileSize, &mediaMimeType, &mediaDuration)
	}

	// Build entities JSON
	var entities json.RawMessage
	if len(msg.Entities) > 0 {
		entitiesData, _ := json.Marshal(msg.Entities)
		entities = entitiesData
	}

	// Prepare message for storage
	storeMsg := &store.Message{
		TelegramMessageID: int64(msg.ID),
		ChatID:            msg.Chat.ID,
		MessageDate:       time.Unix(msg.Unixtime, 0),
		MessageType:       messageType,
		MediaFileName:     mediaFileName,
		MediaFileSize:     mediaFileSize,
		MediaMimeType:     mediaMimeType,
		MediaDuration:     mediaDuration,
		MediaWidth:        mediaWidth,
		MediaHeight:       mediaHeight,
		Entities:          entities,
	}

	if msg.Sender != nil {
		userID := msg.Sender.ID
		storeMsg.UserID = &userID
	}

	if msg.Text != "" {
		storeMsg.Text = &msg.Text
	} else if msg.Caption != "" {
		storeMsg.Text = &msg.Caption
	}

	if msg.ReplyTo != nil {
		replyID := int64(msg.ReplyTo.ID)
		storeMsg.ReplyToMessageID = &replyID
	}

	// Insert message
	messageID, err := h.store.InsertMessage(ctx, storeMsg)
	if err != nil {
		slog.Error("failed to insert message", "error", err, "telegram_message_id", msg.ID)
		return err
	}

	slog.Info("message processed",
		"message_id", messageID,
		"telegram_message_id", msg.ID,
		"chat_id", msg.Chat.ID,
		"type", messageType)

	return nil
}

func (h *Handler) handlePhoto(photo *tele.Photo, name **string, size **int64, mimeType **string, width, height **int) {
	*name = stringPtr(photo.FileID)
	fileSize := int64(photo.FileSize)
	*size = &fileSize
	*mimeType = stringPtr("image/jpeg")
	w := photo.Width
	*width = &w
	photHeight := photo.Height
	*height = &photHeight
}

func (h *Handler) handleVideo(video *tele.Video, name **string, size **int64, mimeType **string, duration, width, height **int) {
	*name = stringPtr(video.FileName)
	fileSize := int64(video.FileSize)
	*size = &fileSize
	*mimeType = stringPtr(video.MIME)
	d := video.Duration
	*duration = &d
	w := video.Width
	*width = &w
	vidHeight := video.Height
	*height = &vidHeight
}

func (h *Handler) handleVoice(voice *tele.Voice, name **string, size **int64, mimeType **string, duration **int) {
	*name = stringPtr(voice.FileID)
	fileSize := int64(voice.FileSize)
	*size = &fileSize
	*mimeType = stringPtr(voice.MIME)
	d := voice.Duration
	*duration = &d
}

func (h *Handler) handleDocument(doc *tele.Document, name **string, size **int64, mimeType **string) {
	*name = stringPtr(doc.FileName)
	fileSize := int64(doc.FileSize)
	*size = &fileSize
	*mimeType = stringPtr(doc.MIME)
}

func (h *Handler) handleSticker(sticker *tele.Sticker, name **string, size **int64, mimeType **string, width, height **int) {
	*name = stringPtr(sticker.FileID)
	fileSize := int64(sticker.FileSize)
	*size = &fileSize
	*mimeType = stringPtr("image/webp")
	w := sticker.Width
	*width = &w
	stickerHeight := sticker.Height
	*height = &stickerHeight
}

func (h *Handler) handleAnimation(anim *tele.Animation, name **string, size **int64, mimeType **string, duration, width, height **int) {
	*name = stringPtr(anim.FileName)
	fileSize := int64(anim.FileSize)
	*size = &fileSize
	*mimeType = stringPtr(anim.MIME)
	d := anim.Duration
	*duration = &d
	w := anim.Width
	*width = &w
	animHeight := anim.Height
	*height = &animHeight
}

func (h *Handler) handleVideoNote(videoNote *tele.VideoNote, name **string, size **int64, mimeType **string, duration **int) {
	*name = stringPtr(videoNote.FileID)
	fileSize := int64(videoNote.FileSize)
	*size = &fileSize
	*mimeType = stringPtr("video/mp4")
	d := videoNote.Duration
	*duration = &d
}

func stringPtr(s string) *string {
	return &s
}
