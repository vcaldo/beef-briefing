package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
)

// MiniAppRepository handles database operations for Mini App analytics.
type MiniAppRepository struct {
	db    *sql.DB
	nrApp *newrelic.Application
}

// NewMiniAppRepository creates a new MiniAppRepository.
func NewMiniAppRepository(db *sql.DB, nrApp *newrelic.Application) *MiniAppRepository {
	return &MiniAppRepository{db: db, nrApp: nrApp}
}

// OverviewStats represents overview statistics for a chat.
type OverviewStats struct {
	TotalMessages  int64   `json:"total_messages"`
	TotalUsers     int64   `json:"total_users"`
	TotalReactions int64   `json:"total_reactions"`
	TotalMedia     int64   `json:"total_media"`
	MessagesPerDay float64 `json:"messages_per_day"`
}

// DailyActivity represents a single day's activity.
type DailyActivity struct {
	Date     string `json:"date"`
	Messages int64  `json:"messages"`
	Users    int64  `json:"users"`
}

// UserRanking represents a user's ranking in the leaderboard.
type UserRanking struct {
	Rank      int     `json:"rank"`
	UserID    int64   `json:"user_id"`
	FirstName string  `json:"first_name"`
	LastName  *string `json:"last_name,omitempty"`
	Username  *string `json:"username,omitempty"`
	Score     int64   `json:"score"`
}

// TopReaction represents a reaction emoji and its count.
type TopReaction struct {
	Emoji        string `json:"emoji"`
	ReactionType string `json:"reaction_type"`
	Count        int64  `json:"count"`
}

// ReactionUser represents a user in reaction rankings.
type ReactionUser struct {
	Rank      int     `json:"rank"`
	UserID    int64   `json:"user_id"`
	FirstName string  `json:"first_name"`
	LastName  *string `json:"last_name,omitempty"`
	Username  *string `json:"username,omitempty"`
	Score     int64   `json:"score"`
}

// ProfileStats represents personal stats for a user.
type ProfileStats struct {
	MessageCount            int64 `json:"message_count"`
	ReactionsSent           int64 `json:"reactions_sent"`
	ReactionsReceived       int64 `json:"reactions_received"`
	ActiveDays              int64 `json:"active_days"`
	RankByMessages          int   `json:"rank_by_messages"`
	RankByReactionsReceived int   `json:"rank_by_reactions_received"`
}

// TopInteractor represents a user who interacts with another user.
type TopInteractor struct {
	Rank      int     `json:"rank"`
	UserID    int64   `json:"user_id"`
	FirstName string  `json:"first_name"`
	LastName  *string `json:"last_name,omitempty"`
	Username  *string `json:"username,omitempty"`
	Score     int64   `json:"score"`
	TopEmoji  *string `json:"top_emoji,omitempty"`
}

// HeatmapCell represents a single cell in the heatmap grid.
type HeatmapCell struct {
	DayOfWeek    int   `json:"day_of_week"`
	Hour         int   `json:"hour"`
	MessageCount int64 `json:"message_count"`
	UniqueUsers  int64 `json:"unique_users,omitempty"`
}

// HeatmapData represents heatmap data with metadata.
type HeatmapData struct {
	Data          []HeatmapCell `json:"data"`
	MaxCount      int64         `json:"max_count"`
	TotalMessages int64         `json:"total_messages"`
}

// GetOverviewStats returns overview statistics for a chat.
// If startDate/endDate are nil, uses materialized views for all-time stats.
func (r *MiniAppRepository) GetOverviewStats(ctx context.Context, chatID int64, startDate, endDate *time.Time) (*OverviewStats, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:mini-app-overview-stats")
		defer segment.End()
	}

	stats := &OverviewStats{}

	if startDate == nil && endDate == nil {
		// All-time: use materialized views
		err := r.db.QueryRowContext(ctx, `
			SELECT
				COALESCE(SUM(message_count), 0) as total_messages,
				COUNT(DISTINCT user_id) as total_users
			FROM mv_user_statistics
			WHERE chat_id = $1
		`, chatID).Scan(&stats.TotalMessages, &stats.TotalUsers)
		if err != nil {
			return nil, err
		}

		// Get reactions from MV
		err = r.db.QueryRowContext(ctx, `
			SELECT COALESCE(SUM(count), 0) as total_reactions
			FROM mv_reaction_distribution
			WHERE chat_id = $1
		`, chatID).Scan(&stats.TotalReactions)
		if err != nil {
			return nil, err
		}

		// Get media from MV
		err = r.db.QueryRowContext(ctx, `
			SELECT COALESCE(SUM(count), 0) as total_media
			FROM mv_media_distribution
			WHERE chat_id = $1
		`, chatID).Scan(&stats.TotalMedia)
		if err != nil {
			return nil, err
		}

		// Calculate messages per day
		var activeDays int64
		err = r.db.QueryRowContext(ctx, `
			SELECT COUNT(*) as active_days
			FROM mv_daily_message_stats
			WHERE chat_id = $1
		`, chatID).Scan(&activeDays)
		if err != nil {
			return nil, err
		}

		if activeDays > 0 {
			stats.MessagesPerDay = float64(stats.TotalMessages) / float64(activeDays)
		}
	} else {
		// Date-filtered: use live queries
		err := r.db.QueryRowContext(ctx, `
			SELECT
				COUNT(DISTINCT m.id) as total_messages,
				COUNT(DISTINCT m.user_id) as total_users,
				COUNT(DISTINCT mf.id) as total_media
			FROM messages m
			LEFT JOIN media_files mf ON mf.message_id = m.id
			WHERE m.chat_id = $1
				AND m.date >= $2
				AND m.date < $3
		`, chatID, startDate, endDate).Scan(&stats.TotalMessages, &stats.TotalUsers, &stats.TotalMedia)
		if err != nil {
			return nil, err
		}

		// Reactions with date filter
		err = r.db.QueryRowContext(ctx, `
			SELECT COUNT(*) as total_reactions
			FROM message_reactions
			WHERE chat_id = $1
				AND date >= $2
				AND date < $3
				AND is_removed = false
		`, chatID, startDate, endDate).Scan(&stats.TotalReactions)
		if err != nil {
			return nil, err
		}

		// Calculate messages per day
		if startDate != nil && endDate != nil {
			days := int(endDate.Sub(*startDate).Hours() / 24)
			if days > 0 {
				stats.MessagesPerDay = float64(stats.TotalMessages) / float64(days)
			}
		}
	}

	return stats, nil
}

// GetDailyActivity returns daily message activity for a chat.
func (r *MiniAppRepository) GetDailyActivity(ctx context.Context, chatID int64, startDate, endDate *time.Time) ([]DailyActivity, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:mini-app-daily-activity")
		defer segment.End()
	}

	var rows *sql.Rows
	var err error

	if startDate == nil && endDate == nil {
		// All-time from MV
		rows, err = r.db.QueryContext(ctx, `
			SELECT date, message_count, unique_users
			FROM mv_daily_message_stats
			WHERE chat_id = $1
			ORDER BY date
		`, chatID)
	} else {
		// Date-filtered from MV
		rows, err = r.db.QueryContext(ctx, `
			SELECT date, message_count, unique_users
			FROM mv_daily_message_stats
			WHERE chat_id = $1
				AND date >= $2
				AND date < $3
			ORDER BY date
		`, chatID, startDate, endDate)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var activity []DailyActivity
	for rows.Next() {
		var date time.Time
		var a DailyActivity
		if err := rows.Scan(&date, &a.Messages, &a.Users); err != nil {
			return nil, err
		}
		a.Date = date.Format("2006-01-02")
		activity = append(activity, a)
	}

	return activity, rows.Err()
}

// GetUserRankings returns user rankings for a chat.
func (r *MiniAppRepository) GetUserRankings(ctx context.Context, chatID int64, metric string, limit, offset int, startDate, endDate *time.Time) ([]UserRanking, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:mini-app-user-rankings")
		defer segment.End()
	}

	var rows *sql.Rows
	var err error

	if startDate == nil && endDate == nil {
		// All-time: use materialized view
		// Note: We use a safe column name by validating metric
		query := `
			SELECT
				user_id,
				first_name,
				last_name,
				username,
				` + sanitizeMetricColumn(metric) + ` as score,
				ROW_NUMBER() OVER (ORDER BY ` + sanitizeMetricColumn(metric) + ` DESC) as rank
			FROM mv_user_statistics
			WHERE chat_id = $1
				AND is_bot = false
			ORDER BY ` + sanitizeMetricColumn(metric) + ` DESC
			LIMIT $2 OFFSET $3
		`
		rows, err = r.db.QueryContext(ctx, query, chatID, limit, offset)
	} else {
		// Date-filtered: use live query
		query := `
			WITH user_stats AS (
				SELECT
					m.user_id,
					u.first_name,
					u.last_name,
					u.username,
					u.is_bot,
					COUNT(*) as message_count,
					COUNT(DISTINCT DATE(m.date)) as active_days
				FROM messages m
				JOIN users u ON u.id = m.user_id
				WHERE m.chat_id = $1
					AND m.date >= $2
					AND m.date < $3
				GROUP BY m.user_id, u.first_name, u.last_name, u.username, u.is_bot
			),
			reactions_sent AS (
				SELECT user_id, COUNT(*) as reactions_sent
				FROM message_reactions
				WHERE chat_id = $1
					AND date >= $2
					AND date < $3
					AND is_removed = false
				GROUP BY user_id
			),
			reactions_received AS (
				SELECT m.user_id, COUNT(*) as reactions_received
				FROM message_reactions mr
				JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
				WHERE mr.chat_id = $1
					AND mr.date >= $2
					AND mr.date < $3
					AND mr.is_removed = false
				GROUP BY m.user_id
			)
			SELECT
				us.user_id,
				us.first_name,
				us.last_name,
				us.username,
				COALESCE(` + sanitizeMetricColumnCTE(metric) + `, 0) as score,
				ROW_NUMBER() OVER (ORDER BY COALESCE(` + sanitizeMetricColumnCTE(metric) + `, 0) DESC) as rank
			FROM user_stats us
			LEFT JOIN reactions_sent rs ON rs.user_id = us.user_id
			LEFT JOIN reactions_received rr ON rr.user_id = us.user_id
			WHERE us.is_bot = false
			ORDER BY COALESCE(` + sanitizeMetricColumnCTE(metric) + `, 0) DESC
			LIMIT $4 OFFSET $5
		`
		rows, err = r.db.QueryContext(ctx, query, chatID, startDate, endDate, limit, offset)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rankings []UserRanking
	for rows.Next() {
		var rank int64
		var r UserRanking
		if err := rows.Scan(&r.UserID, &r.FirstName, &r.LastName, &r.Username, &r.Score, &rank); err != nil {
			return nil, err
		}
		// Adjust rank for offset
		r.Rank = offset + len(rankings) + 1
		rankings = append(rankings, r)
	}

	return rankings, rows.Err()
}

// GetUserRankingsTotal returns the total count of users for pagination.
func (r *MiniAppRepository) GetUserRankingsTotal(ctx context.Context, chatID int64, startDate, endDate *time.Time) (int, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:mini-app-user-rankings-total")
		defer segment.End()
	}

	var total int

	if startDate == nil && endDate == nil {
		err := r.db.QueryRowContext(ctx, `
			SELECT COUNT(*) as total
			FROM mv_user_statistics
			WHERE chat_id = $1
				AND is_bot = false
		`, chatID).Scan(&total)
		if err != nil {
			return 0, err
		}
	} else {
		err := r.db.QueryRowContext(ctx, `
			SELECT COUNT(DISTINCT m.user_id) as total
			FROM messages m
			JOIN users u ON u.id = m.user_id
			WHERE m.chat_id = $1
				AND m.date >= $2
				AND m.date < $3
				AND u.is_bot = false
		`, chatID, startDate, endDate).Scan(&total)
		if err != nil {
			return 0, err
		}
	}

	return total, nil
}

// sanitizeMetricColumn ensures the metric column name is safe for SQL
func sanitizeMetricColumn(metric string) string {
	switch metric {
	case "message_count":
		return "message_count"
	case "reactions_sent":
		return "reactions_sent"
	case "reactions_received":
		return "reactions_received"
	case "active_days":
		return "active_days"
	default:
		return "message_count"
	}
}

// sanitizeMetricColumnCTE returns the column reference for CTE queries
func sanitizeMetricColumnCTE(metric string) string {
	switch metric {
	case "message_count":
		return "us.message_count"
	case "reactions_sent":
		return "rs.reactions_sent"
	case "reactions_received":
		return "rr.reactions_received"
	case "active_days":
		return "us.active_days"
	default:
		return "us.message_count"
	}
}

// GetTopReactions returns the top reactions used in a chat.
func (r *MiniAppRepository) GetTopReactions(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]TopReaction, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:mini-app-top-reactions")
		defer segment.End()
	}

	var rows *sql.Rows
	var err error

	if startDate == nil && endDate == nil {
		// All-time: use materialized view
		rows, err = r.db.QueryContext(ctx, `
			SELECT emoji, reaction_type, count
			FROM mv_reaction_distribution
			WHERE chat_id = $1
			ORDER BY count DESC
			LIMIT $2
		`, chatID, limit)
	} else {
		// Date-filtered: use live query
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				COALESCE(mr.emoji_value, mr.custom_emoji_id, 'paid') as emoji,
				mr.reaction_type,
				COUNT(*) as count
			FROM message_reactions mr
			WHERE mr.chat_id = $1
				AND mr.is_removed = false
				AND mr.date >= $2
				AND mr.date < $3
			GROUP BY COALESCE(mr.emoji_value, mr.custom_emoji_id, 'paid'), mr.reaction_type
			ORDER BY count DESC
			LIMIT $4
		`, chatID, startDate, endDate, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reactions []TopReaction
	for rows.Next() {
		var r TopReaction
		if err := rows.Scan(&r.Emoji, &r.ReactionType, &r.Count); err != nil {
			return nil, err
		}
		reactions = append(reactions, r)
	}

	return reactions, rows.Err()
}

// GetTopReactionGivers returns users who give the most reactions.
func (r *MiniAppRepository) GetTopReactionGivers(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]ReactionUser, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:mini-app-top-reaction-givers")
		defer segment.End()
	}

	var rows *sql.Rows
	var err error

	if startDate == nil && endDate == nil {
		// All-time: use materialized view
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				user_id,
				first_name,
				last_name,
				username,
				reactions_sent as score,
				ROW_NUMBER() OVER (ORDER BY reactions_sent DESC) as rank
			FROM mv_user_statistics
			WHERE chat_id = $1
				AND is_bot = false
				AND reactions_sent > 0
			ORDER BY reactions_sent DESC
			LIMIT $2
		`, chatID, limit)
	} else {
		// Date-filtered: use live query
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				mr.user_id,
				u.first_name,
				u.last_name,
				u.username,
				COUNT(*) as score,
				ROW_NUMBER() OVER (ORDER BY COUNT(*) DESC) as rank
			FROM message_reactions mr
			JOIN users u ON u.id = mr.user_id
			WHERE mr.chat_id = $1
				AND mr.is_removed = false
				AND mr.user_id IS NOT NULL
				AND mr.date >= $2
				AND mr.date < $3
				AND u.is_bot = false
			GROUP BY mr.user_id, u.first_name, u.last_name, u.username
			ORDER BY score DESC
			LIMIT $4
		`, chatID, startDate, endDate, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []ReactionUser
	for rows.Next() {
		var rank int64
		var u ReactionUser
		if err := rows.Scan(&u.UserID, &u.FirstName, &u.LastName, &u.Username, &u.Score, &rank); err != nil {
			return nil, err
		}
		u.Rank = int(rank)
		users = append(users, u)
	}

	return users, rows.Err()
}

// GetTopReactionReceivers returns users who receive the most reactions.
func (r *MiniAppRepository) GetTopReactionReceivers(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]ReactionUser, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:mini-app-top-reaction-receivers")
		defer segment.End()
	}

	var rows *sql.Rows
	var err error

	if startDate == nil && endDate == nil {
		// All-time: use materialized view
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				user_id,
				first_name,
				last_name,
				username,
				reactions_received as score,
				ROW_NUMBER() OVER (ORDER BY reactions_received DESC) as rank
			FROM mv_user_statistics
			WHERE chat_id = $1
				AND is_bot = false
				AND reactions_received > 0
			ORDER BY reactions_received DESC
			LIMIT $2
		`, chatID, limit)
	} else {
		// Date-filtered: use live query
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				m.user_id,
				u.first_name,
				u.last_name,
				u.username,
				COUNT(*) as score,
				ROW_NUMBER() OVER (ORDER BY COUNT(*) DESC) as rank
			FROM message_reactions mr
			JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
			JOIN users u ON u.id = m.user_id
			WHERE mr.chat_id = $1
				AND mr.is_removed = false
				AND m.user_id IS NOT NULL
				AND mr.date >= $2
				AND mr.date < $3
				AND u.is_bot = false
			GROUP BY m.user_id, u.first_name, u.last_name, u.username
			ORDER BY score DESC
			LIMIT $4
		`, chatID, startDate, endDate, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []ReactionUser
	for rows.Next() {
		var rank int64
		var u ReactionUser
		if err := rows.Scan(&u.UserID, &u.FirstName, &u.LastName, &u.Username, &u.Score, &rank); err != nil {
			return nil, err
		}
		u.Rank = int(rank)
		users = append(users, u)
	}

	return users, rows.Err()
}

// GetUserProfileStats returns personal stats for a user including their rankings.
func (r *MiniAppRepository) GetUserProfileStats(ctx context.Context, chatID, userID int64, startDate, endDate *time.Time) (*ProfileStats, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:mini-app-user-profile-stats")
		defer segment.End()
	}

	stats := &ProfileStats{}

	if startDate == nil && endDate == nil {
		// All-time: use materialized view with window functions for ranking
		err := r.db.QueryRowContext(ctx, `
			WITH ranked AS (
				SELECT
					user_id,
					message_count,
					reactions_sent,
					reactions_received,
					active_days,
					ROW_NUMBER() OVER (ORDER BY message_count DESC) as rank_by_messages,
					ROW_NUMBER() OVER (ORDER BY reactions_received DESC) as rank_by_reactions
				FROM mv_user_statistics
				WHERE chat_id = $1 AND is_bot = false
			)
			SELECT
				message_count,
				reactions_sent,
				reactions_received,
				active_days,
				rank_by_messages,
				rank_by_reactions
			FROM ranked
			WHERE user_id = $2
		`, chatID, userID).Scan(
			&stats.MessageCount,
			&stats.ReactionsSent,
			&stats.ReactionsReceived,
			&stats.ActiveDays,
			&stats.RankByMessages,
			&stats.RankByReactionsReceived,
		)
		if err == sql.ErrNoRows {
			// User not found in stats, return zeros
			return stats, nil
		}
		if err != nil {
			return nil, err
		}
	} else {
		// Date-filtered: use live query
		err := r.db.QueryRowContext(ctx, `
			WITH user_stats AS (
				SELECT
					m.user_id,
					COUNT(*) as message_count,
					COUNT(DISTINCT DATE(m.date)) as active_days
				FROM messages m
				JOIN users u ON u.id = m.user_id
				WHERE m.chat_id = $1
					AND m.date >= $2
					AND m.date < $3
					AND u.is_bot = false
				GROUP BY m.user_id
			),
			reactions_sent AS (
				SELECT user_id, COUNT(*) as reactions_sent
				FROM message_reactions
				WHERE chat_id = $1
					AND date >= $2
					AND date < $3
					AND is_removed = false
				GROUP BY user_id
			),
			reactions_received AS (
				SELECT m.user_id, COUNT(*) as reactions_received
				FROM message_reactions mr
				JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
				WHERE mr.chat_id = $1
					AND mr.date >= $2
					AND mr.date < $3
					AND mr.is_removed = false
				GROUP BY m.user_id
			),
			combined AS (
				SELECT
					us.user_id,
					us.message_count,
					us.active_days,
					COALESCE(rs.reactions_sent, 0) as reactions_sent,
					COALESCE(rr.reactions_received, 0) as reactions_received,
					ROW_NUMBER() OVER (ORDER BY us.message_count DESC) as rank_by_messages,
					ROW_NUMBER() OVER (ORDER BY COALESCE(rr.reactions_received, 0) DESC) as rank_by_reactions
				FROM user_stats us
				LEFT JOIN reactions_sent rs ON rs.user_id = us.user_id
				LEFT JOIN reactions_received rr ON rr.user_id = us.user_id
			)
			SELECT
				message_count,
				reactions_sent,
				reactions_received,
				active_days,
				rank_by_messages,
				rank_by_reactions
			FROM combined
			WHERE user_id = $4
		`, chatID, startDate, endDate, userID).Scan(
			&stats.MessageCount,
			&stats.ReactionsSent,
			&stats.ReactionsReceived,
			&stats.ActiveDays,
			&stats.RankByMessages,
			&stats.RankByReactionsReceived,
		)
		if err == sql.ErrNoRows {
			return stats, nil
		}
		if err != nil {
			return nil, err
		}
	}

	return stats, nil
}

// GetTopReactorsToUser returns users who react most to a specific user's messages.
func (r *MiniAppRepository) GetTopReactorsToUser(ctx context.Context, chatID, userID int64, limit int, startDate, endDate *time.Time) ([]TopInteractor, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:mini-app-top-reactors-to-user")
		defer segment.End()
	}

	var rows *sql.Rows
	var err error

	if startDate == nil && endDate == nil {
		// All-time: live query (no MV for this specific data)
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				mr.user_id,
				u.first_name,
				u.last_name,
				u.username,
				COUNT(*) as score,
				MODE() WITHIN GROUP (ORDER BY COALESCE(mr.emoji_value, mr.custom_emoji_id, 'paid')) as top_emoji,
				ROW_NUMBER() OVER (ORDER BY COUNT(*) DESC) as rank
			FROM message_reactions mr
			JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
			JOIN users u ON u.id = mr.user_id
			WHERE mr.chat_id = $1
				AND m.user_id = $2
				AND mr.user_id != $2
				AND mr.is_removed = false
				AND u.is_bot = false
			GROUP BY mr.user_id, u.first_name, u.last_name, u.username
			ORDER BY score DESC
			LIMIT $3
		`, chatID, userID, limit)
	} else {
		// Date-filtered
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				mr.user_id,
				u.first_name,
				u.last_name,
				u.username,
				COUNT(*) as score,
				MODE() WITHIN GROUP (ORDER BY COALESCE(mr.emoji_value, mr.custom_emoji_id, 'paid')) as top_emoji,
				ROW_NUMBER() OVER (ORDER BY COUNT(*) DESC) as rank
			FROM message_reactions mr
			JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
			JOIN users u ON u.id = mr.user_id
			WHERE mr.chat_id = $1
				AND m.user_id = $2
				AND mr.user_id != $2
				AND mr.is_removed = false
				AND mr.date >= $3
				AND mr.date < $4
				AND u.is_bot = false
			GROUP BY mr.user_id, u.first_name, u.last_name, u.username
			ORDER BY score DESC
			LIMIT $5
		`, chatID, userID, startDate, endDate, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var interactors []TopInteractor
	for rows.Next() {
		var rank int64
		var i TopInteractor
		if err := rows.Scan(&i.UserID, &i.FirstName, &i.LastName, &i.Username, &i.Score, &i.TopEmoji, &rank); err != nil {
			return nil, err
		}
		i.Rank = int(rank)
		interactors = append(interactors, i)
	}

	return interactors, rows.Err()
}

// GetTopRepliersToUser returns users who reply most to a specific user's messages.
func (r *MiniAppRepository) GetTopRepliersToUser(ctx context.Context, chatID, userID int64, limit int, startDate, endDate *time.Time) ([]TopInteractor, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:mini-app-top-repliers-to-user")
		defer segment.End()
	}

	var rows *sql.Rows
	var err error

	if startDate == nil && endDate == nil {
		// All-time
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				m.user_id,
				u.first_name,
				u.last_name,
				u.username,
				COUNT(*) as score,
				ROW_NUMBER() OVER (ORDER BY COUNT(*) DESC) as rank
			FROM messages m
			JOIN messages orig ON orig.chat_id = m.chat_id AND orig.message_id = m.reply_to_message_id
			JOIN users u ON u.id = m.user_id
			WHERE m.chat_id = $1
				AND orig.user_id = $2
				AND m.user_id != $2
				AND m.reply_to_message_id IS NOT NULL
				AND u.is_bot = false
			GROUP BY m.user_id, u.first_name, u.last_name, u.username
			ORDER BY score DESC
			LIMIT $3
		`, chatID, userID, limit)
	} else {
		// Date-filtered
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				m.user_id,
				u.first_name,
				u.last_name,
				u.username,
				COUNT(*) as score,
				ROW_NUMBER() OVER (ORDER BY COUNT(*) DESC) as rank
			FROM messages m
			JOIN messages orig ON orig.chat_id = m.chat_id AND orig.message_id = m.reply_to_message_id
			JOIN users u ON u.id = m.user_id
			WHERE m.chat_id = $1
				AND orig.user_id = $2
				AND m.user_id != $2
				AND m.reply_to_message_id IS NOT NULL
				AND m.date >= $3
				AND m.date < $4
				AND u.is_bot = false
			GROUP BY m.user_id, u.first_name, u.last_name, u.username
			ORDER BY score DESC
			LIMIT $5
		`, chatID, userID, startDate, endDate, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var interactors []TopInteractor
	for rows.Next() {
		var rank int64
		var i TopInteractor
		if err := rows.Scan(&i.UserID, &i.FirstName, &i.LastName, &i.Username, &i.Score, &rank); err != nil {
			return nil, err
		}
		i.Rank = int(rank)
		interactors = append(interactors, i)
	}

	return interactors, rows.Err()
}

// GetTopRepliedToByUser returns users that a specific user replies to most.
func (r *MiniAppRepository) GetTopRepliedToByUser(ctx context.Context, chatID, userID int64, limit int, startDate, endDate *time.Time) ([]TopInteractor, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:mini-app-top-replied-to-by-user")
		defer segment.End()
	}

	var rows *sql.Rows
	var err error

	if startDate == nil && endDate == nil {
		// All-time
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				orig.user_id,
				u.first_name,
				u.last_name,
				u.username,
				COUNT(*) as score,
				ROW_NUMBER() OVER (ORDER BY COUNT(*) DESC) as rank
			FROM messages m
			JOIN messages orig ON orig.chat_id = m.chat_id AND orig.message_id = m.reply_to_message_id
			JOIN users u ON u.id = orig.user_id
			WHERE m.chat_id = $1
				AND m.user_id = $2
				AND orig.user_id != $2
				AND m.reply_to_message_id IS NOT NULL
				AND u.is_bot = false
			GROUP BY orig.user_id, u.first_name, u.last_name, u.username
			ORDER BY score DESC
			LIMIT $3
		`, chatID, userID, limit)
	} else {
		// Date-filtered
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				orig.user_id,
				u.first_name,
				u.last_name,
				u.username,
				COUNT(*) as score,
				ROW_NUMBER() OVER (ORDER BY COUNT(*) DESC) as rank
			FROM messages m
			JOIN messages orig ON orig.chat_id = m.chat_id AND orig.message_id = m.reply_to_message_id
			JOIN users u ON u.id = orig.user_id
			WHERE m.chat_id = $1
				AND m.user_id = $2
				AND orig.user_id != $2
				AND m.reply_to_message_id IS NOT NULL
				AND m.date >= $3
				AND m.date < $4
				AND u.is_bot = false
			GROUP BY orig.user_id, u.first_name, u.last_name, u.username
			ORDER BY score DESC
			LIMIT $5
		`, chatID, userID, startDate, endDate, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var interactors []TopInteractor
	for rows.Next() {
		var rank int64
		var i TopInteractor
		if err := rows.Scan(&i.UserID, &i.FirstName, &i.LastName, &i.Username, &i.Score, &rank); err != nil {
			return nil, err
		}
		i.Rank = int(rank)
		interactors = append(interactors, i)
	}

	return interactors, rows.Err()
}

// GetGroupHeatmap returns the activity heatmap for a chat from the materialized view.
func (r *MiniAppRepository) GetGroupHeatmap(ctx context.Context, chatID int64) (*HeatmapData, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:mini-app-group-heatmap")
		defer segment.End()
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT day_of_week, hour, message_count, unique_users
		FROM mv_hourly_activity
		WHERE chat_id = $1
		ORDER BY day_of_week, hour
	`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	heatmap := &HeatmapData{
		Data: []HeatmapCell{},
	}

	for rows.Next() {
		var cell HeatmapCell
		if err := rows.Scan(&cell.DayOfWeek, &cell.Hour, &cell.MessageCount, &cell.UniqueUsers); err != nil {
			return nil, err
		}
		heatmap.Data = append(heatmap.Data, cell)
		heatmap.TotalMessages += cell.MessageCount
		if cell.MessageCount > heatmap.MaxCount {
			heatmap.MaxCount = cell.MessageCount
		}
	}

	return heatmap, rows.Err()
}

// GetUserHeatmap returns the activity heatmap for a specific user.
func (r *MiniAppRepository) GetUserHeatmap(ctx context.Context, chatID, userID int64, startDate, endDate *time.Time) (*HeatmapData, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:mini-app-user-heatmap")
		defer segment.End()
	}

	var rows *sql.Rows
	var err error

	if startDate == nil && endDate == nil {
		// All-time
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				EXTRACT(DOW FROM m.date)::int as day_of_week,
				EXTRACT(HOUR FROM m.date)::int as hour,
				COUNT(*) as message_count
			FROM messages m
			WHERE m.chat_id = $1 AND m.user_id = $2
			GROUP BY EXTRACT(DOW FROM m.date), EXTRACT(HOUR FROM m.date)
			ORDER BY day_of_week, hour
		`, chatID, userID)
	} else {
		// Date-filtered
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				EXTRACT(DOW FROM m.date)::int as day_of_week,
				EXTRACT(HOUR FROM m.date)::int as hour,
				COUNT(*) as message_count
			FROM messages m
			WHERE m.chat_id = $1
				AND m.user_id = $2
				AND m.date >= $3
				AND m.date < $4
			GROUP BY EXTRACT(DOW FROM m.date), EXTRACT(HOUR FROM m.date)
			ORDER BY day_of_week, hour
		`, chatID, userID, startDate, endDate)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	heatmap := &HeatmapData{
		Data: []HeatmapCell{},
	}

	for rows.Next() {
		var cell HeatmapCell
		if err := rows.Scan(&cell.DayOfWeek, &cell.Hour, &cell.MessageCount); err != nil {
			return nil, err
		}
		heatmap.Data = append(heatmap.Data, cell)
		heatmap.TotalMessages += cell.MessageCount
		if cell.MessageCount > heatmap.MaxCount {
			heatmap.MaxCount = cell.MessageCount
		}
	}

	return heatmap, rows.Err()
}
