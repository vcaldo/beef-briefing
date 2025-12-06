package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) Close() error {
	return s.db.Close()
}

// Chat represents a Telegram chat/group
type Chat struct {
	ID        int64
	Type      string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// User represents a Telegram user
type User struct {
	ID        int64
	Username  string
	FirstName string
	LastName  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Message represents a Telegram message
type Message struct {
	ID                  int64
	TelegramMessageID   int64
	ChatID              int64
	UserID              *int64
	MessageDate         time.Time
	MessageType         string
	Text                *string
	ReplyToMessageID    *int64
	ForwardedFromUserID *int64
	ForwardedFromChatID *int64
	ForwardedDate       *time.Time
	EditDate            *time.Time
	MediaSHA256         *string
	MediaFileName       *string
	MediaFileSize       *int64
	MediaMimeType       *string
	MediaDuration       *int
	MediaWidth          *int
	MediaHeight         *int
	Entities            json.RawMessage
	Metadata            json.RawMessage
	Latitude            *float64
	Longitude           *float64
	VenueTitle          *string
	VenueAddress        *string
}

// ServiceMessage represents a service message (user joined, left, etc.)
type ServiceMessage struct {
	ID                int64
	TelegramMessageID int64
	ChatID            int64
	ActorUserID       *int64
	MessageDate       time.Time
	Action            string
	Metadata          json.RawMessage
}

// Reaction represents a message reaction
type Reaction struct {
	ID        int64
	MessageID int64
	UserID    int64
	Emoji     string
	CreatedAt time.Time
}

// UpsertChat creates or updates a chat
func (s *PostgresStore) UpsertChat(ctx context.Context, chat *Chat) error {
	query := `
		INSERT INTO chats (id, type, name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			type = EXCLUDED.type,
			name = EXCLUDED.name,
			updated_at = EXCLUDED.updated_at
	`
	_, err := s.db.ExecContext(ctx, query,
		chat.ID, chat.Type, chat.Name, chat.CreatedAt, chat.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to upsert chat: %w", err)
	}
	return nil
}

// UpsertUser creates or updates a user
func (s *PostgresStore) UpsertUser(ctx context.Context, user *User) error {
	query := `
		INSERT INTO users (id, username, first_name, last_name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			username = EXCLUDED.username,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			updated_at = EXCLUDED.updated_at
	`
	_, err := s.db.ExecContext(ctx, query,
		user.ID, user.Username, user.FirstName, user.LastName, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to upsert user: %w", err)
	}
	return nil
}

// InsertMessage creates a new message
func (s *PostgresStore) InsertMessage(ctx context.Context, msg *Message) (int64, error) {
	// Ensure we have valid JSON for JSONB fields
	entities := msg.Entities
	if len(entities) == 0 {
		entities = json.RawMessage("null")
	}

	metadata := msg.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage("{}")
	}

	// Build location point if coordinates are provided
	var args []interface{}

	if msg.Latitude != nil && msg.Longitude != nil {
		query := `
			INSERT INTO messages (
				telegram_message_id, chat_id, user_id, message_date, message_type,
				text, reply_to_message_id, forwarded_from_user_id, forwarded_from_chat_id,
				forwarded_date, edit_date, media_sha256, media_file_name, media_file_size,
				media_mime_type, media_duration_seconds, media_width, media_height,
				entities, metadata, location, venue_title, venue_address
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, ST_SetSRID(ST_MakePoint($21, $22), 4326)::geography, $23, $24)
			RETURNING id
		`
		args = []interface{}{
			msg.TelegramMessageID, msg.ChatID, msg.UserID, msg.MessageDate, msg.MessageType,
			msg.Text, msg.ReplyToMessageID, msg.ForwardedFromUserID, msg.ForwardedFromChatID,
			msg.ForwardedDate, msg.EditDate, msg.MediaSHA256, msg.MediaFileName, msg.MediaFileSize,
			msg.MediaMimeType, msg.MediaDuration, msg.MediaWidth, msg.MediaHeight,
			entities, metadata, *msg.Longitude, *msg.Latitude, msg.VenueTitle, msg.VenueAddress,
		}
		var id int64
		err := s.db.QueryRowContext(ctx, query, args...).Scan(&id)
		if err != nil {
			return 0, fmt.Errorf("failed to insert message with location: %w", err)
		}
		return id, nil
	}

	query := `
		INSERT INTO messages (
			telegram_message_id, chat_id, user_id, message_date, message_type,
			text, reply_to_message_id, forwarded_from_user_id, forwarded_from_chat_id,
			forwarded_date, edit_date, media_sha256, media_file_name, media_file_size,
			media_mime_type, media_duration_seconds, media_width, media_height,
			entities, metadata, venue_title, venue_address
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)
		RETURNING id
	`
	var id int64
	err := s.db.QueryRowContext(ctx, query,
		msg.TelegramMessageID, msg.ChatID, msg.UserID, msg.MessageDate, msg.MessageType,
		msg.Text, msg.ReplyToMessageID, msg.ForwardedFromUserID, msg.ForwardedFromChatID,
		msg.ForwardedDate, msg.EditDate, msg.MediaSHA256, msg.MediaFileName, msg.MediaFileSize,
		msg.MediaMimeType, msg.MediaDuration, msg.MediaWidth, msg.MediaHeight,
		entities, metadata, msg.VenueTitle, msg.VenueAddress,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to insert message: %w", err)
	}
	return id, nil
}

// InsertServiceMessage creates a new service message
func (s *PostgresStore) InsertServiceMessage(ctx context.Context, msg *ServiceMessage) error {
	// Ensure we have valid JSON for JSONB fields
	metadata := msg.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage("{}")
	}

	query := `
		INSERT INTO service_messages (
			telegram_message_id, chat_id, actor_user_id, message_date, action, metadata
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (chat_id, telegram_message_id) DO NOTHING
	`
	_, err := s.db.ExecContext(ctx, query,
		msg.TelegramMessageID, msg.ChatID, msg.ActorUserID, msg.MessageDate, msg.Action, metadata)
	if err != nil {
		return fmt.Errorf("failed to insert service message: %w", err)
	}
	return nil
}

// ShouldStoreLocationUpdate checks if a location update should be stored
// Returns true if there are no previous locations within 15 meters of the new location
func (s *PostgresStore) ShouldStoreLocationUpdate(ctx context.Context, chatID, telegramMessageID int64, newLat, newLng float64) (bool, error) {
	query := `
		SELECT COUNT(*)
		FROM messages
		WHERE chat_id = $1
			AND telegram_message_id = $2
			AND location IS NOT NULL
			AND ST_Distance(
				location,
				ST_SetSRID(ST_MakePoint($4, $3), 4326)::geography
			) < 15
	`
	var count int
	err := s.db.QueryRowContext(ctx, query, chatID, telegramMessageID, newLat, newLng).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check location distance: %w", err)
	}

	// Store if no locations are within 15 meters
	return count == 0, nil
}

// InsertReaction creates a new reaction
func (s *PostgresStore) InsertReaction(ctx context.Context, reaction *Reaction) error {
	query := `
		INSERT INTO message_reactions (message_id, user_id, emoji, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (message_id, user_id, emoji) DO NOTHING
	`
	_, err := s.db.ExecContext(ctx, query,
		reaction.MessageID, reaction.UserID, reaction.Emoji, reaction.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert reaction: %w", err)
	}
	return nil
}

// GetMessageIDByTelegramID retrieves the internal message ID by Telegram message ID and chat ID
func (s *PostgresStore) GetMessageIDByTelegramID(ctx context.Context, chatID, telegramMessageID int64) (int64, error) {
	var id int64
	query := `SELECT id FROM messages WHERE chat_id = $1 AND telegram_message_id = $2`
	err := s.db.QueryRowContext(ctx, query, chatID, telegramMessageID).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to get message ID: %w", err)
	}
	return id, nil
}

// MessageExists checks if a message already exists in the database
func (s *PostgresStore) MessageExists(ctx context.Context, chatID, telegramMessageID int64) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM messages WHERE chat_id = $1 AND telegram_message_id = $2)`
	err := s.db.QueryRowContext(ctx, query, chatID, telegramMessageID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check message existence: %w", err)
	}
	return exists, nil
}

// ServiceMessageExists checks if a service message already exists in the database
func (s *PostgresStore) ServiceMessageExists(ctx context.Context, chatID, telegramMessageID int64) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM service_messages WHERE chat_id = $1 AND telegram_message_id = $2)`
	err := s.db.QueryRowContext(ctx, query, chatID, telegramMessageID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check service message existence: %w", err)
	}
	return exists, nil
}

// BeginTx starts a new database transaction
func (s *PostgresStore) BeginTx(ctx context.Context) (*sql.Tx, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return tx, nil
}
