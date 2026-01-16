package repository

import (
	"context"
	"database/sql"
	"time"

	"beef-briefing/apps/api-service/internal/nrutil"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// LeaderboardRepository handles leaderboard and ranking-related database queries.
type LeaderboardRepository struct {
	db    *sql.DB
	nrApp *newrelic.Application
}

// NewLeaderboardRepository creates a new LeaderboardRepository.
func NewLeaderboardRepository(db *sql.DB, nrApp *newrelic.Application) *LeaderboardRepository {
	return &LeaderboardRepository{db: db, nrApp: nrApp}
}

// GetUserRankings returns user rankings for a chat.
// tzName is the IANA timezone name for streak calculations (e.g., "America/Sao_Paulo").
// If empty, defaults to UTC.
func (r *LeaderboardRepository) GetUserRankings(ctx context.Context, chatID int64, metric string, limit, offset int, startDate, endDate *time.Time, tzName string) ([]UserRanking, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-user-rankings")()

	// Default to UTC if no timezone specified
	if tzName == "" {
		tzName = "UTC"
	}

	var rows *sql.Rows
	var err error

	if startDate == nil && endDate == nil {
		// All-time: use materialized view for standard metrics, live query for reply/streak metrics
		if metric == "current_streak" {
			// Streak metric: compute consecutive days for all users (timezone-aware)
			query := `
				WITH user_dates AS (
					SELECT user_id, DATE(date AT TIME ZONE $4) as activity_date
					FROM messages
					WHERE chat_id = $1
					GROUP BY user_id, DATE(date AT TIME ZONE $4)
				),
				numbered AS (
					SELECT
						user_id,
						activity_date,
						activity_date - (ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY activity_date ASC))::int as grp
					FROM user_dates
				),
				user_last_active AS (
					SELECT user_id, MAX(activity_date) as last_active
					FROM user_dates
					GROUP BY user_id
				),
				user_streaks AS (
					SELECT
						n.user_id,
						CASE
							-- Only count streak if last activity was today or yesterday (in specified timezone)
							WHEN la.last_active >= (CURRENT_TIMESTAMP AT TIME ZONE $4)::date - INTERVAL '1 day'
							THEN (
								SELECT COUNT(*)
								FROM numbered n2
								WHERE n2.user_id = n.user_id
									AND n2.grp = (
										SELECT grp FROM numbered n3
										WHERE n3.user_id = n.user_id
										ORDER BY n3.activity_date DESC LIMIT 1
									)
							)
							ELSE 0
						END as streak
					FROM (SELECT DISTINCT user_id FROM numbered) n
					JOIN user_last_active la ON la.user_id = n.user_id
				)
				SELECT
					mv.user_id,
					mv.first_name,
					mv.last_name,
					mv.username,
					COALESCE(us.streak, 0) as score,
					ROW_NUMBER() OVER (ORDER BY COALESCE(us.streak, 0) DESC, mv.message_count DESC) as rank,
					(SELECT minio_object_key FROM user_profile_photos WHERE user_id = mv.user_id ORDER BY width DESC LIMIT 1) as photo_object_key
				FROM mv_user_statistics mv
				LEFT JOIN user_streaks us ON us.user_id = mv.user_id
				WHERE mv.chat_id = $1
					AND mv.is_bot = false
				ORDER BY COALESCE(us.streak, 0) DESC, mv.message_count DESC
				LIMIT $2 OFFSET $3
			`
			rows, err = r.db.QueryContext(ctx, query, chatID, limit, offset, tzName)
		} else if metric == "replies_sent" || metric == "replies_received" {
			// Reply metrics need live computation
			query := `
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
				)
				SELECT
					mv.user_id,
					mv.first_name,
					mv.last_name,
					mv.username,
					COALESCE(` + sanitizeMetricColumnCTE(metric) + `, 0) as score,
					ROW_NUMBER() OVER (ORDER BY COALESCE(` + sanitizeMetricColumnCTE(metric) + `, 0) DESC) as rank,
					(SELECT minio_object_key FROM user_profile_photos WHERE user_id = mv.user_id ORDER BY width DESC LIMIT 1) as photo_object_key
				FROM mv_user_statistics mv
				LEFT JOIN replies_sent rps ON rps.user_id = mv.user_id
				LEFT JOIN replies_received rpr ON rpr.user_id = mv.user_id
				WHERE mv.chat_id = $1
					AND mv.is_bot = false
				ORDER BY COALESCE(` + sanitizeMetricColumnCTE(metric) + `, 0) DESC
				LIMIT $2 OFFSET $3
			`
			rows, err = r.db.QueryContext(ctx, query, chatID, limit, offset)
		} else {
			// Standard metrics from MV
			query := `
				SELECT
					user_id,
					first_name,
					last_name,
					username,
					` + sanitizeMetricColumn(metric) + ` as score,
					ROW_NUMBER() OVER (ORDER BY ` + sanitizeMetricColumn(metric) + ` DESC) as rank,
					(SELECT minio_object_key FROM user_profile_photos WHERE user_id = mv_user_statistics.user_id ORDER BY width DESC LIMIT 1) as photo_object_key
				FROM mv_user_statistics
				WHERE chat_id = $1
					AND is_bot = false
				ORDER BY ` + sanitizeMetricColumn(metric) + ` DESC
				LIMIT $2 OFFSET $3
			`
			rows, err = r.db.QueryContext(ctx, query, chatID, limit, offset)
		}
	} else {
		// Date-filtered: use live query
		if metric == "current_streak" {
			// Streak metric with date filter (timezone-aware)
			query := `
				WITH user_dates AS (
					SELECT user_id, DATE(date AT TIME ZONE $6) as activity_date
					FROM messages
					WHERE chat_id = $1
						AND date >= $2
						AND date < $3
					GROUP BY user_id, DATE(date AT TIME ZONE $6)
				),
				numbered AS (
					SELECT
						user_id,
						activity_date,
						activity_date - (ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY activity_date ASC))::int as grp
					FROM user_dates
				),
				user_last_active AS (
					SELECT user_id, MAX(activity_date) as last_active
					FROM user_dates
					GROUP BY user_id
				),
				user_streaks AS (
					SELECT
						n.user_id,
						CASE
							-- Only count streak if last activity was within period end or yesterday (in specified timezone)
							WHEN la.last_active >= LEAST($3::date, (CURRENT_TIMESTAMP AT TIME ZONE $6)::date) - INTERVAL '1 day'
							THEN (
								SELECT COUNT(*)
								FROM numbered n2
								WHERE n2.user_id = n.user_id
									AND n2.grp = (
										SELECT grp FROM numbered n3
										WHERE n3.user_id = n.user_id
										ORDER BY n3.activity_date DESC LIMIT 1
									)
							)
							ELSE 0
						END as streak
					FROM (SELECT DISTINCT user_id FROM numbered) n
					JOIN user_last_active la ON la.user_id = n.user_id
				),
				user_info AS (
					SELECT
						m.user_id,
						u.first_name,
						u.last_name,
						u.username,
						u.is_bot,
						COUNT(*) as message_count
					FROM messages m
					JOIN users u ON u.id = m.user_id
					WHERE m.chat_id = $1
						AND m.date >= $2
						AND m.date < $3
					GROUP BY m.user_id, u.first_name, u.last_name, u.username, u.is_bot
				)
				SELECT
					ui.user_id,
					ui.first_name,
					ui.last_name,
					ui.username,
					COALESCE(us.streak, 0) as score,
					ROW_NUMBER() OVER (ORDER BY COALESCE(us.streak, 0) DESC, ui.message_count DESC) as rank,
					(SELECT minio_object_key FROM user_profile_photos WHERE user_id = ui.user_id ORDER BY width DESC LIMIT 1) as photo_object_key
				FROM user_info ui
				LEFT JOIN user_streaks us ON us.user_id = ui.user_id
				WHERE ui.is_bot = false
				ORDER BY COALESCE(us.streak, 0) DESC, ui.message_count DESC
				LIMIT $4 OFFSET $5
			`
			rows, err = r.db.QueryContext(ctx, query, chatID, startDate, endDate, limit, offset, tzName)
		} else {
			query := `
				WITH user_stats AS (
					SELECT
						m.user_id,
						u.first_name,
						u.last_name,
						u.username,
						u.is_bot,
						COUNT(*) as message_count,
						COUNT(DISTINCT DATE(m.date AT TIME ZONE $6)) as active_days
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
				)
				SELECT
					us.user_id,
					us.first_name,
					us.last_name,
					us.username,
					COALESCE(` + sanitizeMetricColumnCTE(metric) + `, 0) as score,
					ROW_NUMBER() OVER (ORDER BY COALESCE(` + sanitizeMetricColumnCTE(metric) + `, 0) DESC) as rank,
					(SELECT minio_object_key FROM user_profile_photos WHERE user_id = us.user_id ORDER BY width DESC LIMIT 1) as photo_object_key
				FROM user_stats us
				LEFT JOIN reactions_sent rs ON rs.user_id = us.user_id
				LEFT JOIN reactions_received rr ON rr.user_id = us.user_id
				LEFT JOIN replies_sent rps ON rps.user_id = us.user_id
				LEFT JOIN replies_received rpr ON rpr.user_id = us.user_id
				WHERE us.is_bot = false
				ORDER BY COALESCE(` + sanitizeMetricColumnCTE(metric) + `, 0) DESC
				LIMIT $4 OFFSET $5
			`
			rows, err = r.db.QueryContext(ctx, query, chatID, startDate, endDate, limit, offset, tzName)
		}
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rankings []UserRanking
	for rows.Next() {
		var rank int64
		var r UserRanking
		if err := rows.Scan(&r.UserID, &r.FirstName, &r.LastName, &r.Username, &r.Score, &rank, &r.PhotoObjectKey); err != nil {
			return nil, err
		}
		// Adjust rank for offset
		r.Rank = offset + len(rankings) + 1
		rankings = append(rankings, r)
	}

	return rankings, rows.Err()
}

// GetUserRankingsTotal returns the total count of users for pagination.
func (r *LeaderboardRepository) GetUserRankingsTotal(ctx context.Context, chatID int64, startDate, endDate *time.Time) (int, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-user-rankings-total")()

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

// GetTopReactions returns the top reactions used in a chat.
func (r *LeaderboardRepository) GetTopReactions(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]TopReaction, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-top-reactions")()

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
func (r *LeaderboardRepository) GetTopReactionGivers(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]ReactionUser, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-top-reaction-givers")()

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
				ROW_NUMBER() OVER (ORDER BY reactions_sent DESC) as rank,
				(SELECT minio_object_key FROM user_profile_photos WHERE user_id = mv_user_statistics.user_id ORDER BY width DESC LIMIT 1) as photo_object_key
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
				ROW_NUMBER() OVER (ORDER BY COUNT(*) DESC) as rank,
				(SELECT minio_object_key FROM user_profile_photos WHERE user_id = mr.user_id ORDER BY width DESC LIMIT 1) as photo_object_key
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
		if err := rows.Scan(&u.UserID, &u.FirstName, &u.LastName, &u.Username, &u.Score, &rank, &u.PhotoObjectKey); err != nil {
			return nil, err
		}
		u.Rank = int(rank)
		users = append(users, u)
	}

	return users, rows.Err()
}

// GetTopReactionReceivers returns users who receive the most reactions.
func (r *LeaderboardRepository) GetTopReactionReceivers(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]ReactionUser, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-top-reaction-receivers")()

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
				ROW_NUMBER() OVER (ORDER BY reactions_received DESC) as rank,
				(SELECT minio_object_key FROM user_profile_photos WHERE user_id = mv_user_statistics.user_id ORDER BY width DESC LIMIT 1) as photo_object_key
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
				ROW_NUMBER() OVER (ORDER BY COUNT(*) DESC) as rank,
				(SELECT minio_object_key FROM user_profile_photos WHERE user_id = m.user_id ORDER BY width DESC LIMIT 1) as photo_object_key
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
		if err := rows.Scan(&u.UserID, &u.FirstName, &u.LastName, &u.Username, &u.Score, &rank, &u.PhotoObjectKey); err != nil {
			return nil, err
		}
		u.Rank = int(rank)
		users = append(users, u)
	}

	return users, rows.Err()
}

// GetTopReactorsToUser returns users who react most to a specific user's messages.
func (r *LeaderboardRepository) GetTopReactorsToUser(ctx context.Context, chatID, userID int64, limit int, startDate, endDate *time.Time) ([]TopInteractor, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-top-reactors-to-user")()

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
				ROW_NUMBER() OVER (ORDER BY COUNT(*) DESC) as rank,
				(SELECT minio_object_key FROM user_profile_photos WHERE user_id = mr.user_id ORDER BY width DESC LIMIT 1) as photo_object_key
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
				ROW_NUMBER() OVER (ORDER BY COUNT(*) DESC) as rank,
				(SELECT minio_object_key FROM user_profile_photos WHERE user_id = mr.user_id ORDER BY width DESC LIMIT 1) as photo_object_key
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
		if err := rows.Scan(&i.UserID, &i.FirstName, &i.LastName, &i.Username, &i.Score, &i.TopEmoji, &rank, &i.PhotoObjectKey); err != nil {
			return nil, err
		}
		i.Rank = int(rank)
		interactors = append(interactors, i)
	}

	return interactors, rows.Err()
}

// GetTopReactedToByUser returns users whose messages a specific user reacts to most.
func (r *LeaderboardRepository) GetTopReactedToByUser(ctx context.Context, chatID, userID int64, limit int, startDate, endDate *time.Time) ([]TopInteractor, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-top-reacted-to-by-user")()

	var rows *sql.Rows
	var err error

	if startDate == nil && endDate == nil {
		// All-time: live query (no MV for this specific data)
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				m.user_id,
				u.first_name,
				u.last_name,
				u.username,
				COUNT(*) as score,
				MODE() WITHIN GROUP (ORDER BY COALESCE(mr.emoji_value, mr.custom_emoji_id, 'paid')) as top_emoji,
				ROW_NUMBER() OVER (ORDER BY COUNT(*) DESC) as rank,
				(SELECT minio_object_key FROM user_profile_photos WHERE user_id = m.user_id ORDER BY width DESC LIMIT 1) as photo_object_key
			FROM message_reactions mr
			JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
			JOIN users u ON u.id = m.user_id
			WHERE mr.chat_id = $1
				AND mr.user_id = $2
				AND m.user_id != $2
				AND mr.is_removed = false
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
				MODE() WITHIN GROUP (ORDER BY COALESCE(mr.emoji_value, mr.custom_emoji_id, 'paid')) as top_emoji,
				ROW_NUMBER() OVER (ORDER BY COUNT(*) DESC) as rank,
				(SELECT minio_object_key FROM user_profile_photos WHERE user_id = m.user_id ORDER BY width DESC LIMIT 1) as photo_object_key
			FROM message_reactions mr
			JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
			JOIN users u ON u.id = m.user_id
			WHERE mr.chat_id = $1
				AND mr.user_id = $2
				AND m.user_id != $2
				AND mr.is_removed = false
				AND mr.date >= $3
				AND mr.date < $4
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
		if err := rows.Scan(&i.UserID, &i.FirstName, &i.LastName, &i.Username, &i.Score, &i.TopEmoji, &rank, &i.PhotoObjectKey); err != nil {
			return nil, err
		}
		i.Rank = int(rank)
		interactors = append(interactors, i)
	}

	return interactors, rows.Err()
}

// GetTopRepliersToUser returns users who reply most to a specific user's messages.
func (r *LeaderboardRepository) GetTopRepliersToUser(ctx context.Context, chatID, userID int64, limit int, startDate, endDate *time.Time) ([]TopInteractor, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-top-repliers-to-user")()

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
				ROW_NUMBER() OVER (ORDER BY COUNT(*) DESC) as rank,
				(SELECT minio_object_key FROM user_profile_photos WHERE user_id = m.user_id ORDER BY width DESC LIMIT 1) as photo_object_key
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
				ROW_NUMBER() OVER (ORDER BY COUNT(*) DESC) as rank,
				(SELECT minio_object_key FROM user_profile_photos WHERE user_id = m.user_id ORDER BY width DESC LIMIT 1) as photo_object_key
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
		if err := rows.Scan(&i.UserID, &i.FirstName, &i.LastName, &i.Username, &i.Score, &rank, &i.PhotoObjectKey); err != nil {
			return nil, err
		}
		i.Rank = int(rank)
		interactors = append(interactors, i)
	}

	return interactors, rows.Err()
}

// GetTopRepliedToByUser returns users that a specific user replies to most.
func (r *LeaderboardRepository) GetTopRepliedToByUser(ctx context.Context, chatID, userID int64, limit int, startDate, endDate *time.Time) ([]TopInteractor, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-top-replied-to-by-user")()

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
				ROW_NUMBER() OVER (ORDER BY COUNT(*) DESC) as rank,
				(SELECT minio_object_key FROM user_profile_photos WHERE user_id = orig.user_id ORDER BY width DESC LIMIT 1) as photo_object_key
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
				ROW_NUMBER() OVER (ORDER BY COUNT(*) DESC) as rank,
				(SELECT minio_object_key FROM user_profile_photos WHERE user_id = orig.user_id ORDER BY width DESC LIMIT 1) as photo_object_key
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
		if err := rows.Scan(&i.UserID, &i.FirstName, &i.LastName, &i.Username, &i.Score, &rank, &i.PhotoObjectKey); err != nil {
			return nil, err
		}
		i.Rank = int(rank)
		interactors = append(interactors, i)
	}

	return interactors, rows.Err()
}

// GetTopReplySenders returns users who send the most replies in a chat.
func (r *LeaderboardRepository) GetTopReplySenders(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]ReactionUser, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-top-reply-senders")()

	var rows *sql.Rows
	var err error

	if startDate == nil && endDate == nil {
		// All-time: live query (no MV for reply stats)
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				m.user_id,
				u.first_name,
				u.last_name,
				u.username,
				COUNT(*) as score,
				ROW_NUMBER() OVER (ORDER BY COUNT(*) DESC) as rank,
				(SELECT minio_object_key FROM user_profile_photos WHERE user_id = m.user_id ORDER BY width DESC LIMIT 1) as photo_object_key
			FROM messages m
			JOIN users u ON u.id = m.user_id
			WHERE m.chat_id = $1
				AND m.reply_to_message_id IS NOT NULL
				AND u.is_bot = false
			GROUP BY m.user_id, u.first_name, u.last_name, u.username
			ORDER BY score DESC
			LIMIT $2
		`, chatID, limit)
	} else {
		// Date-filtered
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				m.user_id,
				u.first_name,
				u.last_name,
				u.username,
				COUNT(*) as score,
				ROW_NUMBER() OVER (ORDER BY COUNT(*) DESC) as rank,
				(SELECT minio_object_key FROM user_profile_photos WHERE user_id = m.user_id ORDER BY width DESC LIMIT 1) as photo_object_key
			FROM messages m
			JOIN users u ON u.id = m.user_id
			WHERE m.chat_id = $1
				AND m.reply_to_message_id IS NOT NULL
				AND m.date >= $2
				AND m.date < $3
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
		if err := rows.Scan(&u.UserID, &u.FirstName, &u.LastName, &u.Username, &u.Score, &rank, &u.PhotoObjectKey); err != nil {
			return nil, err
		}
		u.Rank = int(rank)
		users = append(users, u)
	}

	return users, rows.Err()
}

// GetTopReplyReceivers returns users whose messages receive the most replies in a chat.
func (r *LeaderboardRepository) GetTopReplyReceivers(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]ReactionUser, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-top-reply-receivers")()

	var rows *sql.Rows
	var err error

	if startDate == nil && endDate == nil {
		// All-time: live query (no MV for reply stats)
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				orig.user_id,
				u.first_name,
				u.last_name,
				u.username,
				COUNT(*) as score,
				ROW_NUMBER() OVER (ORDER BY COUNT(*) DESC) as rank,
				(SELECT minio_object_key FROM user_profile_photos WHERE user_id = orig.user_id ORDER BY width DESC LIMIT 1) as photo_object_key
			FROM messages m
			JOIN messages orig ON orig.chat_id = m.chat_id AND orig.message_id = m.reply_to_message_id
			JOIN users u ON u.id = orig.user_id
			WHERE m.chat_id = $1
				AND m.reply_to_message_id IS NOT NULL
				AND u.is_bot = false
			GROUP BY orig.user_id, u.first_name, u.last_name, u.username
			ORDER BY score DESC
			LIMIT $2
		`, chatID, limit)
	} else {
		// Date-filtered
		rows, err = r.db.QueryContext(ctx, `
			SELECT
				orig.user_id,
				u.first_name,
				u.last_name,
				u.username,
				COUNT(*) as score,
				ROW_NUMBER() OVER (ORDER BY COUNT(*) DESC) as rank,
				(SELECT minio_object_key FROM user_profile_photos WHERE user_id = orig.user_id ORDER BY width DESC LIMIT 1) as photo_object_key
			FROM messages m
			JOIN messages orig ON orig.chat_id = m.chat_id AND orig.message_id = m.reply_to_message_id
			JOIN users u ON u.id = orig.user_id
			WHERE m.chat_id = $1
				AND m.reply_to_message_id IS NOT NULL
				AND m.date >= $2
				AND m.date < $3
				AND u.is_bot = false
			GROUP BY orig.user_id, u.first_name, u.last_name, u.username
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
		if err := rows.Scan(&u.UserID, &u.FirstName, &u.LastName, &u.Username, &u.Score, &rank, &u.PhotoObjectKey); err != nil {
			return nil, err
		}
		u.Rank = int(rank)
		users = append(users, u)
	}

	return users, rows.Err()
}

// GetTopMediaSenders returns users who send the most media in a chat.
func (r *LeaderboardRepository) GetTopMediaSenders(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]MediaUser, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-top-media-senders")()

	var rows *sql.Rows
	var err error

	if startDate == nil && endDate == nil {
		// All-time: use live query (no MV for per-user media stats)
		rows, err = r.db.QueryContext(ctx, `
			WITH user_media AS (
				SELECT
					m.user_id,
					COUNT(DISTINCT COALESCE(mf.id::text, 'p' || p.id::text)) as media_count
				FROM messages m
				LEFT JOIN media_files mf ON mf.message_id = m.id
				LEFT JOIN photos p ON p.message_id = m.id
				WHERE m.chat_id = $1
					AND m.user_id IS NOT NULL
					AND (mf.id IS NOT NULL OR p.id IS NOT NULL)
				GROUP BY m.user_id
			)
			SELECT
				um.user_id,
				u.first_name,
				u.last_name,
				u.username,
				um.media_count,
				ROW_NUMBER() OVER (ORDER BY um.media_count DESC) as rank,
				(SELECT minio_object_key FROM user_profile_photos WHERE user_id = um.user_id ORDER BY width DESC LIMIT 1) as photo_object_key
			FROM user_media um
			JOIN users u ON u.id = um.user_id
			WHERE u.is_bot = false
			ORDER BY um.media_count DESC
			LIMIT $2
		`, chatID, limit)
	} else {
		// Date-filtered
		rows, err = r.db.QueryContext(ctx, `
			WITH user_media AS (
				SELECT
					m.user_id,
					COUNT(DISTINCT COALESCE(mf.id::text, 'p' || p.id::text)) as media_count
				FROM messages m
				LEFT JOIN media_files mf ON mf.message_id = m.id
				LEFT JOIN photos p ON p.message_id = m.id
				WHERE m.chat_id = $1
					AND m.user_id IS NOT NULL
					AND m.date >= $2
					AND m.date < $3
					AND (mf.id IS NOT NULL OR p.id IS NOT NULL)
				GROUP BY m.user_id
			)
			SELECT
				um.user_id,
				u.first_name,
				u.last_name,
				u.username,
				um.media_count,
				ROW_NUMBER() OVER (ORDER BY um.media_count DESC) as rank,
				(SELECT minio_object_key FROM user_profile_photos WHERE user_id = um.user_id ORDER BY width DESC LIMIT 1) as photo_object_key
			FROM user_media um
			JOIN users u ON u.id = um.user_id
			WHERE u.is_bot = false
			ORDER BY um.media_count DESC
			LIMIT $4
		`, chatID, startDate, endDate, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []MediaUser
	for rows.Next() {
		var rank int64
		var u MediaUser
		if err := rows.Scan(&u.UserID, &u.FirstName, &u.LastName, &u.Username, &u.MediaCount, &rank, &u.PhotoObjectKey); err != nil {
			return nil, err
		}
		u.Rank = int(rank)
		users = append(users, u)
	}

	return users, rows.Err()
}
