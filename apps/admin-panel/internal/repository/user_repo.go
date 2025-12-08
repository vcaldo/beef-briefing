package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// User represents a user with profile info and stats
type User struct {
	ID           int64
	IsBot        bool
	FirstName    string
	LastName     string
	Username     string
	LanguageCode string
	IsPremium    bool
	CreatedAt    time.Time

	// Computed stats
	TotalMessages     int
	TotalChats        int
	MediaShared       int
	FirstActive       time.Time
	LastActive        time.Time
	ReactionsGiven    int
	ReactionsReceived int
}

// UserChat represents a chat the user participates in
type UserChat struct {
	ChatID       int64
	Title        string
	Type         string
	Username     string
	MessageCount int
	FirstActive  time.Time
	LastActive   time.Time
}

// UserMessage represents a message from the user
type UserMessage struct {
	ID        int64
	ChatID    int64
	ChatTitle string
	Date      time.Time
	Text      string
	HasMedia  bool
}

// UserReactionStats holds reaction statistics for a user
type UserReactionStats struct {
	TotalGiven    int
	TotalReceived int
	TopGiven      []EmojiCount
	TopReceived   []EmojiCount
}

// EmojiCount represents an emoji and its count
type EmojiCount struct {
	Emoji string
	Count int
}

// UserStreaks holds streak information for a user
type UserStreaks struct {
	MaxPostingStreak  int  // max consecutive days with posts
	MaxSilenceStreak  int  // max consecutive days without posts
	CurrentStreak     int  // current streak from today backwards
	IsCurrentlyActive bool // whether current streak is posting (true) or silence (false)
}

// UserRepository handles database operations for users
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository creates a new UserRepository
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// GetUserByID returns a user with their profile and aggregate stats
func (r *UserRepository) GetUserByID(ctx context.Context, userID int64) (*User, error) {
	query := `
		SELECT
			u.id,
			u.is_bot,
			COALESCE(u.first_name, '') as first_name,
			COALESCE(u.last_name, '') as last_name,
			COALESCE(u.username, '') as username,
			COALESCE(u.language_code, '') as language_code,
			u.is_premium,
			u.created_at,
			COUNT(DISTINCT m.id) as total_messages,
			COUNT(DISTINCT m.chat_id) as total_chats,
			COUNT(DISTINCT mf.id) as media_shared,
			MIN(m.date) as first_active,
			MAX(m.date) as last_active,
			COALESCE(rg.count, 0) as reactions_given,
			COALESCE(rr.count, 0) as reactions_received
		FROM users u
		LEFT JOIN messages m ON m.user_id = u.id
		LEFT JOIN media_files mf ON mf.message_id = m.id
		LEFT JOIN (
			SELECT user_id, COUNT(*) as count
			FROM message_reactions
			WHERE is_removed = false
			GROUP BY user_id
		) rg ON rg.user_id = u.id
		LEFT JOIN (
			SELECT m2.user_id, COUNT(*) as count
			FROM message_reactions mr
			INNER JOIN messages m2 ON m2.chat_id = mr.chat_id AND m2.message_id = mr.message_id
			WHERE mr.is_removed = false
			GROUP BY m2.user_id
		) rr ON rr.user_id = u.id
		WHERE u.id = $1
		GROUP BY u.id, u.is_bot, u.first_name, u.last_name, u.username, u.language_code, u.is_premium, u.created_at, rg.count, rr.count
	`

	var user User
	var firstActive, lastActive sql.NullTime
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&user.ID,
		&user.IsBot,
		&user.FirstName,
		&user.LastName,
		&user.Username,
		&user.LanguageCode,
		&user.IsPremium,
		&user.CreatedAt,
		&user.TotalMessages,
		&user.TotalChats,
		&user.MediaShared,
		&firstActive,
		&lastActive,
		&user.ReactionsGiven,
		&user.ReactionsReceived,
	)
	if err != nil {
		return nil, fmt.Errorf("querying user %d: %w", userID, err)
	}

	if firstActive.Valid {
		user.FirstActive = firstActive.Time
	}
	if lastActive.Valid {
		user.LastActive = lastActive.Time
	}

	return &user, nil
}

// GetUserChats returns all chats the user participates in
func (r *UserRepository) GetUserChats(ctx context.Context, userID int64) ([]UserChat, error) {
	query := `
		SELECT
			c.id,
			COALESCE(c.title, '') as title,
			c.type,
			COALESCE(c.username, '') as username,
			COUNT(m.id) as message_count,
			MIN(m.date) as first_active,
			MAX(m.date) as last_active
		FROM chats c
		INNER JOIN messages m ON m.chat_id = c.id AND m.user_id = $1
		GROUP BY c.id, c.title, c.type, c.username
		ORDER BY message_count DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("querying user chats: %w", err)
	}
	defer rows.Close()

	var chats []UserChat
	for rows.Next() {
		var c UserChat
		if err := rows.Scan(
			&c.ChatID,
			&c.Title,
			&c.Type,
			&c.Username,
			&c.MessageCount,
			&c.FirstActive,
			&c.LastActive,
		); err != nil {
			return nil, fmt.Errorf("scanning user chat: %w", err)
		}
		chats = append(chats, c)
	}

	return chats, rows.Err()
}

// GetUserMessages returns paginated messages for a user using cursor-based pagination
// cursor is the last message ID from the previous page (0 for first page)
// Returns messages ordered by ID descending (newest first)
func (r *UserRepository) GetUserMessages(ctx context.Context, userID int64, cursor int64, pageSize int) ([]UserMessage, error) {
	query := `
		SELECT
			m.id,
			m.chat_id,
			COALESCE(c.title, c.username, CAST(c.id AS TEXT)) as chat_title,
			m.date,
			COALESCE(m.text, m.caption, '') as text,
			EXISTS(SELECT 1 FROM media_files mf WHERE mf.message_id = m.id) as has_media
		FROM messages m
		INNER JOIN chats c ON c.id = m.chat_id
		WHERE m.user_id = $1
	`

	args := []interface{}{userID}
	if cursor > 0 {
		query += " AND m.id < $2"
		args = append(args, cursor)
		query += fmt.Sprintf(" ORDER BY m.id DESC LIMIT $%d", len(args)+1)
		args = append(args, pageSize)
	} else {
		query += " ORDER BY m.id DESC LIMIT $2"
		args = append(args, pageSize)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying user messages: %w", err)
	}
	defer rows.Close()

	var messages []UserMessage
	for rows.Next() {
		var m UserMessage
		if err := rows.Scan(
			&m.ID,
			&m.ChatID,
			&m.ChatTitle,
			&m.Date,
			&m.Text,
			&m.HasMedia,
		); err != nil {
			return nil, fmt.Errorf("scanning user message: %w", err)
		}
		messages = append(messages, m)
	}

	return messages, rows.Err()
}

// GetUserReactionStats returns reaction statistics for a user
func (r *UserRepository) GetUserReactionStats(ctx context.Context, userID int64) (*UserReactionStats, error) {
	stats := &UserReactionStats{}

	// Get total given
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM message_reactions WHERE user_id = $1 AND is_removed = false
	`, userID).Scan(&stats.TotalGiven)
	if err != nil {
		return nil, fmt.Errorf("querying total reactions given: %w", err)
	}

	// Get total received
	err = r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM message_reactions mr
		INNER JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
		WHERE m.user_id = $1 AND mr.is_removed = false
	`, userID).Scan(&stats.TotalReceived)
	if err != nil {
		return nil, fmt.Errorf("querying total reactions received: %w", err)
	}

	// Get top 5 emojis given
	rows, err := r.db.QueryContext(ctx, `
		SELECT emoji_value, COUNT(*) as count
		FROM message_reactions
		WHERE user_id = $1 AND is_removed = false AND emoji_value IS NOT NULL
		GROUP BY emoji_value
		ORDER BY count DESC
		LIMIT 5
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("querying top emojis given: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var ec EmojiCount
		if err := rows.Scan(&ec.Emoji, &ec.Count); err != nil {
			return nil, fmt.Errorf("scanning emoji count: %w", err)
		}
		stats.TopGiven = append(stats.TopGiven, ec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Get top 5 emojis received
	rows2, err := r.db.QueryContext(ctx, `
		SELECT mr.emoji_value, COUNT(*) as count
		FROM message_reactions mr
		INNER JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
		WHERE m.user_id = $1 AND mr.is_removed = false AND mr.emoji_value IS NOT NULL
		GROUP BY mr.emoji_value
		ORDER BY count DESC
		LIMIT 5
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("querying top emojis received: %w", err)
	}
	defer rows2.Close()

	for rows2.Next() {
		var ec EmojiCount
		if err := rows2.Scan(&ec.Emoji, &ec.Count); err != nil {
			return nil, fmt.Errorf("scanning emoji count: %w", err)
		}
		stats.TopReceived = append(stats.TopReceived, ec)
	}

	return stats, rows2.Err()
}

// GetUserActivityDates returns all distinct dates when the user posted messages
func (r *UserRepository) GetUserActivityDates(ctx context.Context, userID int64) ([]time.Time, error) {
	query := `
		SELECT DISTINCT DATE(date) as activity_date
		FROM messages
		WHERE user_id = $1
		ORDER BY activity_date
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("querying activity dates: %w", err)
	}
	defer rows.Close()

	var dates []time.Time
	for rows.Next() {
		var d time.Time
		if err := rows.Scan(&d); err != nil {
			return nil, fmt.Errorf("scanning activity date: %w", err)
		}
		dates = append(dates, d)
	}

	return dates, rows.Err()
}

// ComputeStreaks calculates posting and silence streaks from activity dates
func ComputeStreaks(dates []time.Time) UserStreaks {
	if len(dates) == 0 {
		return UserStreaks{}
	}

	// Sort dates ascending (should already be sorted, but ensure)
	sort.Slice(dates, func(i, j int) bool {
		return dates[i].Before(dates[j])
	})

	streaks := UserStreaks{}
	today := time.Now().UTC().Truncate(24 * time.Hour)

	// Calculate max posting streak (consecutive days with activity)
	currentPostingStreak := 1
	maxPostingStreak := 1

	for i := 1; i < len(dates); i++ {
		daysDiff := int(dates[i].Sub(dates[i-1]).Hours() / 24)
		if daysDiff == 1 {
			currentPostingStreak++
			if currentPostingStreak > maxPostingStreak {
				maxPostingStreak = currentPostingStreak
			}
		} else {
			currentPostingStreak = 1
		}
	}
	streaks.MaxPostingStreak = maxPostingStreak

	// Calculate max silence streak (consecutive days without activity)
	maxSilenceStreak := 0
	for i := 1; i < len(dates); i++ {
		daysDiff := int(dates[i].Sub(dates[i-1]).Hours() / 24)
		gap := daysDiff - 1 // days between the two activity days
		if gap > maxSilenceStreak {
			maxSilenceStreak = gap
		}
	}
	// Also consider gap from last activity to today
	if len(dates) > 0 {
		lastDate := dates[len(dates)-1].Truncate(24 * time.Hour)
		gapToToday := int(today.Sub(lastDate).Hours()/24) - 1
		if gapToToday > maxSilenceStreak {
			maxSilenceStreak = gapToToday
		}
	}
	streaks.MaxSilenceStreak = maxSilenceStreak

	// Calculate current streak (from today backwards)
	lastDate := dates[len(dates)-1].Truncate(24 * time.Hour)
	daysFromToday := int(today.Sub(lastDate).Hours() / 24)

	if daysFromToday <= 1 {
		// User was active today or yesterday - count posting streak
		streaks.IsCurrentlyActive = true
		streaks.CurrentStreak = 1

		for i := len(dates) - 2; i >= 0; i-- {
			daysDiff := int(dates[i+1].Sub(dates[i]).Hours() / 24)
			if daysDiff == 1 {
				streaks.CurrentStreak++
			} else {
				break
			}
		}
	} else {
		// User hasn't been active - count silence streak
		streaks.IsCurrentlyActive = false
		streaks.CurrentStreak = daysFromToday
	}

	return streaks
}

// TruncateText truncates text to maxLen characters, adding ellipsis if truncated
func TruncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	if maxLen <= 3 {
		return text[:maxLen]
	}
	return text[:maxLen-3] + "..."
}
