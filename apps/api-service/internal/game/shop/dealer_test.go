package shop

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// mockCardService implements the CardService interface for testing
type mockCardService struct {
	placeholderPositions json.RawMessage
	cardImageURL         string
	cardImageError       error
}

func (m *mockCardService) GetCardImageURLString(ctx context.Context, userID, chatID int64, weekStart *time.Time, theme string, expirySeconds int) (string, error) {
	if m.cardImageError != nil {
		return "", m.cardImageError
	}
	return m.cardImageURL, nil
}

func (m *mockCardService) GetPlaceholderPositions(theme string) json.RawMessage {
	return m.placeholderPositions
}

// mockStorageClient implements the storage interface for testing
type mockStorageClient struct {
	presignedURL string
	urlError     error
}

func (m *mockStorageClient) GetPresignedURL(ctx context.Context, objectKey string, expiry time.Duration) (string, error) {
	if m.urlError != nil {
		return "", m.urlError
	}
	return m.presignedURL, nil
}

// setupTestDB creates a test database connection
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	host := getEnvOrDefault("TEST_DB_HOST", "localhost")
	port := getEnvOrDefault("TEST_DB_PORT", "5432")
	user := getEnvOrDefault("TEST_DB_USER", "postgres")
	password := getEnvOrDefault("TEST_DB_PASSWORD", "Vb96vpmAEZVdSpruALAgDmZrHFRDXATg")
	dbname := getEnvOrDefault("TEST_DB_NAME", "beef_briefing")
	sslmode := getEnvOrDefault("TEST_DB_SSLMODE", "disable")

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Fatalf("failed to ping test database: %v", err)
	}

	return db
}

// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// setupTestData creates test data in the database for dealer tests
func setupTestData(t *testing.T, db *sql.DB, chatID int64, weekStart time.Time) {
	t.Helper()

	ctx := context.Background()

	// Start transaction for cleanup
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Insert test chat
	_, err = tx.Exec(`
		INSERT INTO chats (id, type, title)
		VALUES ($1, 'supergroup', 'Test Chat')
		ON CONFLICT (id) DO NOTHING
	`, chatID)
	if err != nil {
		t.Fatalf("failed to insert test chat: %v", err)
	}

	// Insert test users
	users := []struct {
		id        int64
		firstName string
		username  string
	}{
		{1001, "Alice", "alice"},
		{1002, "Bob", "bob"},
		{1003, "Charlie", "charlie"},
	}

	for _, u := range users {
		_, err := tx.Exec(`
			INSERT INTO users (id, first_name, username)
			VALUES ($1, $2, $3)
			ON CONFLICT (id) DO UPDATE SET first_name = $2, username = $3
		`, u.id, u.firstName, u.username)
		if err != nil {
			t.Fatalf("failed to insert user %d: %v", u.id, err)
		}
	}

	// Insert test cards for the current week
	stats1 := `{"combat":{"atk":75,"def":50,"hp":80,"raw_atk":7.5,"raw_def":5.0}}`
	stats2 := `{"combat":{"atk":60,"def":70,"hp":90,"raw_atk":6.0,"raw_def":7.0}}`
	stats3 := `{"combat":{"atk":85,"def":40,"hp":70,"raw_atk":8.5,"raw_def":4.0}}`

	// Calculate week_end (6 days after week_start)
	weekEnd := weekStart.AddDate(0, 0, 6)
	statsWindowStart := weekStart.AddDate(0, 0, -23) // 30-day window
	statsWindowEnd := weekEnd

	cards := []struct {
		userID int64
		stats  string
	}{
		{1001, stats1},
		{1002, stats2},
		{1003, stats3},
	}

	for _, c := range cards {
		_, err := tx.Exec(`
			INSERT INTO ml_user_cards (
				chat_id, user_id, week_start, week_end,
				stats_window_start, stats_window_end,
				stats, messages_analyzed
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 100)
		`, chatID, c.userID, weekStart, weekEnd, statsWindowStart, statsWindowEnd, c.stats)
		if err != nil {
			t.Fatalf("failed to insert card for user %d: %v", c.userID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit test data: %v", err)
	}
}

// cleanupTestData removes test data from the database
func cleanupTestData(t *testing.T, db *sql.DB, chatID int64) {
	t.Helper()

	ctx := context.Background()
	_, _ = db.ExecContext(ctx, `DELETE FROM ml_user_cards WHERE chat_id = $1`, chatID)
	_, _ = db.ExecContext(ctx, `DELETE FROM chats WHERE id = $1`, chatID)
	_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id IN (1001, 1002, 1003)`)
}

// TestDealCards_IncludesPlaceholderPositions verifies that dealt cards include placeholder positions
func TestDealCards_IncludesPlaceholderPositions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	chatID := int64(-1001234567890)
	weekStart := time.Now().Truncate(24 * time.Hour)

	// Setup test data
	setupTestData(t, db, chatID, weekStart)
	defer cleanupTestData(t, db, chatID)

	// Create mock card service with placeholder positions
	placeholderJSON := json.RawMessage(`{
		"version": "1.0",
		"card_dimensions": {"width": 300, "height": 450},
		"placeholders": {
			"atk": {"x": 10, "y": 20},
			"def": {"x": 30, "y": 40},
			"hp": {"x": 50, "y": 60}
		}
	}`)
	mockCardSvc := &mockCardService{
		placeholderPositions: placeholderJSON,
		cardImageURL:         "https://example.com/card.png",
	}

	// Create dealer
	dealer := NewDealer(db, nil, &mockStorageClient{}, mockCardSvc)

	// Deal cards
	cards, err := dealer.DealCards(ctx, chatID, 3)
	if err != nil {
		t.Fatalf("DealCards failed: %v", err)
	}

	// Verify we got cards
	if len(cards) == 0 {
		t.Fatal("expected at least one card, got none")
	}

	// Verify each card has placeholder positions
	for i, card := range cards {
		if card.PlaceholderPositions == nil {
			t.Errorf("card %d: PlaceholderPositions is nil", i)
			continue
		}

		// Verify it's valid JSON
		var parsed map[string]interface{}
		if err := json.Unmarshal(card.PlaceholderPositions, &parsed); err != nil {
			t.Errorf("card %d: PlaceholderPositions is not valid JSON: %v", i, err)
			continue
		}

		// Verify structure
		if _, ok := parsed["version"]; !ok {
			t.Errorf("card %d: missing 'version' field in PlaceholderPositions", i)
		}
		if _, ok := parsed["card_dimensions"]; !ok {
			t.Errorf("card %d: missing 'card_dimensions' field in PlaceholderPositions", i)
		}
		if _, ok := parsed["placeholders"]; !ok {
			t.Errorf("card %d: missing 'placeholders' field in PlaceholderPositions", i)
		}

		// Verify it matches the expected JSON
		if string(card.PlaceholderPositions) != string(placeholderJSON) {
			t.Errorf("card %d: PlaceholderPositions does not match expected JSON", i)
		}
	}
}

// TestDealCards_NilCardService verifies graceful handling when card service is nil
func TestDealCards_NilCardService(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	chatID := int64(-1001234567891)
	weekStart := time.Now().Truncate(24 * time.Hour)

	// Setup test data
	setupTestData(t, db, chatID, weekStart)
	defer cleanupTestData(t, db, chatID)

	// Create dealer with nil card service
	dealer := NewDealer(db, nil, nil, nil)

	// Deal cards
	cards, err := dealer.DealCards(ctx, chatID, 3)
	if err != nil {
		t.Fatalf("DealCards failed: %v", err)
	}

	// Verify we got cards
	if len(cards) == 0 {
		t.Fatal("expected at least one card, got none")
	}

	// Verify placeholder positions are nil (graceful degradation)
	for i, card := range cards {
		if card.PlaceholderPositions != nil {
			t.Errorf("card %d: expected nil PlaceholderPositions when card service is nil, got %v", i, card.PlaceholderPositions)
		}
	}
}

// TestDealRerollCards_IncludesPlaceholderPositions verifies that reroll cards include placeholder positions
func TestDealRerollCards_IncludesPlaceholderPositions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	chatID := int64(-1001234567892)
	weekStart := time.Now().Truncate(24 * time.Hour)

	// Setup test data
	setupTestData(t, db, chatID, weekStart)
	defer cleanupTestData(t, db, chatID)

	// Create mock card service with placeholder positions
	placeholderJSON := json.RawMessage(`{
		"version": "1.0",
		"card_dimensions": {"width": 300, "height": 450},
		"placeholders": {
			"atk": {"x": 10, "y": 20},
			"def": {"x": 30, "y": 40},
			"hp": {"x": 50, "y": 60}
		}
	}`)
	mockCardSvc := &mockCardService{
		placeholderPositions: placeholderJSON,
	}

	// Create dealer
	dealer := NewDealer(db, nil, &mockStorageClient{}, mockCardSvc)

	// Deal reroll cards (excluding none)
	cards, err := dealer.DealRerollCards(ctx, chatID, 2, []int64{})
	if err != nil {
		t.Fatalf("DealRerollCards failed: %v", err)
	}

	// Verify we got cards
	if len(cards) == 0 {
		t.Fatal("expected at least one card, got none")
	}

	// Verify each card has placeholder positions
	for i, card := range cards {
		if card.PlaceholderPositions == nil {
			t.Errorf("card %d: PlaceholderPositions is nil", i)
			continue
		}

		// Verify it's valid JSON
		var parsed map[string]interface{}
		if err := json.Unmarshal(card.PlaceholderPositions, &parsed); err != nil {
			t.Errorf("card %d: PlaceholderPositions is not valid JSON: %v", i, err)
			continue
		}

		// Verify it matches the expected JSON
		if string(card.PlaceholderPositions) != string(placeholderJSON) {
			t.Errorf("card %d: PlaceholderPositions does not match expected JSON", i)
		}
	}
}

// TestDealCards_VerifiesAllFields verifies that all ShopCard fields are populated correctly
func TestDealCards_VerifiesAllFields(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	chatID := int64(-1001234567893)
	weekStart := time.Now().Truncate(24 * time.Hour)

	// Setup test data
	setupTestData(t, db, chatID, weekStart)
	defer cleanupTestData(t, db, chatID)

	// Create mock card service
	placeholderJSON := json.RawMessage(`{"version": "1.0"}`)
	mockCardSvc := &mockCardService{
		placeholderPositions: placeholderJSON,
		cardImageURL:         "https://example.com/card.png",
	}

	// Create dealer
	dealer := NewDealer(db, nil, &mockStorageClient{}, mockCardSvc)

	// Deal cards
	cards, err := dealer.DealCards(ctx, chatID, 3)
	if err != nil {
		t.Fatalf("DealCards failed: %v", err)
	}

	// Verify we got 3 cards
	if len(cards) != 3 {
		t.Fatalf("expected 3 cards, got %d", len(cards))
	}

	// Verify all fields are populated
	for i, card := range cards {
		if card.CardID == 0 {
			t.Errorf("card %d: CardID is zero", i)
		}
		if card.UserID == 0 {
			t.Errorf("card %d: UserID is zero", i)
		}
		if card.Name == "" {
			t.Errorf("card %d: Name is empty", i)
		}
		if card.ATK == 0 {
			t.Errorf("card %d: ATK is zero", i)
		}
		if card.HP == 0 {
			t.Errorf("card %d: HP is zero", i)
		}
		if card.Stats == nil {
			t.Errorf("card %d: Stats is nil", i)
		}
		if card.IsPurchased {
			t.Errorf("card %d: IsPurchased should be false for newly dealt cards", i)
		}
		if card.PlaceholderPositions == nil {
			t.Errorf("card %d: PlaceholderPositions is nil", i)
		}
		if card.CardImageURL == "" {
			t.Errorf("card %d: CardImageURL is empty", i)
		}
	}
}

// TestDealRerollCards_ExcludesCards verifies that reroll excludes specified card IDs
func TestDealRerollCards_ExcludesCards(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	chatID := int64(-1001234567894)
	weekStart := time.Now().Truncate(24 * time.Hour)

	// Setup test data
	setupTestData(t, db, chatID, weekStart)
	defer cleanupTestData(t, db, chatID)

	// Create dealer
	mockCardSvc := &mockCardService{
		placeholderPositions: json.RawMessage(`{}`),
	}
	dealer := NewDealer(db, nil, &mockStorageClient{}, mockCardSvc)

	// First, deal initial cards to get their IDs
	initialCards, err := dealer.DealCards(ctx, chatID, 2)
	if err != nil {
		t.Fatalf("DealCards failed: %v", err)
	}
	if len(initialCards) < 2 {
		t.Fatalf("expected at least 2 cards, got %d", len(initialCards))
	}

	// Extract card IDs to exclude
	excludeIDs := []int64{initialCards[0].CardID, initialCards[1].CardID}

	// Deal reroll cards excluding the first two
	rerollCards, err := dealer.DealRerollCards(ctx, chatID, 1, excludeIDs)
	if err != nil {
		t.Fatalf("DealRerollCards failed: %v", err)
	}

	// Verify we got cards
	if len(rerollCards) == 0 {
		t.Fatal("expected at least one reroll card, got none")
	}

	// Verify none of the reroll cards match excluded IDs
	for _, rerollCard := range rerollCards {
		for _, excludedID := range excludeIDs {
			if rerollCard.CardID == excludedID {
				t.Errorf("reroll card has excluded ID %d", excludedID)
			}
		}
	}

	// Verify reroll cards still have placeholder positions
	for i, card := range rerollCards {
		if card.PlaceholderPositions == nil {
			t.Errorf("reroll card %d: PlaceholderPositions is nil", i)
		}
	}
}

// TestGetCardCount verifies card count query works correctly
func TestGetCardCount(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	chatID := int64(-1001234567895)
	weekStart := time.Now().Truncate(24 * time.Hour)

	// Setup test data (creates 3 cards)
	setupTestData(t, db, chatID, weekStart)
	defer cleanupTestData(t, db, chatID)

	// Create dealer
	dealer := NewDealer(db, nil, nil, nil)

	// Get card count
	count, err := dealer.GetCardCount(ctx, chatID)
	if err != nil {
		t.Fatalf("GetCardCount failed: %v", err)
	}

	// Verify count
	if count != 3 {
		t.Errorf("expected count of 3, got %d", count)
	}
}
