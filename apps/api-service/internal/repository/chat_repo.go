package repository

import (
	"context"
	"database/sql"
	"fmt"

	"beef-briefing/apps/api-service/internal/models"
	"beef-briefing/apps/api-service/internal/nrutil"

	"github.com/newrelic/go-agent/v3/newrelic"
)

type ChatRepository struct {
	db    *sql.DB
	nrApp *newrelic.Application
}

func NewChatRepository(db *sql.DB, nrApp *newrelic.Application) *ChatRepository {
	return &ChatRepository{db: db, nrApp: nrApp}
}

// UpsertChat inserts or updates a chat
func (r *ChatRepository) UpsertChat(ctx context.Context, tx *sql.Tx, chat *models.Chat) error {
	defer nrutil.StartSegment(ctx, "db:upsert-chat")()

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

// LinkMigratedChat records the migration relationship between old group and new supergroup
func (r *ChatRepository) LinkMigratedChat(ctx context.Context, tx *sql.Tx, newChatID, oldChatID int64) error {
	defer nrutil.StartSegment(ctx, "db:link-migrated-chat")()

	query := `
		UPDATE chats
		SET migrated_from_chat_id = $2, updated_at = NOW()
		WHERE id = $1
	`

	_, err := tx.ExecContext(ctx, query, newChatID, oldChatID)
	if err != nil {
		return fmt.Errorf("failed to link migrated chat: %w", err)
	}

	return nil
}
