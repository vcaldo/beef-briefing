package repository

import (
	"context"
	"database/sql"
	"fmt"

	"beef-briefing/apps/api-service/internal/models"

	"github.com/newrelic/go-agent/v3/newrelic"
)

type UserRepository struct {
	db    *sql.DB
	nrApp *newrelic.Application
}

func NewUserRepository(db *sql.DB, nrApp *newrelic.Application) *UserRepository {
	return &UserRepository{db: db, nrApp: nrApp}
}

// UpsertUser inserts or updates a user
func (r *UserRepository) UpsertUser(ctx context.Context, tx *sql.Tx, user *models.User) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:upsert-user")
		defer segment.End()
	}

	query := `
		INSERT INTO users (id, is_bot, first_name, last_name, username, language_code, is_premium)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			is_bot = EXCLUDED.is_bot,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			username = EXCLUDED.username,
			language_code = EXCLUDED.language_code,
			is_premium = EXCLUDED.is_premium,
			updated_at = NOW()
	`

	_, err := tx.ExecContext(ctx, query,
		user.ID,
		user.IsBot,
		user.FirstName,
		NullString(user.LastName),
		NullString(user.Username),
		NullString(user.LanguageCode),
		user.IsPremium,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert user: %w", err)
	}

	return nil
}
