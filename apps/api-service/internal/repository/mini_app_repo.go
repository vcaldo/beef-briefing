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
	TotalMessages   int64   `json:"total_messages"`
	TotalUsers      int64   `json:"total_users"`
	TotalReactions  int64   `json:"total_reactions"`
	TotalMedia      int64   `json:"total_media"`
	MessagesPerDay  float64 `json:"messages_per_day"`
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
