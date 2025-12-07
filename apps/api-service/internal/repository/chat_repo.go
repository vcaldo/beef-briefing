package repository

import (
	"context"
	"database/sql"
	"fmt"

	"beef-briefing/apps/api-service/internal/models"
)

type ChatRepository struct {
	db *sql.DB
}

func NewChatRepository(db *sql.DB) *ChatRepository {
	return &ChatRepository{db: db}
}

// UpsertChat inserts or updates a chat
func (r *ChatRepository) UpsertChat(ctx context.Context, tx *sql.Tx, chat *models.Chat) error {
	query := `
		INSERT INTO chats (id, type, title, username, first_name, last_name, is_forum)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			type = EXCLUDED.type,
			title = EXCLUDED.title,
			username = EXCLUDED.username,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			is_forum = EXCLUDED.is_forum,
			updated_at = NOW()
	`

	_, err := tx.ExecContext(ctx, query,
		chat.ID,
		chat.Type,
		NullString(chat.Title),
		NullString(chat.Username),
		NullString(chat.FirstName),
		NullString(chat.LastName),
		chat.IsForum,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert chat: %w", err)
	}

	return nil
}
