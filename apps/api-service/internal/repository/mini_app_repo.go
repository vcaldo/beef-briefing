package repository

import (
	"context"
	"database/sql"
	"time"

	"beef-briefing/apps/api-service/internal/nrutil"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// MiniAppRepository handles database operations for Mini App analytics.
// It acts as a facade that delegates to specialized sub-repositories.
type MiniAppRepository struct {
	db    *sql.DB
	nrApp *newrelic.Application

	// Sub-repositories
	stats       *StatsRepository
	leaderboard *LeaderboardRepository
	heatmap     *HeatmapRepository
}

// NewMiniAppRepository creates a new MiniAppRepository.
func NewMiniAppRepository(db *sql.DB, nrApp *newrelic.Application) *MiniAppRepository {
	return &MiniAppRepository{
		db:          db,
		nrApp:       nrApp,
		stats:       NewStatsRepository(db, nrApp),
		leaderboard: NewLeaderboardRepository(db, nrApp),
		heatmap:     NewHeatmapRepository(db, nrApp),
	}
}

// =============================================================================
// Types
// =============================================================================

// OverviewStats represents overview statistics for a chat.
type OverviewStats struct {
	TotalMessages  int64   `json:"total_messages"`
	TotalUsers     int64   `json:"total_users"`
	TotalReactions int64   `json:"total_reactions"`
	TotalMedia     int64   `json:"total_media"`
	MessagesPerDay float64 `json:"messages_per_day"`
	// Trend fields (percentage change vs previous period, null if unavailable)
	TotalMessagesTrend  *float64 `json:"total_messages_trend,omitempty"`
	TotalUsersTrend     *float64 `json:"total_users_trend,omitempty"`
	TotalReactionsTrend *float64 `json:"total_reactions_trend,omitempty"`
	TotalMediaTrend     *float64 `json:"total_media_trend,omitempty"`
	MessagesPerDayTrend *float64 `json:"messages_per_day_trend,omitempty"`
}

// DailyActivity represents a single day's activity.
type DailyActivity struct {
	Date     string `json:"date"`
	Messages int64  `json:"messages"`
	Users    int64  `json:"users"`
}

// UserRanking represents a user's ranking in the leaderboard.
type UserRanking struct {
	Rank           int     `json:"rank"`
	UserID         int64   `json:"user_id"`
	FirstName      string  `json:"first_name"`
	LastName       *string `json:"last_name,omitempty"`
	Username       *string `json:"username,omitempty"`
	Score          int64   `json:"score"`
	PhotoObjectKey *string `json:"-"` // Internal use, not serialized
}

// TopReaction represents a reaction emoji and its count.
type TopReaction struct {
	Emoji        string `json:"emoji"`
	ReactionType string `json:"reaction_type"`
	Count        int64  `json:"count"`
}

// ReactionUser represents a user in reaction rankings.
type ReactionUser struct {
	Rank           int     `json:"rank"`
	UserID         int64   `json:"user_id"`
	FirstName      string  `json:"first_name"`
	LastName       *string `json:"last_name,omitempty"`
	Username       *string `json:"username,omitempty"`
	Score          int64   `json:"score"`
	PhotoObjectKey *string `json:"-"` // Internal use, not serialized
}

// ProfileStats represents personal stats for a user.
type ProfileStats struct {
	MessageCount            int64   `json:"message_count"`
	ReactionsSent           int64   `json:"reactions_sent"`
	ReactionsReceived       int64   `json:"reactions_received"`
	RepliesSent             int64   `json:"replies_sent"`
	RepliesReceived         int64   `json:"replies_received"`
	ActiveDays              int64   `json:"active_days"`
	CurrentStreak           int64   `json:"current_streak"`
	AvgMessagesPerDay       float64 `json:"avg_messages_per_day"`
	RankByMessages          int     `json:"rank_by_messages"`
	RankByReactionsReceived int     `json:"rank_by_reactions_received"`
	// Trend fields (percentage change vs previous period, null if unavailable)
	MessageCountTrend      *float64 `json:"message_count_trend,omitempty"`
	ReactionsSentTrend     *float64 `json:"reactions_sent_trend,omitempty"`
	ReactionsReceivedTrend *float64 `json:"reactions_received_trend,omitempty"`
	RepliesSentTrend       *float64 `json:"replies_sent_trend,omitempty"`
	RepliesReceivedTrend   *float64 `json:"replies_received_trend,omitempty"`
	ActiveDaysTrend        *float64 `json:"active_days_trend,omitempty"`
	AvgMessagesPerDayTrend *float64 `json:"avg_messages_per_day_trend,omitempty"`
}

// TopInteractor represents a user who interacts with another user.
type TopInteractor struct {
	Rank           int     `json:"rank"`
	UserID         int64   `json:"user_id"`
	FirstName      string  `json:"first_name"`
	LastName       *string `json:"last_name,omitempty"`
	Username       *string `json:"username,omitempty"`
	Score          int64   `json:"score"`
	TopEmoji       *string `json:"top_emoji,omitempty"`
	PhotoObjectKey *string `json:"-"` // Internal use, not serialized
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

// ChatUser represents a user in a chat for admin selection.
type ChatUser struct {
	UserID         int64   `json:"user_id"`
	FirstName      string  `json:"first_name"`
	LastName       *string `json:"last_name,omitempty"`
	Username       *string `json:"username,omitempty"`
	PhotoObjectKey *string `json:"-"` // Internal use, not serialized
}

// MediaOverviewStats represents aggregate media statistics for a chat.
type MediaOverviewStats struct {
	TotalMedia     int64   `json:"total_media"`
	TotalPhotos    int64   `json:"total_photos"`
	TotalVideos    int64   `json:"total_videos"`
	TotalGifs      int64   `json:"total_gifs"`
	TotalVoice     int64   `json:"total_voice"`
	TotalDocuments int64   `json:"total_documents"`
	TotalStickers  int64   `json:"total_stickers"`
	TotalOther     int64   `json:"total_other"`
	TotalSize      int64   `json:"total_size"`
	MediaPerDay    float64 `json:"media_per_day"`
	// Trends (percentage change from previous period)
	TotalMediaTrend     *float64 `json:"total_media_trend,omitempty"`
	TotalPhotosTrend    *float64 `json:"total_photos_trend,omitempty"`
	TotalVideosTrend    *float64 `json:"total_videos_trend,omitempty"`
	TotalGifsTrend      *float64 `json:"total_gifs_trend,omitempty"`
	TotalVoiceTrend     *float64 `json:"total_voice_trend,omitempty"`
	TotalDocumentsTrend *float64 `json:"total_documents_trend,omitempty"`
	TotalStickersTrend  *float64 `json:"total_stickers_trend,omitempty"`
	MediaPerDayTrend    *float64 `json:"media_per_day_trend,omitempty"`
}

// MediaTypeDistribution represents a media type and its count.
type MediaTypeDistribution struct {
	MediaType string `json:"media_type"`
	Count     int64  `json:"count"`
	TotalSize int64  `json:"total_size"`
}

// MediaActivity represents daily media upload counts.
type MediaActivity struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

// MediaUser represents a user in media rankings.
type MediaUser struct {
	Rank           int     `json:"rank"`
	UserID         int64   `json:"user_id"`
	FirstName      string  `json:"first_name"`
	LastName       *string `json:"last_name,omitempty"`
	Username       *string `json:"username,omitempty"`
	MediaCount     int64   `json:"media_count"`
	PhotoObjectKey *string `json:"-"` // Internal use, not serialized
}

// =============================================================================
// Helper functions
// =============================================================================

// sanitizeMetricColumn ensures the metric column name is safe for SQL
func sanitizeMetricColumn(metric string) string {
	switch metric {
	case "message_count":
		return "message_count"
	case "reactions_sent":
		return "reactions_sent"
	case "reactions_received":
		return "reactions_received"
	case "replies_sent":
		return "replies_sent"
	case "replies_received":
		return "replies_received"
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
	case "replies_sent":
		return "rps.replies_sent"
	case "replies_received":
		return "rpr.replies_received"
	case "active_days":
		return "us.active_days"
	default:
		return "us.message_count"
	}
}

// =============================================================================
// Stats delegation
// =============================================================================

// GetOverviewStats returns overview statistics for a chat.
func (r *MiniAppRepository) GetOverviewStats(ctx context.Context, chatID int64, startDate, endDate *time.Time) (*OverviewStats, error) {
	return r.stats.GetOverviewStats(ctx, chatID, startDate, endDate)
}

// GetDailyActivity returns daily message activity for a chat.
func (r *MiniAppRepository) GetDailyActivity(ctx context.Context, chatID int64, startDate, endDate *time.Time, tz *time.Location) ([]DailyActivity, error) {
	return r.stats.GetDailyActivity(ctx, chatID, startDate, endDate, tz)
}

// GetUserDailyActivity returns daily message activity for a specific user in a chat.
func (r *MiniAppRepository) GetUserDailyActivity(ctx context.Context, chatID, userID int64, startDate, endDate *time.Time, tz *time.Location) ([]DailyActivity, error) {
	return r.stats.GetUserDailyActivity(ctx, chatID, userID, startDate, endDate, tz)
}

// GetUserProfileStats returns personal stats for a user including their rankings.
func (r *MiniAppRepository) GetUserProfileStats(ctx context.Context, chatID, userID int64, startDate, endDate *time.Time, tzName string) (*ProfileStats, error) {
	return r.stats.GetUserProfileStats(ctx, chatID, userID, startDate, endDate, tzName)
}

// GetMediaOverviewStats returns aggregate media statistics for a chat.
func (r *MiniAppRepository) GetMediaOverviewStats(ctx context.Context, chatID int64, startDate, endDate *time.Time) (*MediaOverviewStats, error) {
	return r.stats.GetMediaOverviewStats(ctx, chatID, startDate, endDate)
}

// GetMediaTypeDistribution returns the distribution of media types for a chat.
func (r *MiniAppRepository) GetMediaTypeDistribution(ctx context.Context, chatID int64, startDate, endDate *time.Time) ([]MediaTypeDistribution, error) {
	return r.stats.GetMediaTypeDistribution(ctx, chatID, startDate, endDate)
}

// GetMediaActivity returns daily media upload activity for a chat.
func (r *MiniAppRepository) GetMediaActivity(ctx context.Context, chatID int64, startDate, endDate *time.Time, tz *time.Location) ([]MediaActivity, error) {
	return r.stats.GetMediaActivity(ctx, chatID, startDate, endDate, tz)
}

// =============================================================================
// Leaderboard delegation
// =============================================================================

// GetUserRankings returns user rankings for a chat.
func (r *MiniAppRepository) GetUserRankings(ctx context.Context, chatID int64, metric string, limit, offset int, startDate, endDate *time.Time, tzName string) ([]UserRanking, error) {
	return r.leaderboard.GetUserRankings(ctx, chatID, metric, limit, offset, startDate, endDate, tzName)
}

// GetUserRankingsTotal returns the total count of users for pagination.
func (r *MiniAppRepository) GetUserRankingsTotal(ctx context.Context, chatID int64, startDate, endDate *time.Time) (int, error) {
	return r.leaderboard.GetUserRankingsTotal(ctx, chatID, startDate, endDate)
}

// GetTopReactions returns the top reactions used in a chat.
func (r *MiniAppRepository) GetTopReactions(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]TopReaction, error) {
	return r.leaderboard.GetTopReactions(ctx, chatID, limit, startDate, endDate)
}

// GetTopReactionGivers returns users who give the most reactions.
func (r *MiniAppRepository) GetTopReactionGivers(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]ReactionUser, error) {
	return r.leaderboard.GetTopReactionGivers(ctx, chatID, limit, startDate, endDate)
}

// GetTopReactionReceivers returns users who receive the most reactions.
func (r *MiniAppRepository) GetTopReactionReceivers(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]ReactionUser, error) {
	return r.leaderboard.GetTopReactionReceivers(ctx, chatID, limit, startDate, endDate)
}

// GetTopReactorsToUser returns users who react most to a specific user's messages.
func (r *MiniAppRepository) GetTopReactorsToUser(ctx context.Context, chatID, userID int64, limit int, startDate, endDate *time.Time) ([]TopInteractor, error) {
	return r.leaderboard.GetTopReactorsToUser(ctx, chatID, userID, limit, startDate, endDate)
}

// GetTopReactedToByUser returns users whose messages a specific user reacts to most.
func (r *MiniAppRepository) GetTopReactedToByUser(ctx context.Context, chatID, userID int64, limit int, startDate, endDate *time.Time) ([]TopInteractor, error) {
	return r.leaderboard.GetTopReactedToByUser(ctx, chatID, userID, limit, startDate, endDate)
}

// GetTopRepliersToUser returns users who reply most to a specific user's messages.
func (r *MiniAppRepository) GetTopRepliersToUser(ctx context.Context, chatID, userID int64, limit int, startDate, endDate *time.Time) ([]TopInteractor, error) {
	return r.leaderboard.GetTopRepliersToUser(ctx, chatID, userID, limit, startDate, endDate)
}

// GetTopRepliedToByUser returns users that a specific user replies to most.
func (r *MiniAppRepository) GetTopRepliedToByUser(ctx context.Context, chatID, userID int64, limit int, startDate, endDate *time.Time) ([]TopInteractor, error) {
	return r.leaderboard.GetTopRepliedToByUser(ctx, chatID, userID, limit, startDate, endDate)
}

// GetTopReplySenders returns users who send the most replies in a chat.
func (r *MiniAppRepository) GetTopReplySenders(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]ReactionUser, error) {
	return r.leaderboard.GetTopReplySenders(ctx, chatID, limit, startDate, endDate)
}

// GetTopReplyReceivers returns users whose messages receive the most replies in a chat.
func (r *MiniAppRepository) GetTopReplyReceivers(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]ReactionUser, error) {
	return r.leaderboard.GetTopReplyReceivers(ctx, chatID, limit, startDate, endDate)
}

// GetTopMediaSenders returns users who send the most media in a chat.
func (r *MiniAppRepository) GetTopMediaSenders(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]MediaUser, error) {
	return r.leaderboard.GetTopMediaSenders(ctx, chatID, limit, startDate, endDate)
}

// =============================================================================
// Heatmap delegation
// =============================================================================

// GetGroupHeatmap returns the activity heatmap for a chat.
func (r *MiniAppRepository) GetGroupHeatmap(ctx context.Context, chatID int64, startDate, endDate *time.Time, tz *time.Location) (*HeatmapData, error) {
	return r.heatmap.GetGroupHeatmap(ctx, chatID, startDate, endDate, tz)
}

// GetUserHeatmap returns the activity heatmap for a specific user.
func (r *MiniAppRepository) GetUserHeatmap(ctx context.Context, chatID, userID int64, startDate, endDate *time.Time, tz *time.Location) (*HeatmapData, error) {
	return r.heatmap.GetUserHeatmap(ctx, chatID, userID, startDate, endDate, tz)
}

// =============================================================================
// Chat and user info (kept in main repository)
// =============================================================================

// GetUserPhotoObjectKey returns the largest profile photo object key for a user.
func (r *MiniAppRepository) GetUserPhotoObjectKey(ctx context.Context, userID int64) (*string, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-user-photo-key")()

	var objectKey sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT minio_object_key
		FROM user_profile_photos
		WHERE user_id = $1
		ORDER BY width DESC
		LIMIT 1
	`, userID).Scan(&objectKey)

	if err == sql.ErrNoRows || !objectKey.Valid {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &objectKey.String, nil
}

// GetChatTitle returns the title of a chat.
func (r *MiniAppRepository) GetChatTitle(ctx context.Context, chatID int64) (*string, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-chat-title")()

	var title sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT title
		FROM chats
		WHERE id = $1
	`, chatID).Scan(&title)

	if err == sql.ErrNoRows || !title.Valid {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &title.String, nil
}

// GetChatUsers returns all non-bot users who have sent messages in a chat.
func (r *MiniAppRepository) GetChatUsers(ctx context.Context, chatID int64) ([]ChatUser, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-chat-users")()

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			user_id,
			first_name,
			last_name,
			username,
			(SELECT minio_object_key FROM user_profile_photos WHERE user_id = mv_user_statistics.user_id ORDER BY width DESC LIMIT 1) as photo_object_key
		FROM mv_user_statistics
		WHERE chat_id = $1 AND is_bot = false
		ORDER BY first_name, last_name
	`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []ChatUser
	for rows.Next() {
		var u ChatUser
		if err := rows.Scan(&u.UserID, &u.FirstName, &u.LastName, &u.Username, &u.PhotoObjectKey); err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	return users, rows.Err()
}

// GetUserInfo returns basic info for a specific user.
func (r *MiniAppRepository) GetUserInfo(ctx context.Context, userID int64) (*ChatUser, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-user-info")()

	var u ChatUser
	err := r.db.QueryRowContext(ctx, `
		SELECT
			id,
			first_name,
			last_name,
			username,
			(SELECT minio_object_key FROM user_profile_photos WHERE user_id = users.id ORDER BY width DESC LIMIT 1) as photo_object_key
		FROM users
		WHERE id = $1
	`, userID).Scan(&u.UserID, &u.FirstName, &u.LastName, &u.Username, &u.PhotoObjectKey)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &u, nil
}

// =============================================================================
// Settings (kept in main repository)
// =============================================================================

// GetChatTimezone returns the configured timezone for a chat, or nil if not set.
func (r *MiniAppRepository) GetChatTimezone(ctx context.Context, chatID int64) (*string, error) {
	defer nrutil.StartSegment(ctx, "db:mini-app-chat-timezone")()

	var timezone sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT timezone FROM chats WHERE id = $1`,
		chatID,
	).Scan(&timezone)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if timezone.Valid {
		return &timezone.String, nil
	}
	return nil, nil
}

// SetChatTimezone sets the timezone for a chat. Only admins should call this.
func (r *MiniAppRepository) SetChatTimezone(ctx context.Context, chatID int64, timezone string) error {
	defer nrutil.StartSegment(ctx, "db:mini-app-set-chat-timezone")()

	_, err := r.db.ExecContext(ctx,
		`UPDATE chats SET timezone = $1, updated_at = NOW() WHERE id = $2`,
		timezone, chatID,
	)
	return err
}
