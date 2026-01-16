package repository

import (
	"context"
	"database/sql"
	"time"

	"beef-briefing/apps/api-service/internal/nrutil"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// StatsRepository handles statistics-related database queries for Mini App.
type StatsRepository struct {
	db    *sql.DB
	nrApp *newrelic.Application
}

// NewStatsRepository creates a new StatsRepository.
func NewStatsRepository(db *sql.DB, nrApp *newrelic.Application) *StatsRepository {
	return &StatsRepository{db: db, nrApp: nrApp}
}

// GetOverviewStats returns overview statistics for a chat.
// If startDate/endDate are nil, uses materialized views for all-time stats.
func (r *StatsRepository) GetOverviewStats(ctx context.Context, chatID int64, startDate, endDate *time.Time) (*OverviewStats, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-overview-stats")()

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

// GetDailyActivity returns daily message activity for a chat in the specified timezone.
func (r *StatsRepository) GetDailyActivity(ctx context.Context, chatID int64, startDate, endDate *time.Time, tz *time.Location) ([]DailyActivity, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-daily-activity")()

	tzName := "UTC"
	if tz != nil {
		tzName = tz.String()
	}

	var rows *sql.Rows
	var err error

	if startDate == nil && endDate == nil {
		// All-time: query live data with timezone conversion
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				DATE(m.date AT TIME ZONE $2) as activity_date,
				COUNT(*) as message_count,
				COUNT(DISTINCT m.user_id) as unique_users
			FROM messages m
			WHERE m.chat_id = $1
			GROUP BY DATE(m.date AT TIME ZONE $2)
			ORDER BY activity_date
		`, chatID, tzName)
	} else {
		// Date-filtered: query live data with timezone conversion
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				DATE(m.date AT TIME ZONE $4) as activity_date,
				COUNT(*) as message_count,
				COUNT(DISTINCT m.user_id) as unique_users
			FROM messages m
			WHERE m.chat_id = $1
				AND m.date >= $2
				AND m.date < $3
			GROUP BY DATE(m.date AT TIME ZONE $4)
			ORDER BY activity_date
		`, chatID, startDate, endDate, tzName)
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

// GetUserDailyActivity returns daily message activity for a specific user in a chat.
func (r *StatsRepository) GetUserDailyActivity(ctx context.Context, chatID, userID int64, startDate, endDate *time.Time, tz *time.Location) ([]DailyActivity, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-user-daily-activity")()

	tzName := "UTC"
	if tz != nil {
		tzName = tz.String()
	}

	var rows *sql.Rows
	var err error

	if startDate == nil && endDate == nil {
		// All-time: query live data with timezone conversion
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				DATE(m.date AT TIME ZONE $3) as activity_date,
				COUNT(*) as message_count
			FROM messages m
			WHERE m.chat_id = $1 AND m.user_id = $2
			GROUP BY DATE(m.date AT TIME ZONE $3)
			ORDER BY activity_date
		`, chatID, userID, tzName)
	} else {
		// Date-filtered: query live data with timezone conversion
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				DATE(m.date AT TIME ZONE $5) as activity_date,
				COUNT(*) as message_count
			FROM messages m
			WHERE m.chat_id = $1 AND m.user_id = $2
				AND m.date >= $3
				AND m.date < $4
			GROUP BY DATE(m.date AT TIME ZONE $5)
			ORDER BY activity_date
		`, chatID, userID, startDate, endDate, tzName)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var activity []DailyActivity
	for rows.Next() {
		var date time.Time
		var a DailyActivity
		if err := rows.Scan(&date, &a.Messages); err != nil {
			return nil, err
		}
		a.Date = date.Format("2006-01-02")
		a.Users = 1 // Single user
		activity = append(activity, a)
	}

	return activity, rows.Err()
}

// GetUserProfileStats returns personal stats for a user including their rankings.
// tzName is the IANA timezone name for streak calculations (e.g., "America/Sao_Paulo").
// If empty, defaults to UTC.
func (r *StatsRepository) GetUserProfileStats(ctx context.Context, chatID, userID int64, startDate, endDate *time.Time, tzName string) (*ProfileStats, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-user-profile-stats")()

	stats := &ProfileStats{}

	if startDate == nil && endDate == nil {
		// All-time: use materialized view with window functions for ranking
		err := r.db.QueryRowContext(ctx, `
			WITH replies_sent AS (
				SELECT m.user_id, COUNT(*) as replies_sent
				FROM messages m
				WHERE m.chat_id = $1
					AND m.reply_to_message_id IS NOT NULL
				GROUP BY m.user_id
			),
			replies_received AS (
				SELECT orig.user_id, COUNT(*) as replies_received
				FROM messages m
				JOIN messages orig ON orig.chat_id = m.chat_id AND orig.message_id = m.reply_to_message_id
				WHERE m.chat_id = $1
					AND m.reply_to_message_id IS NOT NULL
				GROUP BY orig.user_id
			),
			ranked AS (
				SELECT
					mv.user_id,
					mv.message_count,
					mv.reactions_sent,
					mv.reactions_received,
					mv.active_days,
					COALESCE(rps.replies_sent, 0) as replies_sent,
					COALESCE(rpr.replies_received, 0) as replies_received,
					ROW_NUMBER() OVER (ORDER BY mv.message_count DESC) as rank_by_messages,
					ROW_NUMBER() OVER (ORDER BY mv.reactions_received DESC) as rank_by_reactions
				FROM mv_user_statistics mv
				LEFT JOIN replies_sent rps ON rps.user_id = mv.user_id
				LEFT JOIN replies_received rpr ON rpr.user_id = mv.user_id
				WHERE mv.chat_id = $1 AND mv.is_bot = false
			)
			SELECT
				message_count,
				reactions_sent,
				reactions_received,
				replies_sent,
				replies_received,
				active_days,
				rank_by_messages,
				rank_by_reactions
			FROM ranked
			WHERE user_id = $2
		`, chatID, userID).Scan(
			&stats.MessageCount,
			&stats.ReactionsSent,
			&stats.ReactionsReceived,
			&stats.RepliesSent,
			&stats.RepliesReceived,
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
					COUNT(DISTINCT DATE(m.date AT TIME ZONE $5)) as active_days
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
			replies_sent AS (
				SELECT m.user_id, COUNT(*) as replies_sent
				FROM messages m
				WHERE m.chat_id = $1
					AND m.date >= $2
					AND m.date < $3
					AND m.reply_to_message_id IS NOT NULL
				GROUP BY m.user_id
			),
			replies_received AS (
				SELECT orig.user_id, COUNT(*) as replies_received
				FROM messages m
				JOIN messages orig ON orig.chat_id = m.chat_id AND orig.message_id = m.reply_to_message_id
				WHERE m.chat_id = $1
					AND m.date >= $2
					AND m.date < $3
					AND m.reply_to_message_id IS NOT NULL
				GROUP BY orig.user_id
			),
			combined AS (
				SELECT
					us.user_id,
					us.message_count,
					us.active_days,
					COALESCE(rs.reactions_sent, 0) as reactions_sent,
					COALESCE(rr.reactions_received, 0) as reactions_received,
					COALESCE(rps.replies_sent, 0) as replies_sent,
					COALESCE(rpr.replies_received, 0) as replies_received,
					ROW_NUMBER() OVER (ORDER BY us.message_count DESC) as rank_by_messages,
					ROW_NUMBER() OVER (ORDER BY COALESCE(rr.reactions_received, 0) DESC) as rank_by_reactions
				FROM user_stats us
				LEFT JOIN reactions_sent rs ON rs.user_id = us.user_id
				LEFT JOIN reactions_received rr ON rr.user_id = us.user_id
				LEFT JOIN replies_sent rps ON rps.user_id = us.user_id
				LEFT JOIN replies_received rpr ON rpr.user_id = us.user_id
			)
			SELECT
				message_count,
				reactions_sent,
				reactions_received,
				replies_sent,
				replies_received,
				active_days,
				rank_by_messages,
				rank_by_reactions
			FROM combined
			WHERE user_id = $4
		`, chatID, startDate, endDate, userID, tzName).Scan(
			&stats.MessageCount,
			&stats.ReactionsSent,
			&stats.ReactionsReceived,
			&stats.RepliesSent,
			&stats.RepliesReceived,
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

	// Calculate avg messages per day
	if stats.ActiveDays > 0 {
		stats.AvgMessagesPerDay = float64(stats.MessageCount) / float64(stats.ActiveDays)
	}

	// Calculate current streak
	streak, err := r.getUserCurrentStreak(ctx, chatID, userID, startDate, endDate, tzName)
	if err != nil {
		// Log but don't fail - streak is not critical
		stats.CurrentStreak = 0
	} else {
		stats.CurrentStreak = streak
	}

	return stats, nil
}

// getUserCurrentStreak calculates the current consecutive days streak for a user.
// tzName is the IANA timezone name for streak calculations (e.g., "America/Sao_Paulo").
// If empty, defaults to UTC.
func (r *StatsRepository) getUserCurrentStreak(ctx context.Context, chatID, userID int64, startDate, endDate *time.Time, tzName string) (int64, error) {
	// Default to UTC if no timezone specified
	if tzName == "" {
		tzName = "UTC"
	}

	var query string
	var args []interface{}

	if startDate == nil && endDate == nil {
		// All-time streak: consecutive days ending today or yesterday (active streak only, timezone-aware)
		query = `
			WITH user_dates AS (
				SELECT DISTINCT DATE(date AT TIME ZONE $3) as activity_date
				FROM messages
				WHERE chat_id = $1 AND user_id = $2
			),
			most_recent AS (
				SELECT MAX(activity_date) as last_active FROM user_dates
			),
			numbered AS (
				SELECT
					activity_date,
					activity_date - (ROW_NUMBER() OVER (ORDER BY activity_date ASC))::int as grp
				FROM user_dates
			)
			SELECT CASE
				-- Only count streak if last activity was today or yesterday (in specified timezone)
				WHEN (SELECT last_active FROM most_recent) >= (CURRENT_TIMESTAMP AT TIME ZONE $3)::date - INTERVAL '1 day'
				THEN (
					SELECT COUNT(*)
					FROM numbered
					WHERE grp = (SELECT grp FROM numbered ORDER BY activity_date DESC LIMIT 1)
				)
				ELSE 0
			END
		`
		args = []interface{}{chatID, userID, tzName}
	} else {
		// Period-filtered streak: consecutive days ending at period end or yesterday (timezone-aware)
		query = `
			WITH user_dates AS (
				SELECT DISTINCT DATE(date AT TIME ZONE $5) as activity_date
				FROM messages
				WHERE chat_id = $1 AND user_id = $2
					AND date >= $3 AND date < $4
			),
			most_recent AS (
				SELECT MAX(activity_date) as last_active FROM user_dates
			),
			numbered AS (
				SELECT
					activity_date,
					activity_date - (ROW_NUMBER() OVER (ORDER BY activity_date ASC))::int as grp
				FROM user_dates
			)
			SELECT CASE
				-- Only count streak if last activity was within the period's end or yesterday (in specified timezone)
				WHEN (SELECT last_active FROM most_recent) >= LEAST($4::date, (CURRENT_TIMESTAMP AT TIME ZONE $5)::date) - INTERVAL '1 day'
				THEN (
					SELECT COUNT(*)
					FROM numbered
					WHERE grp = (SELECT grp FROM numbered ORDER BY activity_date DESC LIMIT 1)
				)
				ELSE 0
			END
		`
		args = []interface{}{chatID, userID, startDate, endDate, tzName}
	}

	var streak int64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&streak)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return streak, err
}

// GetMediaOverviewStats returns aggregate media statistics for a chat.
func (r *StatsRepository) GetMediaOverviewStats(ctx context.Context, chatID int64, startDate, endDate *time.Time) (*MediaOverviewStats, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-media-overview-stats")()

	stats := &MediaOverviewStats{}

	if startDate == nil && endDate == nil {
		// All-time: use materialized view
		err := r.db.QueryRowContext(ctx, `
			SELECT
				COALESCE(SUM(count), 0) as total_media,
				COALESCE(SUM(CASE WHEN media_type = 'photo' THEN count ELSE 0 END), 0) as total_photos,
				COALESCE(SUM(CASE WHEN media_type IN ('video', 'video_note') THEN count ELSE 0 END), 0) as total_videos,
				COALESCE(SUM(CASE WHEN media_type = 'animation' THEN count ELSE 0 END), 0) as total_gifs,
				COALESCE(SUM(CASE WHEN media_type = 'voice' THEN count ELSE 0 END), 0) as total_voice,
				COALESCE(SUM(CASE WHEN media_type = 'document' THEN count ELSE 0 END), 0) as total_documents,
				COALESCE(SUM(CASE WHEN media_type = 'sticker' THEN count ELSE 0 END), 0) as total_stickers,
				COALESCE(SUM(total_size), 0) as total_size
			FROM mv_media_distribution
			WHERE chat_id = $1
		`, chatID).Scan(&stats.TotalMedia, &stats.TotalPhotos, &stats.TotalVideos,
			&stats.TotalGifs, &stats.TotalVoice, &stats.TotalDocuments, &stats.TotalStickers,
			&stats.TotalSize)
		if err != nil {
			return nil, err
		}

		// Calculate other (total - known types)
		stats.TotalOther = stats.TotalMedia - stats.TotalPhotos - stats.TotalVideos -
			stats.TotalGifs - stats.TotalVoice - stats.TotalDocuments - stats.TotalStickers

		// Calculate media per day using active days
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
			stats.MediaPerDay = float64(stats.TotalMedia) / float64(activeDays)
		}
	} else {
		// Date-filtered: use live query combining media_files and photos tables
		err := r.db.QueryRowContext(ctx, `
			WITH media_stats AS (
				SELECT
					mf.media_type,
					COUNT(*) as count,
					COALESCE(SUM(mf.file_size), 0) as total_size
				FROM media_files mf
				JOIN messages m ON m.id = mf.message_id
				WHERE m.chat_id = $1
					AND m.date >= $2
					AND m.date < $3
				GROUP BY mf.media_type

				UNION ALL

				SELECT
					'photo' as media_type,
					COUNT(*) as count,
					COALESCE(SUM(p.file_size), 0) as total_size
				FROM photos p
				JOIN messages m ON m.id = p.message_id
				WHERE m.chat_id = $1
					AND m.date >= $2
					AND m.date < $3
			)
			SELECT
				COALESCE(SUM(count), 0) as total_media,
				COALESCE(SUM(CASE WHEN media_type = 'photo' THEN count ELSE 0 END), 0) as total_photos,
				COALESCE(SUM(CASE WHEN media_type IN ('video', 'video_note') THEN count ELSE 0 END), 0) as total_videos,
				COALESCE(SUM(CASE WHEN media_type = 'animation' THEN count ELSE 0 END), 0) as total_gifs,
				COALESCE(SUM(CASE WHEN media_type = 'voice' THEN count ELSE 0 END), 0) as total_voice,
				COALESCE(SUM(CASE WHEN media_type = 'document' THEN count ELSE 0 END), 0) as total_documents,
				COALESCE(SUM(CASE WHEN media_type = 'sticker' THEN count ELSE 0 END), 0) as total_stickers,
				COALESCE(SUM(total_size), 0) as total_size
			FROM media_stats
		`, chatID, startDate, endDate).Scan(&stats.TotalMedia, &stats.TotalPhotos, &stats.TotalVideos,
			&stats.TotalGifs, &stats.TotalVoice, &stats.TotalDocuments, &stats.TotalStickers,
			&stats.TotalSize)
		if err != nil {
			return nil, err
		}

		stats.TotalOther = stats.TotalMedia - stats.TotalPhotos - stats.TotalVideos -
			stats.TotalGifs - stats.TotalVoice - stats.TotalDocuments - stats.TotalStickers

		// Calculate media per day
		if startDate != nil && endDate != nil {
			days := int(endDate.Sub(*startDate).Hours() / 24)
			if days > 0 {
				stats.MediaPerDay = float64(stats.TotalMedia) / float64(days)
			}
		}
	}

	return stats, nil
}

// GetMediaTypeDistribution returns the distribution of media types for a chat.
func (r *StatsRepository) GetMediaTypeDistribution(ctx context.Context, chatID int64, startDate, endDate *time.Time) ([]MediaTypeDistribution, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-media-type-distribution")()

	var rows *sql.Rows
	var err error

	if startDate == nil && endDate == nil {
		// All-time: use materialized view
		rows, err = r.db.QueryContext(ctx, `
			SELECT media_type, count, COALESCE(total_size, 0) as total_size
			FROM mv_media_distribution
			WHERE chat_id = $1
			ORDER BY count DESC
		`, chatID)
	} else {
		// Date-filtered: use live query
		rows, err = r.db.QueryContext(ctx, `
			WITH media_stats AS (
				SELECT
					mf.media_type::text as media_type,
					COUNT(*) as count,
					COALESCE(SUM(mf.file_size), 0) as total_size
				FROM media_files mf
				JOIN messages m ON m.id = mf.message_id
				WHERE m.chat_id = $1
					AND m.date >= $2
					AND m.date < $3
				GROUP BY mf.media_type

				UNION ALL

				SELECT
					'photo' as media_type,
					COUNT(*) as count,
					COALESCE(SUM(p.file_size), 0) as total_size
				FROM photos p
				JOIN messages m ON m.id = p.message_id
				WHERE m.chat_id = $1
					AND m.date >= $2
					AND m.date < $3
			)
			SELECT media_type, SUM(count) as count, SUM(total_size) as total_size
			FROM media_stats
			GROUP BY media_type
			ORDER BY count DESC
		`, chatID, startDate, endDate)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var distribution []MediaTypeDistribution
	for rows.Next() {
		var d MediaTypeDistribution
		if err := rows.Scan(&d.MediaType, &d.Count, &d.TotalSize); err != nil {
			return nil, err
		}
		distribution = append(distribution, d)
	}

	return distribution, rows.Err()
}

// GetMediaActivity returns daily media upload activity for a chat.
func (r *StatsRepository) GetMediaActivity(ctx context.Context, chatID int64, startDate, endDate *time.Time, tz *time.Location) ([]MediaActivity, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-media-activity")()

	tzName := "UTC"
	if tz != nil {
		tzName = tz.String()
	}

	var rows *sql.Rows
	var err error

	if startDate == nil && endDate == nil {
		// All-time query with timezone
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				DATE(m.date AT TIME ZONE $2) as activity_date,
				COUNT(DISTINCT COALESCE(mf.id::text, 'p' || p.id::text)) as media_count
			FROM messages m
			LEFT JOIN media_files mf ON mf.message_id = m.id
			LEFT JOIN photos p ON p.message_id = m.id
			WHERE m.chat_id = $1
				AND (mf.id IS NOT NULL OR p.id IS NOT NULL)
			GROUP BY DATE(m.date AT TIME ZONE $2)
			ORDER BY activity_date
		`, chatID, tzName)
	} else {
		// Date-filtered query with timezone
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				DATE(m.date AT TIME ZONE $4) as activity_date,
				COUNT(DISTINCT COALESCE(mf.id::text, 'p' || p.id::text)) as media_count
			FROM messages m
			LEFT JOIN media_files mf ON mf.message_id = m.id
			LEFT JOIN photos p ON p.message_id = m.id
			WHERE m.chat_id = $1
				AND m.date >= $2
				AND m.date < $3
				AND (mf.id IS NOT NULL OR p.id IS NOT NULL)
			GROUP BY DATE(m.date AT TIME ZONE $4)
			ORDER BY activity_date
		`, chatID, startDate, endDate, tzName)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var activity []MediaActivity
	for rows.Next() {
		var date time.Time
		var a MediaActivity
		if err := rows.Scan(&date, &a.Count); err != nil {
			return nil, err
		}
		a.Date = date.Format("2006-01-02")
		activity = append(activity, a)
	}

	return activity, rows.Err()
}
