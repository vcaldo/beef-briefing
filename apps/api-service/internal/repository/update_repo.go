package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/newrelic/go-agent/v3/newrelic"
)

type UpdateRepository struct {
	db    *sql.DB
	nrApp *newrelic.Application
}

func NewUpdateRepository(db *sql.DB, nrApp *newrelic.Application) *UpdateRepository {
	return &UpdateRepository{db: db, nrApp: nrApp}
}

// InsertUpdate inserts a new update record
func (r *UpdateRepository) InsertUpdate(ctx context.Context, tx *sql.Tx, updateID int64, updateType string, rawData interface{}) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:insert-update")
		defer segment.End()
	}

	rawJSON, err := json.Marshal(rawData)
	if err != nil {
		return fmt.Errorf("failed to marshal raw data: %w", err)
	}

	query := `
		INSERT INTO updates (update_id, update_type, raw_data)
		VALUES ($1, $2, $3)
		ON CONFLICT (update_id) DO NOTHING
	`

	_, err = tx.ExecContext(ctx, query, updateID, updateType, rawJSON)
	if err != nil {
		return fmt.Errorf("failed to insert update: %w", err)
	}

	return nil
}
