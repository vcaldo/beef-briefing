package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"beef-briefing/apps/telegram-bot/internal/store"

	tele "gopkg.in/telebot.v4"
)

// HandleUserJoined processes user joined events
func (h *Handler) HandleUserJoined(c tele.Context) error {
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
		slog.Error("failed to upsert chat", "error", err)
		return err
	}

	// Upsert joined user
	if msg.UserJoined != nil {
		user := &store.User{
			ID:        msg.UserJoined.ID,
			Username:  msg.UserJoined.Username,
			FirstName: msg.UserJoined.FirstName,
			LastName:  msg.UserJoined.LastName,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := h.store.UpsertUser(ctx, user); err != nil {
			slog.Error("failed to upsert joined user", "error", err)
			return err
		}
	}

	// Upsert actor (the person who added the user)
	var actorID *int64
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
			slog.Error("failed to upsert actor user", "error", err)
		}
		id := msg.Sender.ID
		actorID = &id
	}

	// Create metadata
	metadata := map[string]interface{}{}
	if msg.UserJoined != nil {
		metadata["joined_user_id"] = msg.UserJoined.ID
		metadata["joined_username"] = msg.UserJoined.Username
		metadata["joined_first_name"] = msg.UserJoined.FirstName
		metadata["joined_last_name"] = msg.UserJoined.LastName
	}
	metadataJSON, _ := json.Marshal(metadata)

	// Insert service message
	serviceMsg := &store.ServiceMessage{
		TelegramMessageID: int64(msg.ID),
		ChatID:            msg.Chat.ID,
		ActorUserID:       actorID,
		MessageDate:       time.Unix(msg.Unixtime, 0),
		Action:            "user_joined",
		Metadata:          metadataJSON,
	}

	if err := h.store.InsertServiceMessage(ctx, serviceMsg); err != nil {
		slog.Error("failed to insert service message", "error", err)
		return err
	}

	slog.Info("user joined event processed", "chat_id", msg.Chat.ID, "telegram_message_id", msg.ID)
	return nil
}

// HandleUserLeft processes user left events
func (h *Handler) HandleUserLeft(c tele.Context) error {
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
		slog.Error("failed to upsert chat", "error", err)
		return err
	}

	// Upsert left user
	if msg.UserLeft != nil {
		user := &store.User{
			ID:        msg.UserLeft.ID,
			Username:  msg.UserLeft.Username,
			FirstName: msg.UserLeft.FirstName,
			LastName:  msg.UserLeft.LastName,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := h.store.UpsertUser(ctx, user); err != nil {
			slog.Error("failed to upsert left user", "error", err)
			return err
		}
	}

	// Create metadata
	metadata := map[string]interface{}{}
	if msg.UserLeft != nil {
		metadata["left_user_id"] = msg.UserLeft.ID
		metadata["left_username"] = msg.UserLeft.Username
		metadata["left_first_name"] = msg.UserLeft.FirstName
		metadata["left_last_name"] = msg.UserLeft.LastName
	}
	metadataJSON, _ := json.Marshal(metadata)

	// Insert service message
	serviceMsg := &store.ServiceMessage{
		TelegramMessageID: int64(msg.ID),
		ChatID:            msg.Chat.ID,
		MessageDate:       time.Unix(msg.Unixtime, 0),
		Action:            "user_left",
		Metadata:          metadataJSON,
	}

	if err := h.store.InsertServiceMessage(ctx, serviceMsg); err != nil {
		slog.Error("failed to insert service message", "error", err)
		return err
	}

	slog.Info("user left event processed", "chat_id", msg.Chat.ID, "telegram_message_id", msg.ID)
	return nil
}
