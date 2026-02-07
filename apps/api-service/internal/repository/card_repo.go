package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"beef-briefing/apps/api-service/internal/nrutil"

	"github.com/newrelic/go-agent/v3/newrelic"
)

// CardRepository handles database operations for user cards.
type CardRepository struct {
	db    *sql.DB
	nrApp *newrelic.Application
}

// NewCardRepository creates a new CardRepository.
func NewCardRepository(db *sql.DB, nrApp *newrelic.Application) *CardRepository {
	return &CardRepository{db: db, nrApp: nrApp}
}

// UserCard represents a user card from the database.
// Stats and Trends are passed through as raw JSON without parsing.
type UserCard struct {
	ID               int64           `json:"id"`
	UserID           int64           `json:"user_id"`
	ChatID           int64           `json:"chat_id"`
	WeekStart        string          `json:"week_start"`
	WeekEnd          string          `json:"week_end"`
	StatsWindowStart string          `json:"stats_window_start"`
	StatsWindowEnd   string          `json:"stats_window_end"`
	Stats            json.RawMessage `json:"stats"`
	Trends           json.RawMessage `json:"trends,omitempty"`
	MessagesAnalyzed int             `json:"messages_analyzed"`
	Timezone         *string         `json:"timezone,omitempty"`
	CardVersion      int             `json:"card_version"`
	GeneratedAt      time.Time       `json:"generated_at"`
}

// CardUser represents user display info.
type CardUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

// CardWithUser combines card data with user info for leaderboard.
type CardWithUser struct {
	UserID           int64           `json:"user_id"`
	User             CardUser        `json:"user"`
	WeekStart        string          `json:"week_start"`
	WeekEnd          string          `json:"week_end"`
	Stats            json.RawMessage `json:"stats"`
	Trends           json.RawMessage `json:"trends,omitempty"`
	MessagesAnalyzed int             `json:"messages_analyzed"`
	Rank             int             `json:"rank,omitempty"`
}

// WeekInfo represents available weeks with card counts.
type WeekInfo struct {
	WeekStart string `json:"week_start"`
	WeekEnd   string `json:"week_end"`
	CardCount int    `json:"card_count"`
}

// ChatCardsQuery holds query parameters for GetChatCards.
type ChatCardsQuery struct {
	ChatID    int64
	WeekStart *time.Time
	SortBy    string // mood, influence, activity, reactions
	SortDesc  bool
	Limit     int
	Offset    int
}

// GetUserCard retrieves a single card for a user.
func (r *CardRepository) GetUserCard(
	ctx context.Context,
	userID int64,
	chatID int64,
	weekStart *time.Time,
) (*UserCard, error) {
	defer nrutil.StartSegment(ctx, "db:card-get-user")()

	var query string
	var args []interface{}

	if weekStart != nil {
		query = `
			SELECT id, user_id, chat_id, week_start, week_end,
			       stats_window_start, stats_window_end,
			       stats, trends, messages_analyzed,
			       timezone, card_version, generated_at
			FROM ml_user_cards
			WHERE user_id = $1 AND chat_id = $2 AND week_start = $3
		`
		args = []interface{}{userID, chatID, weekStart}
	} else {
		query = `
			SELECT id, user_id, chat_id, week_start, week_end,
			       stats_window_start, stats_window_end,
			       stats, trends, messages_analyzed,
			       timezone, card_version, generated_at
			FROM ml_user_cards
			WHERE user_id = $1 AND chat_id = $2
			ORDER BY week_start DESC
			LIMIT 1
		`
		args = []interface{}{userID, chatID}
	}

	var card UserCard
	var statsJSON, trendsJSON []byte

	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&card.ID, &card.UserID, &card.ChatID,
		&card.WeekStart, &card.WeekEnd,
		&card.StatsWindowStart, &card.StatsWindowEnd,
		&statsJSON, &trendsJSON, &card.MessagesAnalyzed,
		&card.Timezone, &card.CardVersion, &card.GeneratedAt,
	)
	if err != nil {
		return nil, err
	}

	card.Stats = statsJSON
	if trendsJSON != nil {
		card.Trends = trendsJSON
	}

	return &card, nil
}

// GetUserInfo retrieves user display info.
func (r *CardRepository) GetUserInfo(ctx context.Context, userID int64) (*CardUser, error) {
	defer nrutil.StartSegment(ctx, "db:card-get-user-info")()

	query := `
		SELECT id, first_name, COALESCE(last_name, ''), COALESCE(username, '')
		FROM users
		WHERE id = $1
	`

	var user CardUser
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&user.ID, &user.FirstName, &user.LastName, &user.Username,
	)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// getSortColumn returns the SQL expression for sorting by the given field.
func getSortColumn(sortBy string) string {
	switch sortBy {
	case "mood":
		return "(stats->'mood'->>'score')::numeric"
	case "influence":
		return "(stats->'influence'->>'score')::numeric"
	case "activity":
		return "(stats->'activity'->>'messages')::numeric"
	case "reactions":
		return "(stats->>'reactions_received')::numeric"
	default:
		return "(stats->'mood'->>'score')::numeric"
	}
}

// GetChatCards retrieves all cards for a chat with sorting and pagination.
func (r *CardRepository) GetChatCards(
	ctx context.Context,
	q ChatCardsQuery,
) ([]CardWithUser, int, error) {
	defer nrutil.StartSegment(ctx, "db:card-get-chat")()

	// Determine week to query
	var weekStart time.Time
	if q.WeekStart != nil {
		weekStart = *q.WeekStart
	} else {
		// Get latest week
		err := r.db.QueryRowContext(ctx, `
			SELECT week_start FROM ml_user_cards
			WHERE chat_id = $1
			ORDER BY week_start DESC LIMIT 1
		`, q.ChatID).Scan(&weekStart)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get latest week: %w", err)
		}
	}

	// Count total cards for this week
	var total int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM ml_user_cards
		WHERE chat_id = $1 AND week_start = $2
	`, q.ChatID, weekStart).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count cards: %w", err)
	}

	// Build order clause
	orderDir := "DESC"
	if !q.SortDesc {
		orderDir = "ASC"
	}
	sortColumn := getSortColumn(q.SortBy)

	query := fmt.Sprintf(`
		SELECT c.user_id, c.week_start, c.week_end,
		       c.stats, c.trends, c.messages_analyzed,
		       u.first_name, COALESCE(u.last_name, ''), COALESCE(u.username, ''),
		       ROW_NUMBER() OVER (ORDER BY %s %s NULLS LAST) as rank
		FROM ml_user_cards c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.chat_id = $1 AND c.week_start = $2
		ORDER BY %s %s NULLS LAST
		LIMIT $3 OFFSET $4
	`, sortColumn, orderDir, sortColumn, orderDir)

	rows, err := r.db.QueryContext(ctx, query, q.ChatID, weekStart, q.Limit, q.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query cards: %w", err)
	}
	defer rows.Close()

	var cards []CardWithUser
	for rows.Next() {
		var card CardWithUser
		var statsJSON, trendsJSON []byte

		err := rows.Scan(
			&card.UserID, &card.WeekStart, &card.WeekEnd,
			&statsJSON, &trendsJSON, &card.MessagesAnalyzed,
			&card.User.FirstName, &card.User.LastName, &card.User.Username,
			&card.Rank,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan card: %w", err)
		}

		card.User.ID = card.UserID
		card.Stats = statsJSON
		if trendsJSON != nil {
			card.Trends = trendsJSON
		}

		cards = append(cards, card)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("row iteration error: %w", err)
	}

	return cards, total, nil
}

// GetUserHistory retrieves a user's card history.
func (r *CardRepository) GetUserHistory(
	ctx context.Context,
	userID int64,
	chatID int64,
	limit int,
) ([]UserCard, error) {
	defer nrutil.StartSegment(ctx, "db:card-get-history")()

	query := `
		SELECT id, user_id, chat_id, week_start, week_end,
		       stats_window_start, stats_window_end,
		       stats, trends, messages_analyzed,
		       timezone, card_version, generated_at
		FROM ml_user_cards
		WHERE user_id = $1 AND chat_id = $2
		ORDER BY week_start DESC
		LIMIT $3
	`

	rows, err := r.db.QueryContext(ctx, query, userID, chatID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query history: %w", err)
	}
	defer rows.Close()

	var history []UserCard
	for rows.Next() {
		var card UserCard
		var statsJSON, trendsJSON []byte

		err := rows.Scan(
			&card.ID, &card.UserID, &card.ChatID,
			&card.WeekStart, &card.WeekEnd,
			&card.StatsWindowStart, &card.StatsWindowEnd,
			&statsJSON, &trendsJSON, &card.MessagesAnalyzed,
			&card.Timezone, &card.CardVersion, &card.GeneratedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan history: %w", err)
		}

		card.Stats = statsJSON
		if trendsJSON != nil {
			card.Trends = trendsJSON
		}

		history = append(history, card)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return history, nil
}

// GetAvailableWeeks returns weeks with generated cards for a chat.
func (r *CardRepository) GetAvailableWeeks(
	ctx context.Context,
	chatID int64,
) ([]WeekInfo, error) {
	defer nrutil.StartSegment(ctx, "db:card-get-weeks")()

	query := `
		SELECT week_start, week_end, COUNT(*) as card_count
		FROM ml_user_cards
		WHERE chat_id = $1
		GROUP BY week_start, week_end
		ORDER BY week_start DESC
	`

	rows, err := r.db.QueryContext(ctx, query, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to query weeks: %w", err)
	}
	defer rows.Close()

	var weeks []WeekInfo
	for rows.Next() {
		var week WeekInfo
		err := rows.Scan(&week.WeekStart, &week.WeekEnd, &week.CardCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan week: %w", err)
		}
		weeks = append(weeks, week)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return weeks, nil
}

// CardImage represents a card image from the database.
type CardImage struct {
	ID              int64     `json:"id"`
	CardID          int64     `json:"card_id"`
	UserID          int64     `json:"user_id"`
	ChatID          int64     `json:"chat_id"`
	WeekStart       string    `json:"week_start"`
	StoragePath     string    `json:"storage_path"`
	Theme           string    `json:"theme"`
	Width           int       `json:"width"`
	Height          int       `json:"height"`
	CardDataVersion int       `json:"card_data_version"`
	GeneratedAt     time.Time `json:"generated_at"`
}

// GetCardImage retrieves a card image by user, chat, and optional week.
func (r *CardRepository) GetCardImage(
	ctx context.Context,
	userID int64,
	chatID int64,
	weekStart *time.Time,
	theme string,
) (*CardImage, error) {
	defer nrutil.StartSegment(ctx, "db:card-get-image")()

	if theme == "" {
		theme = "gaming"
	}

	var query string
	var args []interface{}

	if weekStart != nil {
		query = `
			SELECT id, card_id, user_id, chat_id, week_start,
			       storage_path, theme, width, height,
			       card_data_version, generated_at
			FROM ml_user_card_images
			WHERE user_id = $1 AND chat_id = $2 AND week_start = $3 AND theme = $4
		`
		args = []interface{}{userID, chatID, weekStart, theme}
	} else {
		query = `
			SELECT id, card_id, user_id, chat_id, week_start,
			       storage_path, theme, width, height,
			       card_data_version, generated_at
			FROM ml_user_card_images
			WHERE user_id = $1 AND chat_id = $2 AND theme = $3
			ORDER BY week_start DESC
			LIMIT 1
		`
		args = []interface{}{userID, chatID, theme}
	}

	var img CardImage
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&img.ID, &img.CardID, &img.UserID, &img.ChatID, &img.WeekStart,
		&img.StoragePath, &img.Theme, &img.Width, &img.Height,
		&img.CardDataVersion, &img.GeneratedAt,
	)
	if err != nil {
		return nil, err
	}

	return &img, nil
}

// GalleryImage represents a card image for the gallery with user info.
type GalleryImage struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	ChatID      int64     `json:"chat_id"`
	WeekStart   string    `json:"week_start"`
	StoragePath string    `json:"storage_path"`
	Theme       string    `json:"theme"`
	GeneratedAt time.Time `json:"generated_at"`
	FirstName   *string   `json:"first_name,omitempty"`
	LastName    *string   `json:"last_name,omitempty"`
	Username    *string   `json:"username,omitempty"`
}

// GetGalleryWeeks returns weeks with generated card images for a chat.
func (r *CardRepository) GetGalleryWeeks(ctx context.Context, chatID int64) ([]string, error) {
	defer nrutil.StartSegment(ctx, "db:gallery-get-weeks")()

	query := `
		SELECT DISTINCT week_start
		FROM ml_user_card_images
		WHERE chat_id = $1
		ORDER BY week_start DESC
	`

	rows, err := r.db.QueryContext(ctx, query, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to query gallery weeks: %w", err)
	}
	defer rows.Close()

	var weeks []string
	for rows.Next() {
		var week time.Time
		if err := rows.Scan(&week); err != nil {
			return nil, fmt.Errorf("failed to scan week: %w", err)
		}
		weeks = append(weeks, week.Format("2006-01-02"))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return weeks, nil
}

// GetGalleryImages returns card images for a chat/week with user info.
func (r *CardRepository) GetGalleryImages(
	ctx context.Context,
	chatID int64,
	weekStart *time.Time,
	userID *int64,
	theme *string,
) ([]GalleryImage, error) {
	defer nrutil.StartSegment(ctx, "db:gallery-get-images")()

	// Build query with optional filters
	query := `
		SELECT
			i.id, i.user_id, i.chat_id, i.week_start,
			i.storage_path, i.theme, i.generated_at,
			u.first_name, u.last_name, u.username
		FROM ml_user_card_images i
		JOIN users u ON i.user_id = u.id
		JOIN ml_user_cards c ON i.card_id = c.id
		WHERE i.chat_id = $1
		  AND i.week_start = COALESCE($2, (SELECT MAX(week_start) FROM ml_user_card_images WHERE chat_id = $1))
		  AND ($3::bigint IS NULL OR i.user_id = $3)
		  AND ($4::text IS NULL OR i.theme = $4)
		  AND ($4::text IS NOT NULL OR i.theme NOT LIKE '%\_compact' ESCAPE '\')
		ORDER BY (c.stats->'overall'->>'score')::float DESC NULLS LAST
	`

	rows, err := r.db.QueryContext(ctx, query, chatID, weekStart, userID, theme)
	if err != nil {
		return nil, fmt.Errorf("failed to query gallery images: %w", err)
	}
	defer rows.Close()

	var images []GalleryImage
	for rows.Next() {
		var img GalleryImage
		var weekTime time.Time
		err := rows.Scan(
			&img.ID, &img.UserID, &img.ChatID, &weekTime,
			&img.StoragePath, &img.Theme, &img.GeneratedAt,
			&img.FirstName, &img.LastName, &img.Username,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan gallery image: %w", err)
		}
		img.WeekStart = weekTime.Format("2006-01-02")
		images = append(images, img)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return images, nil
}

// GetGalleryImageByID returns a single gallery image by ID.
func (r *CardRepository) GetGalleryImageByID(ctx context.Context, imageID int64) (*GalleryImage, error) {
	defer nrutil.StartSegment(ctx, "db:gallery-get-image-by-id")()

	query := `
		SELECT id, user_id, chat_id, week_start, storage_path, theme, generated_at
		FROM ml_user_card_images
		WHERE id = $1
	`

	var img GalleryImage
	var weekTime time.Time
	err := r.db.QueryRowContext(ctx, query, imageID).Scan(
		&img.ID, &img.UserID, &img.ChatID, &weekTime,
		&img.StoragePath, &img.Theme, &img.GeneratedAt,
	)
	if err != nil {
		return nil, err
	}
	img.WeekStart = weekTime.Format("2006-01-02")

	return &img, nil
}
