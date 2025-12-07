package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"beef-briefing/apps/api-service/internal/models"
)

type MessageRepository struct {
	db *sql.DB
}

func NewMessageRepository(db *sql.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

// InsertMessage inserts a new message and returns its database ID
func (r *MessageRepository) InsertMessage(ctx context.Context, tx *sql.Tx, msg *models.Message) (int64, error) {
	var replyToMessageID sql.NullInt64
	if msg.ReplyToMessage != nil {
		replyToMessageID = sql.NullInt64{Int64: msg.ReplyToMessage.MessageID, Valid: true}
	}

	query := `
		INSERT INTO messages (
			message_id, chat_id, user_id, message_thread_id, reply_to_message_id,
			date, edit_date, text, caption, media_group_id,
			has_protected_content, is_automatic_forward, is_topic_message
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (chat_id, message_id) DO UPDATE SET
			edit_date = EXCLUDED.edit_date,
			text = EXCLUDED.text,
			caption = EXCLUDED.caption,
			updated_at = NOW()
		RETURNING id
	`

	var userID sql.NullInt64
	if msg.From != nil {
		userID = sql.NullInt64{Int64: msg.From.ID, Valid: true}
	}

	var dbID int64
	err := tx.QueryRowContext(ctx, query,
		msg.MessageID,
		msg.Chat.ID,
		userID,
		NullInt64(msg.MessageThreadID),
		replyToMessageID,
		time.Unix(msg.Date, 0).UTC(),
		NullTimeFromUnix(msg.EditDate),
		NullString(msg.Text),
		NullString(msg.Caption),
		NullString(msg.MediaGroupID),
		msg.HasProtectedContent,
		msg.IsAutomaticForward,
		msg.IsTopicMessage,
	).Scan(&dbID)
	if err != nil {
		return 0, fmt.Errorf("failed to insert message: %w", err)
	}

	return dbID, nil
}

// InsertMessageEdit inserts a message edit record
func (r *MessageRepository) InsertMessageEdit(ctx context.Context, tx *sql.Tx, messageID int64, editDate time.Time, previousText, newText, previousCaption, newCaption string) error {
	query := `
		INSERT INTO message_edits (message_id, edit_date, previous_text, new_text, previous_caption, new_caption)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := tx.ExecContext(ctx, query,
		messageID,
		editDate,
		NullString(previousText),
		NullString(newText),
		NullString(previousCaption),
		NullString(newCaption),
	)
	if err != nil {
		return fmt.Errorf("failed to insert message edit: %w", err)
	}

	return nil
}

// InsertEntity inserts a message entity
func (r *MessageRepository) InsertEntity(ctx context.Context, tx *sql.Tx, messageID int64, entity *models.MessageEntity) error {
	var userID sql.NullInt64
	if entity.User != nil {
		userID = sql.NullInt64{Int64: entity.User.ID, Valid: true}
	}

	query := `
		INSERT INTO message_entities (message_id, entity_type, entity_offset, entity_length, url, user_id, language, custom_emoji_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := tx.ExecContext(ctx, query,
		messageID,
		entity.Type,
		entity.Offset,
		entity.Length,
		NullString(entity.URL),
		userID,
		NullString(entity.Language),
		NullString(entity.CustomEmojiID),
	)
	if err != nil {
		return fmt.Errorf("failed to insert message entity: %w", err)
	}

	return nil
}

// GetMessageByID retrieves the previous message content for edit tracking
func (r *MessageRepository) GetMessageByID(ctx context.Context, tx *sql.Tx, chatID, messageID int64) (*models.DBMessage, error) {
	query := `
		SELECT id, text, caption FROM messages
		WHERE chat_id = $1 AND message_id = $2
	`

	var msg models.DBMessage
	err := tx.QueryRowContext(ctx, query, chatID, messageID).Scan(&msg.ID, &msg.Text, &msg.Caption)
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	return &msg, nil
}
