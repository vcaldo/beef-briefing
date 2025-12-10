package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"beef-briefing/apps/api-service/internal/models"
)

type AnalyticsRepository struct {
	db *sql.DB
}

func NewAnalyticsRepository(db *sql.DB) *AnalyticsRepository {
	return &AnalyticsRepository{db: db}
}

// ============================================
// OVERVIEW QUERIES
// ============================================

func (r *AnalyticsRepository) GetOverviewStats(ctx context.Context, chatID int64, startDate, endDate time.Time) (*models.OverviewResponse, error) {
	query := `
		SELECT
			COUNT(DISTINCT m.id) as total_messages,
			COUNT(DISTINCT m.user_id) as total_users,
			COUNT(DISTINCT mf.id) as total_media
		FROM messages m
		LEFT JOIN media_files mf ON mf.message_id = m.id
		WHERE m.chat_id = $1 AND m.date >= $2 AND m.date < $3
	`

	var overview models.OverviewResponse

	err := r.db.QueryRowContext(ctx, query, chatID, startDate, endDate).Scan(
		&overview.TotalMessages,
		&overview.TotalUsers,
		&overview.TotalMedia,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query overview stats: %w", err)
	}

	// Calculate messages per day
	days := endDate.Sub(startDate).Hours() / 24
	if days > 0 {
		overview.MessagesPerDay = float64(overview.TotalMessages) / days
	}

	// Get total reactions (from message_reactions table)
	reactionsQuery := `
		SELECT COUNT(*)
		FROM message_reactions mr
		WHERE mr.chat_id = $1 AND mr.date >= $2 AND mr.date < $3 AND mr.is_removed = false
	`
	err = r.db.QueryRowContext(ctx, reactionsQuery, chatID, startDate, endDate).Scan(&overview.TotalReactions)
	if err != nil {
		return nil, fmt.Errorf("failed to query reaction count: %w", err)
	}

	return &overview, nil
}

func (r *AnalyticsRepository) GetMostActiveUser(ctx context.Context, chatID int64, startDate, endDate time.Time) (*models.UserSummary, error) {
	query := `
		SELECT u.id, COALESCE(u.username, ''), u.first_name, COALESCE(u.last_name, '')
		FROM messages m
		INNER JOIN users u ON u.id = m.user_id
		WHERE m.chat_id = $1 AND m.date >= $2 AND m.date < $3
		GROUP BY u.id, u.username, u.first_name, u.last_name
		ORDER BY COUNT(*) DESC
		LIMIT 1
	`

	var user models.UserSummary
	err := r.db.QueryRowContext(ctx, query, chatID, startDate, endDate).Scan(
		&user.UserID,
		&user.Username,
		&user.FirstName,
		&user.LastName,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query most active user: %w", err)
	}

	return &user, nil
}

func (r *AnalyticsRepository) GetTopEmojis(ctx context.Context, chatID int64, startDate, endDate time.Time, limit int) ([]models.EmojiBreakdown, error) {
	query := `
		SELECT emoji_value, COUNT(*) as count
		FROM message_reactions
		WHERE chat_id = $1
			AND date >= $2
			AND date < $3
			AND is_removed = false
			AND reaction_type = 'emoji'
			AND emoji_value IS NOT NULL
		GROUP BY emoji_value
		ORDER BY count DESC
		LIMIT $4
	`

	rows, err := r.db.QueryContext(ctx, query, chatID, startDate, endDate, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query top emojis: %w", err)
	}
	defer rows.Close()

	var emojis []models.EmojiBreakdown
	for rows.Next() {
		var e models.EmojiBreakdown
		if err := rows.Scan(&e.Emoji, &e.Count); err != nil {
			return nil, fmt.Errorf("failed to scan emoji: %w", err)
		}
		emojis = append(emojis, e)
	}

	return emojis, rows.Err()
}

func (r *AnalyticsRepository) GetMediaTypeBreakdown(ctx context.Context, chatID int64, startDate, endDate time.Time) (map[string]int, error) {
	query := `
		SELECT mf.media_type, COUNT(*) as count
		FROM media_files mf
		INNER JOIN messages m ON m.id = mf.message_id
		WHERE m.chat_id = $1 AND m.date >= $2 AND m.date < $3
		GROUP BY mf.media_type
	`

	rows, err := r.db.QueryContext(ctx, query, chatID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query media breakdown: %w", err)
	}
	defer rows.Close()

	breakdown := make(map[string]int)
	for rows.Next() {
		var mediaType string
		var count int
		if err := rows.Scan(&mediaType, &count); err != nil {
			return nil, fmt.Errorf("failed to scan media type: %w", err)
		}
		breakdown[mediaType] = count
	}

	return breakdown, rows.Err()
}

// ============================================
// LEADERBOARD QUERIES
// ============================================

func (r *AnalyticsRepository) GetLeaderboard(ctx context.Context, chatID int64, startDate, endDate time.Time, metric string, limit int) ([]models.LeaderboardEntry, error) {
	var query string

	switch metric {
	case "messages":
		query = `
			SELECT u.id, COALESCE(u.username, ''), u.first_name, COALESCE(u.last_name, ''), COUNT(*) as score
			FROM messages m
			INNER JOIN users u ON u.id = m.user_id
			WHERE m.chat_id = $1 AND m.date >= $2 AND m.date < $3
			GROUP BY u.id, u.username, u.first_name, u.last_name
			ORDER BY score DESC
			LIMIT $4
		`
	case "reactions_given":
		query = `
			SELECT u.id, COALESCE(u.username, ''), u.first_name, COALESCE(u.last_name, ''), COUNT(*) as score
			FROM message_reactions mr
			INNER JOIN users u ON u.id = mr.user_id
			WHERE mr.chat_id = $1 AND mr.date >= $2 AND mr.date < $3 AND mr.is_removed = false
			GROUP BY u.id, u.username, u.first_name, u.last_name
			ORDER BY score DESC
			LIMIT $4
		`
	case "reactions_received":
		query = `
			SELECT u.id, COALESCE(u.username, ''), u.first_name, COALESCE(u.last_name, ''), COUNT(*) as score
			FROM message_reactions mr
			INNER JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
			INNER JOIN users u ON u.id = m.user_id
			WHERE mr.chat_id = $1 AND mr.date >= $2 AND mr.date < $3 AND mr.is_removed = false
			GROUP BY u.id, u.username, u.first_name, u.last_name
			ORDER BY score DESC
			LIMIT $4
		`
	case "media_sent":
		query = `
			SELECT u.id, COALESCE(u.username, ''), u.first_name, COALESCE(u.last_name, ''), COUNT(DISTINCT mf.id) as score
			FROM messages m
			INNER JOIN users u ON u.id = m.user_id
			INNER JOIN media_files mf ON mf.message_id = m.id
			WHERE m.chat_id = $1 AND m.date >= $2 AND m.date < $3
			GROUP BY u.id, u.username, u.first_name, u.last_name
			ORDER BY score DESC
			LIMIT $4
		`
	default:
		return nil, fmt.Errorf("invalid metric: %s", metric)
	}

	rows, err := r.db.QueryContext(ctx, query, chatID, startDate, endDate, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query leaderboard: %w", err)
	}
	defer rows.Close()

	var entries []models.LeaderboardEntry
	rank := 1
	for rows.Next() {
		var entry models.LeaderboardEntry
		if err := rows.Scan(
			&entry.UserID,
			&entry.Username,
			&entry.FirstName,
			&entry.LastName,
			&entry.Score,
		); err != nil {
			return nil, fmt.Errorf("failed to scan leaderboard entry: %w", err)
		}
		entry.Rank = rank
		rank++
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// ============================================
// USER DETAIL QUERIES
// ============================================

func (r *AnalyticsRepository) GetUserDetailStats(ctx context.Context, chatID, userID int64, startDate, endDate time.Time) (*models.UserDetailStats, error) {
	query := `
		SELECT
			COUNT(DISTINCT m.id) as total_messages,
			COUNT(DISTINCT mf.id) as media_sent,
			COALESCE(AVG(LENGTH(COALESCE(m.text, '') || COALESCE(m.caption, ''))), 0) as avg_length
		FROM messages m
		LEFT JOIN media_files mf ON mf.message_id = m.id
		WHERE m.chat_id = $1 AND m.user_id = $2 AND m.date >= $3 AND m.date < $4
	`

	var stats models.UserDetailStats
	err := r.db.QueryRowContext(ctx, query, chatID, userID, startDate, endDate).Scan(
		&stats.TotalMessages,
		&stats.MediaSent,
		&stats.AvgMessageLength,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query user stats: %w", err)
	}

	// Get replies received
	repliesQuery := `
		SELECT COUNT(DISTINCT r.id)
		FROM messages m
		INNER JOIN messages r ON r.reply_to_message_id = m.message_id AND r.chat_id = m.chat_id
		WHERE m.chat_id = $1 AND m.user_id = $2 AND m.date >= $3 AND m.date < $4
	`
	err = r.db.QueryRowContext(ctx, repliesQuery, chatID, userID, startDate, endDate).Scan(&stats.RepliesReceived)
	if err != nil {
		return nil, fmt.Errorf("failed to query replies received: %w", err)
	}

	// Get reactions given
	err = r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM message_reactions
		WHERE chat_id = $1 AND user_id = $2 AND date >= $3 AND date < $4 AND is_removed = false
	`, chatID, userID, startDate, endDate).Scan(&stats.ReactionsGiven)
	if err != nil {
		return nil, fmt.Errorf("failed to query reactions given: %w", err)
	}

	// Get reactions received
	err = r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM message_reactions mr
		INNER JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
		WHERE mr.chat_id = $1 AND m.user_id = $2 AND mr.date >= $3 AND mr.date < $4 AND mr.is_removed = false
	`, chatID, userID, startDate, endDate).Scan(&stats.ReactionsReceived)
	if err != nil {
		return nil, fmt.Errorf("failed to query reactions received: %w", err)
	}

	// Calculate media percentage
	if stats.TotalMessages > 0 {
		stats.MediaPercentage = float64(stats.MediaSent) / float64(stats.TotalMessages) * 100
	}

	return &stats, nil
}

// GetUserMessageDays returns all days the user sent messages (for streak calculation)
func (r *AnalyticsRepository) GetUserMessageDays(ctx context.Context, chatID, userID int64, startDate, endDate time.Time) ([]time.Time, error) {
	query := `
		SELECT DISTINCT DATE(date) as day
		FROM messages
		WHERE chat_id = $1 AND user_id = $2 AND date >= $3 AND date < $4
		ORDER BY day
	`

	rows, err := r.db.QueryContext(ctx, query, chatID, userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query message days: %w", err)
	}
	defer rows.Close()

	var days []time.Time
	for rows.Next() {
		var day time.Time
		if err := rows.Scan(&day); err != nil {
			return nil, fmt.Errorf("failed to scan day: %w", err)
		}
		days = append(days, day)
	}

	return days, rows.Err()
}

func (r *AnalyticsRepository) GetUserInfo(ctx context.Context, userID int64) (*models.UserSummary, error) {
	query := `
		SELECT id, COALESCE(username, ''), first_name, COALESCE(last_name, '')
		FROM users
		WHERE id = $1
	`

	var user models.UserSummary
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&user.UserID,
		&user.Username,
		&user.FirstName,
		&user.LastName,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query user info: %w", err)
	}

	return &user, nil
}

func (r *AnalyticsRepository) GetUserTopEmojis(ctx context.Context, chatID, userID int64, startDate, endDate time.Time, limit int, given bool) ([]models.EmojiBreakdown, error) {
	var query string

	if given {
		query = `
			SELECT emoji_value, COUNT(*) as count
			FROM message_reactions
			WHERE chat_id = $1 AND user_id = $2 AND date >= $3 AND date < $4
				AND is_removed = false AND reaction_type = 'emoji' AND emoji_value IS NOT NULL
			GROUP BY emoji_value
			ORDER BY count DESC
			LIMIT $5
		`
	} else {
		query = `
			SELECT mr.emoji_value, COUNT(*) as count
			FROM message_reactions mr
			INNER JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
			WHERE mr.chat_id = $1 AND m.user_id = $2 AND mr.date >= $3 AND mr.date < $4
				AND mr.is_removed = false AND mr.reaction_type = 'emoji' AND mr.emoji_value IS NOT NULL
			GROUP BY mr.emoji_value
			ORDER BY count DESC
			LIMIT $5
		`
	}

	rows, err := r.db.QueryContext(ctx, query, chatID, userID, startDate, endDate, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query user emojis: %w", err)
	}
	defer rows.Close()

	var emojis []models.EmojiBreakdown
	for rows.Next() {
		var e models.EmojiBreakdown
		if err := rows.Scan(&e.Emoji, &e.Count); err != nil {
			return nil, fmt.Errorf("failed to scan emoji: %w", err)
		}
		emojis = append(emojis, e)
	}

	return emojis, rows.Err()
}

func (r *AnalyticsRepository) GetUserActivityByHour(ctx context.Context, chatID, userID int64, startDate, endDate time.Time) (map[int]int, error) {
	query := `
		SELECT EXTRACT(HOUR FROM date)::int as hour, COUNT(*) as count
		FROM messages
		WHERE chat_id = $1 AND user_id = $2 AND date >= $3 AND date < $4
		GROUP BY hour
		ORDER BY hour
	`

	rows, err := r.db.QueryContext(ctx, query, chatID, userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query activity by hour: %w", err)
	}
	defer rows.Close()

	activity := make(map[int]int)
	for rows.Next() {
		var hour, count int
		if err := rows.Scan(&hour, &count); err != nil {
			return nil, fmt.Errorf("failed to scan hour: %w", err)
		}
		activity[hour] = count
	}

	return activity, rows.Err()
}

// ============================================
// TIMELINE QUERIES
// ============================================

func (r *AnalyticsRepository) GetTimeline(ctx context.Context, chatID int64, startDate, endDate time.Time, granularity string) ([]models.TimelinePoint, error) {
	var truncFunc, reactionTruncFunc string
	switch granularity {
	case "hour":
		truncFunc = "date_trunc('hour', m.date)"
		reactionTruncFunc = "date_trunc('hour', date)"
	case "day":
		truncFunc = "date_trunc('day', m.date)"
		reactionTruncFunc = "date_trunc('day', date)"
	case "week":
		truncFunc = "date_trunc('week', m.date)"
		reactionTruncFunc = "date_trunc('week', date)"
	case "month":
		truncFunc = "date_trunc('month', m.date)"
		reactionTruncFunc = "date_trunc('month', date)"
	default:
		return nil, fmt.Errorf("invalid granularity: %s", granularity)
	}

	query := fmt.Sprintf(`
		SELECT
			%s as timestamp,
			COUNT(*) as messages,
			COUNT(DISTINCT m.user_id) as active_users
		FROM messages m
		WHERE m.chat_id = $1 AND m.date >= $2 AND m.date < $3
		GROUP BY timestamp
		ORDER BY timestamp
	`, truncFunc)

	rows, err := r.db.QueryContext(ctx, query, chatID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query timeline: %w", err)
	}
	defer rows.Close()

	var points []models.TimelinePoint
	for rows.Next() {
		var point models.TimelinePoint
		if err := rows.Scan(&point.Timestamp, &point.Messages, &point.Users); err != nil {
			return nil, fmt.Errorf("failed to scan timeline point: %w", err)
		}

		// Get reaction count for this time bucket
		reactionQuery := fmt.Sprintf(`
			SELECT COUNT(*)
			FROM message_reactions
			WHERE chat_id = $1 AND %s = $2 AND is_removed = false
		`, reactionTruncFunc)

		err = r.db.QueryRowContext(ctx, reactionQuery, chatID, point.Timestamp).Scan(&point.Reactions)
		if err != nil {
			return nil, fmt.Errorf("failed to query reactions for timeline: %w", err)
		}

		points = append(points, point)
	}

	return points, rows.Err()
}

// ============================================
// HEATMAP QUERIES
// ============================================

func (r *AnalyticsRepository) GetHeatmapData(ctx context.Context, chatID int64, startDate, endDate time.Time) ([]models.HeatmapDay, error) {
	query := `
		SELECT
			TO_CHAR(date, 'YYYY-MM-DD') as date,
			COUNT(*) as message_count
		FROM messages
		WHERE chat_id = $1 AND date >= $2 AND date < $3
		GROUP BY TO_CHAR(date, 'YYYY-MM-DD')
		ORDER BY date
	`

	rows, err := r.db.QueryContext(ctx, query, chatID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query heatmap data: %w", err)
	}
	defer rows.Close()

	var days []models.HeatmapDay
	maxCount := 0

	// First pass: collect data and find max
	for rows.Next() {
		var day models.HeatmapDay
		if err := rows.Scan(&day.Date, &day.MessageCount); err != nil {
			return nil, fmt.Errorf("failed to scan heatmap day: %w", err)
		}
		if day.MessageCount > maxCount {
			maxCount = day.MessageCount
		}
		days = append(days, day)
	}

	// Second pass: calculate levels (0-4)
	if maxCount > 0 {
		for i := range days {
			days[i].Level = calculateHeatmapLevel(days[i].MessageCount, maxCount)
		}
	}

	return days, rows.Err()
}

// calculateHeatmapLevel returns a level 0-4 based on message count
func calculateHeatmapLevel(count, max int) int {
	if count == 0 {
		return 0
	}
	percentage := float64(count) / float64(max)
	switch {
	case percentage >= 0.75:
		return 4
	case percentage >= 0.50:
		return 3
	case percentage >= 0.25:
		return 2
	default:
		return 1
	}
}

// ============================================
// TOP CONTENT QUERIES
// ============================================

func (r *AnalyticsRepository) GetTopMessages(ctx context.Context, chatID int64, startDate, endDate time.Time, metric string, limit int) ([]models.TopMessage, error) {
	var query string

	if metric == "most_reacted" {
		query = `
			SELECT
				m.id,
				m.message_id as telegram_message_id,
				u.id,
				COALESCE(u.username, ''),
				u.first_name,
				COALESCE(u.last_name, ''),
				m.date,
				COALESCE(m.text, ''),
				COALESCE(m.caption, ''),
				COUNT(mr.id) as score
			FROM messages m
			INNER JOIN users u ON u.id = m.user_id
			INNER JOIN message_reactions mr ON mr.chat_id = m.chat_id AND mr.message_id = m.message_id
			WHERE m.chat_id = $1 AND m.date >= $2 AND m.date < $3 AND mr.is_removed = false
			GROUP BY m.id, m.message_id, u.id, u.username, u.first_name, u.last_name, m.date, m.text, m.caption
			ORDER BY score DESC
			LIMIT $4
		`
	} else if metric == "most_replied" {
		query = `
			SELECT
				m.id,
				m.message_id as telegram_message_id,
				u.id,
				COALESCE(u.username, ''),
				u.first_name,
				COALESCE(u.last_name, ''),
				m.date,
				COALESCE(m.text, ''),
				COALESCE(m.caption, ''),
				COUNT(r.id) as score
			FROM messages m
			INNER JOIN users u ON u.id = m.user_id
			INNER JOIN messages r ON r.reply_to_message_id = m.message_id AND r.chat_id = m.chat_id
			WHERE m.chat_id = $1 AND m.date >= $2 AND m.date < $3
			GROUP BY m.id, m.message_id, u.id, u.username, u.first_name, u.last_name, m.date, m.text, m.caption
			ORDER BY score DESC
			LIMIT $4
		`
	} else {
		return nil, fmt.Errorf("invalid metric: %s", metric)
	}

	rows, err := r.db.QueryContext(ctx, query, chatID, startDate, endDate, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query top messages: %w", err)
	}
	defer rows.Close()

	var messages []models.TopMessage
	for rows.Next() {
		var msg models.TopMessage
		if err := rows.Scan(
			&msg.MessageID,
			&msg.TelegramMessageID,
			&msg.UserID,
			&msg.Username,
			&msg.FirstName,
			&msg.LastName,
			&msg.Date,
			&msg.Text,
			&msg.Caption,
			&msg.Score,
		); err != nil {
			return nil, fmt.Errorf("failed to scan top message: %w", err)
		}
		messages = append(messages, msg)
	}

	return messages, rows.Err()
}

func (r *AnalyticsRepository) GetMessageTopReactions(ctx context.Context, chatID int64, telegramMessageID int64, limit int) ([]models.EmojiBreakdown, error) {
	query := `
		SELECT emoji_value, COUNT(*) as count
		FROM message_reactions
		WHERE chat_id = $1 AND message_id = $2 AND is_removed = false
			AND reaction_type = 'emoji' AND emoji_value IS NOT NULL
		GROUP BY emoji_value
		ORDER BY count DESC
		LIMIT $3
	`

	rows, err := r.db.QueryContext(ctx, query, chatID, telegramMessageID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query message reactions: %w", err)
	}
	defer rows.Close()

	var emojis []models.EmojiBreakdown
	for rows.Next() {
		var e models.EmojiBreakdown
		if err := rows.Scan(&e.Emoji, &e.Count); err != nil {
			return nil, fmt.Errorf("failed to scan emoji: %w", err)
		}
		emojis = append(emojis, e)
	}

	return emojis, rows.Err()
}

// ============================================
// COMPARE QUERIES
// ============================================

func (r *AnalyticsRepository) GetUserComparisons(ctx context.Context, chatID int64, userIDs []int64, startDate, endDate time.Time) ([]models.UserComparison, error) {
	// Build IN clause for user IDs
	placeholders := make([]string, len(userIDs))
	args := []interface{}{chatID, startDate, endDate}
	for i, userID := range userIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+4)
		args = append(args, userID)
	}

	query := fmt.Sprintf(`
		SELECT
			u.id,
			COALESCE(u.username, ''),
			u.first_name,
			COALESCE(u.last_name, ''),
			COUNT(DISTINCT m.id) as messages,
			COUNT(DISTINCT mf.id) as media_sent,
			COALESCE(AVG(LENGTH(COALESCE(m.text, '') || COALESCE(m.caption, ''))), 0) as avg_length
		FROM users u
		LEFT JOIN messages m ON m.user_id = u.id AND m.chat_id = $1 AND m.date >= $2 AND m.date < $3
		LEFT JOIN media_files mf ON mf.message_id = m.id
		WHERE u.id IN (%s)
		GROUP BY u.id, u.username, u.first_name, u.last_name
		ORDER BY messages DESC
	`, strings.Join(placeholders, ","))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query user comparisons: %w", err)
	}
	defer rows.Close()

	var comparisons []models.UserComparison
	for rows.Next() {
		var comp models.UserComparison
		if err := rows.Scan(
			&comp.UserID,
			&comp.Username,
			&comp.FirstName,
			&comp.LastName,
			&comp.Messages,
			&comp.MediaSent,
			&comp.AvgMessageLength,
		); err != nil {
			return nil, fmt.Errorf("failed to scan comparison: %w", err)
		}
		comparisons = append(comparisons, comp)
	}

	// Get reactions for each user (separate queries due to complexity)
	for i := range comparisons {
		err = r.db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM message_reactions
			WHERE chat_id = $1 AND user_id = $2 AND date >= $3 AND date < $4 AND is_removed = false
		`, chatID, comparisons[i].UserID, startDate, endDate).Scan(&comparisons[i].ReactionsGiven)
		if err != nil {
			return nil, fmt.Errorf("failed to query reactions given: %w", err)
		}

		err = r.db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM message_reactions mr
			INNER JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
			WHERE mr.chat_id = $1 AND m.user_id = $2 AND mr.date >= $3 AND mr.date < $4 AND mr.is_removed = false
		`, chatID, comparisons[i].UserID, startDate, endDate).Scan(&comparisons[i].ReactionsReceived)
		if err != nil {
			return nil, fmt.Errorf("failed to query reactions received: %w", err)
		}
	}

	return comparisons, nil
}
