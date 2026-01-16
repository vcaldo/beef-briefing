package shop

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"beef-briefing/apps/api-service/internal/game/battle"
	"beef-briefing/apps/api-service/internal/jsonutil"
	"beef-briefing/apps/api-service/internal/nrutil"

	"github.com/newrelic/go-agent/v3/newrelic"
)

// CardService interface for fetching card images
type CardService interface {
	GetCardImageURLString(ctx context.Context, userID, chatID int64, weekStart *time.Time, theme string, expirySeconds int) (string, error)
}

// Dealer handles card dealing from the database
type Dealer struct {
	db            *sql.DB
	nrApp         *newrelic.Application
	storageClient interface {
		GetPresignedURL(ctx context.Context, objectKey string, expiry time.Duration) (string, error)
	}
	cardService CardService
}

// NewDealer creates a new card dealer
func NewDealer(db *sql.DB, nrApp *newrelic.Application, storageClient interface {
	GetPresignedURL(ctx context.Context, objectKey string, expiry time.Duration) (string, error)
}, cardService CardService) *Dealer {
	return &Dealer{db: db, nrApp: nrApp, storageClient: storageClient, cardService: cardService}
}

// CombatStats represents the combat section of card stats
type CombatStats struct {
	ATK    int     `json:"atk"`
	DEF    int     `json:"def"`
	HP     int     `json:"hp"`
	RawATK float64 `json:"raw_atk"`
	RawDEF float64 `json:"raw_def"`
}

// CardStats represents the full stats object from ml_user_cards
type CardStats struct {
	Combat CombatStats `json:"combat"`
	// Other stats can be added as needed
}

// DealCards fetches random cards from the current week for a chat
func (d *Dealer) DealCards(ctx context.Context, chatID int64, count int) ([]*battle.ShopCard, error) {
	defer nrutil.StartSegment(ctx, "db:deal-cards")()

	// Query current week's cards with user info, randomly ordered
	query := `
		SELECT
			c.id,
			c.user_id,
			u.first_name,
			u.username,
			c.stats
		FROM ml_user_cards c
		JOIN users u ON c.user_id = u.id
		WHERE c.chat_id = $1
		  AND c.week_start = (
			SELECT MAX(week_start)
			FROM ml_user_cards
			WHERE chat_id = $1
		  )
		ORDER BY RANDOM()
		LIMIT $2
	`

	rows, err := d.db.QueryContext(ctx, query, chatID, count)
	if err != nil {
		return nil, fmt.Errorf("failed to query cards: %w", err)
	}
	defer rows.Close()

	cards := make([]*battle.ShopCard, 0, count)
	index := 0

	for rows.Next() {
		var (
			cardID    int64
			userID    int64
			firstName string
			username  sql.NullString
			statsJSON []byte
		)

		if err := rows.Scan(&cardID, &userID, &firstName, &username, &statsJSON); err != nil {
			return nil, fmt.Errorf("failed to scan card row: %w", err)
		}

		// Parse stats to get combat values
		var stats CardStats
		if err := jsonutil.Unmarshal(statsJSON, &stats); err != nil {
			return nil, fmt.Errorf("failed to parse card stats: %w", err)
		}

		card := &battle.ShopCard{
			CardID:      cardID,
			UserID:      userID,
			Name:        firstName,
			Username:    username.String,
			ATK:         stats.Combat.ATK,
			DEF:         stats.Combat.DEF,
			HP:          stats.Combat.HP,
			Stats:       statsJSON,
			IsPurchased: false,
			Index:       index,
		}

		// Try to get photo URL (for battle phase)
		photoURL, _ := d.getUserPhotoURL(ctx, userID)
		card.PhotoURL = photoURL

		// Try to get card image URL (for shop phase)
		theme := "neon_arcade"
		cardImageURL := d.getCardImageURL(ctx, userID, chatID, theme)
		card.CardImageURL = cardImageURL

		cards = append(cards, card)
		index++
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating card rows: %w", err)
	}

	if len(cards) < count {
		// Not enough cards, but return what we have
		// The service layer will check if minimum is met
	}

	return cards, nil
}

// DealRerollCards fetches additional random cards excluding already seen ones
func (d *Dealer) DealRerollCards(ctx context.Context, chatID int64, count int, excludeCardIDs []int64) ([]*battle.ShopCard, error) {
	defer nrutil.StartSegment(ctx, "db:deal-reroll-cards")()

	// Build exclusion list for query
	excludeClause := ""
	args := []interface{}{chatID, count}
	if len(excludeCardIDs) > 0 {
		excludeClause = " AND c.id NOT IN ("
		for i, id := range excludeCardIDs {
			if i > 0 {
				excludeClause += ","
			}
			excludeClause += fmt.Sprintf("$%d", len(args)+1)
			args = append(args, id)
		}
		excludeClause += ")"
	}

	query := fmt.Sprintf(`
		SELECT
			c.id,
			c.user_id,
			u.first_name,
			u.username,
			c.stats
		FROM ml_user_cards c
		JOIN users u ON c.user_id = u.id
		WHERE c.chat_id = $1
		  AND c.week_start = (
			SELECT MAX(week_start)
			FROM ml_user_cards
			WHERE chat_id = $1
		  )
		  %s
		ORDER BY RANDOM()
		LIMIT $2
	`, excludeClause)

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query reroll cards: %w", err)
	}
	defer rows.Close()

	cards := make([]*battle.ShopCard, 0, count)

	for rows.Next() {
		var (
			cardID    int64
			userID    int64
			firstName string
			username  sql.NullString
			statsJSON []byte
		)

		if err := rows.Scan(&cardID, &userID, &firstName, &username, &statsJSON); err != nil {
			return nil, fmt.Errorf("failed to scan reroll card row: %w", err)
		}

		var stats CardStats
		if err := jsonutil.Unmarshal(statsJSON, &stats); err != nil {
			return nil, fmt.Errorf("failed to parse card stats: %w", err)
		}

		card := &battle.ShopCard{
			CardID:      cardID,
			UserID:      userID,
			Name:        firstName,
			Username:    username.String,
			ATK:         stats.Combat.ATK,
			DEF:         stats.Combat.DEF,
			HP:          stats.Combat.HP,
			Stats:       statsJSON,
			IsPurchased: false,
		}

		photoURL, _ := d.getUserPhotoURL(ctx, userID)
		card.PhotoURL = photoURL

		// Try to get card image URL (for shop phase)
		theme := "neon_arcade"
		cardImageURL := d.getCardImageURL(ctx, userID, chatID, theme)
		card.CardImageURL = cardImageURL

		cards = append(cards, card)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating reroll card rows: %w", err)
	}

	return cards, nil
}

// GetCardCount returns the number of cards available for a chat in the current week
func (d *Dealer) GetCardCount(ctx context.Context, chatID int64) (int, error) {
	defer nrutil.StartSegment(ctx, "db:count-cards")()

	query := `
		SELECT COUNT(*)
		FROM ml_user_cards
		WHERE chat_id = $1
		  AND week_start = (
			SELECT MAX(week_start)
			FROM ml_user_cards
			WHERE chat_id = $1
		  )
	`

	var count int
	err := d.db.QueryRowContext(ctx, query, chatID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count cards: %w", err)
	}

	return count, nil
}

// getUserPhotoURL fetches the user's profile photo URL as a presigned HTTPS URL
func (d *Dealer) getUserPhotoURL(ctx context.Context, userID int64) (string, error) {
	defer nrutil.StartSegment(ctx, "db:get-user-photo-url")()

	query := `
		SELECT minio_object_key
		FROM user_profile_photos
		WHERE user_id = $1
		ORDER BY width DESC
		LIMIT 1
	`

	var minioObjectKey string
	err := d.db.QueryRowContext(ctx, query, userID).Scan(&minioObjectKey)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}

	// Convert object key to presigned URL
	if d.storageClient == nil || minioObjectKey == "" {
		return "", nil
	}

	url, err := d.storageClient.GetPresignedURL(ctx, minioObjectKey, time.Hour)
	if err != nil {
		// Log warning but don't fail the entire card deal
		return "", nil
	}

	return url, nil
}

// getCardImageURL fetches the user's card image URL for shop display
func (d *Dealer) getCardImageURL(ctx context.Context, userID, chatID int64, theme string) string {
	defer nrutil.StartSegment(ctx, "service:get-card-image-url")()

	if d.cardService == nil {
		return ""
	}

	// Fetch card image for current week (nil weekStart = latest)
	result, err := d.cardService.GetCardImageURLString(ctx, userID, chatID, nil, theme, 3600)
	if err != nil {
		// Log but don't fail - graceful degradation
		// Card image not found is expected for some users
		return ""
	}

	return result
}
