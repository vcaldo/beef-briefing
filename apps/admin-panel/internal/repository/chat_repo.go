package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Chat represents a chat with summary stats
type Chat struct {
	ID           int64
	Title        string
	Type         string
	Username     string
	FirstName    string
	LastName     string
	MessageCount int
	UserCount    int
	MediaCount   int
	FirstMessage time.Time
	LastMessage  time.Time
	LastActivity time.Time
}

// UserStats holds per-user statistics
type UserStats struct {
	UserID            int64
	Username          string
	FirstName         string
	LastName          string
	MessageCount      int
	ReactionsGiven    int
	ReactionsReceived int
	MediaShared       int
	FirstActive       time.Time
	LastActive        time.Time
}

// CalendarDay holds message count for a single day
type CalendarDay struct {
	Date  string
	Count int
}

// ChatRepository handles database operations for chats
type ChatRepository struct {
	db *sql.DB
}

// NewChatRepository creates a new ChatRepository
func NewChatRepository(db *sql.DB) *ChatRepository {
	return &ChatRepository{db: db}
}

// ListChats returns all chats with summary statistics
func (r *ChatRepository) ListChats(ctx context.Context) ([]Chat, error) {
	query := `
		SELECT
			c.id,
			COALESCE(c.title, '') as title,
			c.type,
			COALESCE(c.username, '') as username,
			COUNT(DISTINCT m.id) as message_count,
			COUNT(DISTINCT m.user_id) as user_count,
			COALESCE(MAX(m.date), c.created_at) as last_activity
		FROM chats c
		LEFT JOIN messages m ON m.chat_id = c.id
		GROUP BY c.id, c.title, c.type, c.username, c.created_at
		ORDER BY last_activity DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying chats: %w", err)
	}
	defer rows.Close()

	var chats []Chat
	for rows.Next() {
		var c Chat
		if err := rows.Scan(
			&c.ID,
			&c.Title,
			&c.Type,
			&c.Username,
			&c.MessageCount,
			&c.UserCount,
			&c.LastActivity,
		); err != nil {
			return nil, fmt.Errorf("scanning chat row: %w", err)
		}
		chats = append(chats, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating chat rows: %w", err)
	}

	return chats, nil
}

// GetChat returns detailed information about a single chat
func (r *ChatRepository) GetChat(ctx context.Context, chatID int64) (*Chat, error) {
	query := `
		SELECT
			c.id,
			COALESCE(c.title, '') as title,
			c.type,
			COALESCE(c.username, '') as username,
			COALESCE(c.first_name, '') as first_name,
			COALESCE(c.last_name, '') as last_name,
			COUNT(DISTINCT m.id) as message_count,
			COUNT(DISTINCT m.user_id) as user_count,
			COUNT(DISTINCT mf.id) as media_count,
			COALESCE(MIN(m.date), c.created_at) as first_message,
			COALESCE(MAX(m.date), c.created_at) as last_message
		FROM chats c
		LEFT JOIN messages m ON m.chat_id = c.id
		LEFT JOIN media_files mf ON mf.message_id = m.id
		WHERE c.id = $1
		GROUP BY c.id, c.title, c.type, c.username, c.first_name, c.last_name, c.created_at
	`

	var c Chat
	err := r.db.QueryRowContext(ctx, query, chatID).Scan(
		&c.ID,
		&c.Title,
		&c.Type,
		&c.Username,
		&c.FirstName,
		&c.LastName,
		&c.MessageCount,
		&c.UserCount,
		&c.MediaCount,
		&c.FirstMessage,
		&c.LastMessage,
	)
	if err != nil {
		return nil, fmt.Errorf("querying chat %d: %w", chatID, err)
	}

	return &c, nil
}

// GetAvailableYears returns years that have messages for a chat
func (r *ChatRepository) GetAvailableYears(ctx context.Context, chatID int64) ([]int, error) {
	query := `
		SELECT DISTINCT EXTRACT(YEAR FROM date)::int as year
		FROM messages
		WHERE chat_id = $1
		ORDER BY year DESC
	`

	rows, err := r.db.QueryContext(ctx, query, chatID)
	if err != nil {
		return nil, fmt.Errorf("querying available years: %w", err)
	}
	defer rows.Close()

	var years []int
	for rows.Next() {
		var year int
		if err := rows.Scan(&year); err != nil {
			return nil, fmt.Errorf("scanning year: %w", err)
		}
		years = append(years, year)
	}

	// If no years found, return current year
	if len(years) == 0 {
		years = append(years, time.Now().Year())
	}

	return years, rows.Err()
}

// GetUserStats returns per-user statistics for a chat
// month and year can be 0 to get all-time stats
func (r *ChatRepository) GetUserStats(ctx context.Context, chatID int64, month, year int) ([]UserStats, error) {
	// Build the date filter
	dateFilter := ""
	args := []interface{}{chatID}
	argNum := 2

	if year > 0 {
		dateFilter = fmt.Sprintf(" AND EXTRACT(YEAR FROM m.date) = $%d", argNum)
		args = append(args, year)
		argNum++
	}
	if month > 0 {
		dateFilter += fmt.Sprintf(" AND EXTRACT(MONTH FROM m.date) = $%d", argNum)
		args = append(args, month)
	}

	query := fmt.Sprintf(`
		SELECT
			u.id as user_id,
			COALESCE(u.username, '') as username,
			COALESCE(u.first_name, '') as first_name,
			COALESCE(u.last_name, '') as last_name,
			COUNT(DISTINCT m.id) as message_count,
			COALESCE(reactions_given.count, 0) as reactions_given,
			COALESCE(reactions_received.count, 0) as reactions_received,
			COUNT(DISTINCT mf.id) as media_shared,
			MIN(m.date) as first_active,
			MAX(m.date) as last_active
		FROM users u
		INNER JOIN messages m ON m.user_id = u.id AND m.chat_id = $1 %s
		LEFT JOIN media_files mf ON mf.message_id = m.id
		LEFT JOIN (
			SELECT user_id, COUNT(*) as count
			FROM message_reactions
			WHERE chat_id = $1 AND is_removed = false
			GROUP BY user_id
		) reactions_given ON reactions_given.user_id = u.id
		LEFT JOIN (
			SELECT m2.user_id, COUNT(*) as count
			FROM message_reactions mr
			INNER JOIN messages m2 ON m2.chat_id = mr.chat_id AND m2.message_id = mr.message_id
			WHERE mr.chat_id = $1 AND mr.is_removed = false
			GROUP BY m2.user_id
		) reactions_received ON reactions_received.user_id = u.id
		GROUP BY u.id, u.username, u.first_name, u.last_name, reactions_given.count, reactions_received.count
		ORDER BY message_count DESC
	`, dateFilter)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying user stats: %w", err)
	}
	defer rows.Close()

	var stats []UserStats
	for rows.Next() {
		var s UserStats
		if err := rows.Scan(
			&s.UserID,
			&s.Username,
			&s.FirstName,
			&s.LastName,
			&s.MessageCount,
			&s.ReactionsGiven,
			&s.ReactionsReceived,
			&s.MediaShared,
			&s.FirstActive,
			&s.LastActive,
		); err != nil {
			return nil, fmt.Errorf("scanning user stats: %w", err)
		}
		stats = append(stats, s)
	}

	return stats, rows.Err()
}

// GetCalendarData returns daily message counts for a year
func (r *ChatRepository) GetCalendarData(ctx context.Context, chatID int64, year int) ([]CalendarDay, error) {
	query := `
		SELECT
			TO_CHAR(date, 'YYYY-MM-DD') as date,
			COUNT(*) as count
		FROM messages
		WHERE chat_id = $1 AND EXTRACT(YEAR FROM date) = $2
		GROUP BY TO_CHAR(date, 'YYYY-MM-DD')
		ORDER BY date
	`

	rows, err := r.db.QueryContext(ctx, query, chatID, year)
	if err != nil {
		return nil, fmt.Errorf("querying calendar data: %w", err)
	}
	defer rows.Close()

	var data []CalendarDay
	for rows.Next() {
		var d CalendarDay
		if err := rows.Scan(&d.Date, &d.Count); err != nil {
			return nil, fmt.Errorf("scanning calendar day: %w", err)
		}
		data = append(data, d)
	}

	return data, rows.Err()
}
