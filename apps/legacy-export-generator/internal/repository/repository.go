package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"

	"beef-briefing/apps/legacy-export-generator/internal/models"
)

// Repository handles database operations for legacy messages
type Repository struct {
	db *sql.DB
}

// Config holds database connection parameters
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

// New creates a new Repository with database connection
func New(cfg Config) (*Repository, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return &Repository{db: db}, nil
}

// Close closes the database connection
func (r *Repository) Close() error {
	return r.db.Close()
}

// Query retrieves messages within the specified date range and optional chat filter
func (r *Repository) Query(ctx context.Context, startDate, endDate time.Time, sourceChatID *int64) ([]models.LegacyMessage, error) {
	query := `
		SELECT
			id,
			message_id,
			message_type,
			timestamp,
			chat_id,
			user_id,
			reply_to_message_id,
			first_name,
			last_name,
			username,
			display_name,
			content,
			moderated
		FROM messages
		WHERE timestamp >= $1 AND timestamp <= $2
	`

	args := []any{startDate, endDate}

	if sourceChatID != nil {
		query += " AND chat_id = $3"
		args = append(args, *sourceChatID)
	}

	query += " ORDER BY timestamp ASC, id ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying messages: %w", err)
	}
	defer rows.Close()

	var messages []models.LegacyMessage
	for rows.Next() {
		var msg models.LegacyMessage
		err := rows.Scan(
			&msg.ID,
			&msg.MessageID,
			&msg.MessageType,
			&msg.Timestamp,
			&msg.ChatID,
			&msg.UserID,
			&msg.ReplyToMessageID,
			&msg.FirstName,
			&msg.LastName,
			&msg.Username,
			&msg.DisplayName,
			&msg.Content,
			&msg.Moderated,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning message row: %w", err)
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return messages, nil
}
