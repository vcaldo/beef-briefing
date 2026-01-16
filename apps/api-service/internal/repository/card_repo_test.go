package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"
)

// TestGetUserCard_WithStats tests retrieving a user card with JSON stats
func TestGetUserCard_WithStats(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	repo := NewCardRepository(db, nil)
	cleanupTables(t, db, "ml_user_card_images", "ml_user_cards", "users", "chats")

	ctx := context.Background()
	chatID := int64(-1002345678901)
	userID := int64(12345)
	weekStart := time.Date(2026, 1, 13, 0, 0, 0, 0, time.UTC)

	// Insert test data
	insertTestChat(t, db, chatID)
	insertTestUser(t, db, userID, "TestUser")

	// Create sample JSON stats
	stats := json.RawMessage(`{
		"mood": {"score": 85},
		"influence": {"score": 90},
		"activity": {"messages": 150},
		"reactions_received": 50
	}`)
	trends := json.RawMessage(`{"mood_trend": "up"}`)

	// Insert user card
	_, err := db.Exec(`
		INSERT INTO ml_user_cards (
			user_id, chat_id, week_start, week_end,
			stats_window_start, stats_window_end,
			stats, trends, messages_analyzed,
			timezone, card_version, generated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW())
	`,
		userID, chatID, weekStart, weekStart.AddDate(0, 0, 7),
		weekStart, weekStart.AddDate(0, 0, 7),
		stats, trends, 150,
		"UTC", 1,
	)
	if err != nil {
		t.Fatalf("Failed to insert test card: %v", err)
	}

	t.Run("GetUserCard_WithWeekStart", func(t *testing.T) {
		card, err := repo.GetUserCard(ctx, userID, chatID, &weekStart)
		if err != nil {
			t.Fatalf("GetUserCard failed: %v", err)
		}

		// Verify card properties
		if card.UserID != userID {
			t.Errorf("Expected user_id=%d, got %d", userID, card.UserID)
		}
		if card.ChatID != chatID {
			t.Errorf("Expected chat_id=%d, got %d", chatID, card.ChatID)
		}
		if card.MessagesAnalyzed != 150 {
			t.Errorf("Expected messages_analyzed=150, got %d", card.MessagesAnalyzed)
		}
		if card.CardVersion != 1 {
			t.Errorf("Expected card_version=1, got %d", card.CardVersion)
		}

		// Verify JSON stats are populated
		if len(card.Stats) == 0 {
			t.Error("Expected stats to be populated")
		}
		if len(card.Trends) == 0 {
			t.Error("Expected trends to be populated")
		}

		// Verify stats can be parsed
		var statsMap map[string]interface{}
		if err := json.Unmarshal(card.Stats, &statsMap); err != nil {
			t.Errorf("Failed to unmarshal stats: %v", err)
		}
		if statsMap["mood"] == nil {
			t.Error("Expected mood in stats")
		}
	})

	t.Run("GetUserCard_NoWeekStart_LatestCard", func(t *testing.T) {
		card, err := repo.GetUserCard(ctx, userID, chatID, nil)
		if err != nil {
			t.Fatalf("GetUserCard failed: %v", err)
		}

		// Should return the most recent card
		if card.UserID != userID {
			t.Errorf("Expected user_id=%d, got %d", userID, card.UserID)
		}
		if card.MessagesAnalyzed != 150 {
			t.Errorf("Expected messages_analyzed=150, got %d", card.MessagesAnalyzed)
		}
	})

	t.Run("GetUserCard_NotFound", func(t *testing.T) {
		nonExistentUserID := int64(99999)
		card, err := repo.GetUserCard(ctx, nonExistentUserID, chatID, &weekStart)
		if err == nil {
			t.Error("Expected error for non-existent card")
		}
		if card != nil {
			t.Errorf("Expected nil card for non-existent user, got %v", card)
		}
		if err != sql.ErrNoRows {
			t.Errorf("Expected sql.ErrNoRows, got %v", err)
		}
	})
}

// TestGetChatCards_Sorting tests retrieving chat cards with sorting
func TestGetChatCards_Sorting(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	repo := NewCardRepository(db, nil)
	cleanupTables(t, db, "ml_user_card_images", "ml_user_cards", "users", "chats")

	ctx := context.Background()
	chatID := int64(-1002345678901)
	weekStart := time.Date(2026, 1, 13, 0, 0, 0, 0, time.UTC)

	// Insert test data
	insertTestChat(t, db, chatID)

	// Create 3 users with different scores
	users := []struct {
		id        int64
		name      string
		moodScore int
	}{
		{12345, "User1", 85},
		{12346, "User2", 90},
		{12347, "User3", 80},
	}

	for _, u := range users {
		insertTestUser(t, db, u.id, u.name)

		stats := json.RawMessage(
			`{"mood": {"score": ` + string(rune('0'+u.moodScore/10)) + string(rune('0'+u.moodScore%10)) + `},
			  "influence": {"score": 80},
			  "activity": {"messages": 100},
			  "reactions_received": 40}`)

		_, err := db.Exec(`
			INSERT INTO ml_user_cards (
				user_id, chat_id, week_start, week_end,
				stats_window_start, stats_window_end,
				stats, messages_analyzed,
				card_version, generated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		`,
			u.id, chatID, weekStart, weekStart.AddDate(0, 0, 7),
			weekStart, weekStart.AddDate(0, 0, 7),
			stats, 100, 1,
		)
		if err != nil {
			t.Fatalf("Failed to insert test card for user %d: %v", u.id, err)
		}
	}

	t.Run("SortByMood_Descending", func(t *testing.T) {
		query := ChatCardsQuery{
			ChatID:   chatID,
			SortBy:   "mood",
			SortDesc: true,
			Limit:    10,
			Offset:   0,
		}

		cards, total, err := repo.GetChatCards(ctx, query)
		if err != nil {
			t.Fatalf("GetChatCards failed: %v", err)
		}

		// Verify we got all 3 cards
		if total != 3 {
			t.Errorf("Expected total=3, got %d", total)
		}
		if len(cards) != 3 {
			t.Errorf("Expected 3 cards, got %d", len(cards))
		}

		// Verify sorting order (90, 85, 80)
		if cards[0].UserID != 12346 {
			t.Errorf("Expected first card user_id=12346 (highest mood), got %d", cards[0].UserID)
		}
		if cards[1].UserID != 12345 {
			t.Errorf("Expected second card user_id=12345, got %d", cards[1].UserID)
		}
		if cards[2].UserID != 12347 {
			t.Errorf("Expected third card user_id=12347 (lowest mood), got %d", cards[2].UserID)
		}

		// Verify user info is populated
		if cards[0].User.FirstName != "User2" {
			t.Errorf("Expected first name=User2, got %s", cards[0].User.FirstName)
		}
	})

	t.Run("SortByMood_Ascending", func(t *testing.T) {
		query := ChatCardsQuery{
			ChatID:   chatID,
			SortBy:   "mood",
			SortDesc: false,
			Limit:    10,
			Offset:   0,
		}

		cards, _, err := repo.GetChatCards(ctx, query)
		if err != nil {
			t.Fatalf("GetChatCards failed: %v", err)
		}

		// Verify sorting order (80, 85, 90)
		if cards[0].UserID != 12347 {
			t.Errorf("Expected first card user_id=12347 (lowest mood), got %d", cards[0].UserID)
		}
		if cards[2].UserID != 12346 {
			t.Errorf("Expected third card user_id=12346 (highest mood), got %d", cards[2].UserID)
		}
	})

	t.Run("Pagination", func(t *testing.T) {
		query := ChatCardsQuery{
			ChatID:   chatID,
			SortBy:   "mood",
			SortDesc: true,
			Limit:    2,
			Offset:   0,
		}

		cards, total, err := repo.GetChatCards(ctx, query)
		if err != nil {
			t.Fatalf("GetChatCards failed: %v", err)
		}

		// Verify total is still 3
		if total != 3 {
			t.Errorf("Expected total=3, got %d", total)
		}
		// Verify we only got 2 cards
		if len(cards) != 2 {
			t.Errorf("Expected 2 cards (limit), got %d", len(cards))
		}

		// Get second page
		query.Offset = 2
		cards, total, err = repo.GetChatCards(ctx, query)
		if err != nil {
			t.Fatalf("GetChatCards failed: %v", err)
		}

		// Verify we got the remaining card
		if len(cards) != 1 {
			t.Errorf("Expected 1 card on second page, got %d", len(cards))
		}
		if cards[0].UserID != 12347 {
			t.Errorf("Expected last card user_id=12347, got %d", cards[0].UserID)
		}
	})
}

// TestGetCardImage_Found tests retrieving a card image
func TestGetCardImage_Found(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	repo := NewCardRepository(db, nil)
	cleanupTables(t, db, "ml_user_card_images", "ml_user_cards", "users", "chats")

	ctx := context.Background()
	chatID := int64(-1002345678901)
	userID := int64(12345)
	weekStart := time.Date(2026, 1, 13, 0, 0, 0, 0, time.UTC)
	theme := "gaming"

	// Insert test data
	insertTestChat(t, db, chatID)
	insertTestUser(t, db, userID, "TestUser")

	// Insert user card
	stats := json.RawMessage(`{"mood": {"score": 85}}`)
	var cardID int64
	err := db.QueryRow(`
		INSERT INTO ml_user_cards (
			user_id, chat_id, week_start, week_end,
			stats_window_start, stats_window_end,
			stats, messages_analyzed,
			card_version, generated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		RETURNING id
	`,
		userID, chatID, weekStart, weekStart.AddDate(0, 0, 7),
		weekStart, weekStart.AddDate(0, 0, 7),
		stats, 100, 1,
	).Scan(&cardID)
	if err != nil {
		t.Fatalf("Failed to insert test card: %v", err)
	}

	// Insert card image
	storagePath := "cards/gaming/12345_2026-01-13.png"
	fileHash := "abc123def456789abc123def456789abc123def456789abc123def456789abc1"
	_, err = db.Exec(`
		INSERT INTO ml_user_card_images (
			card_id, user_id, chat_id, week_start,
			storage_path, file_hash, file_size, mime_type,
			theme, width, height,
			template_version, card_data_version, generated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NOW())
	`,
		cardID, userID, chatID, weekStart,
		storagePath, fileHash, 123456, "image/png",
		theme, 400, 600,
		1, 1,
	)
	if err != nil {
		t.Fatalf("Failed to insert test card image: %v", err)
	}

	t.Run("GetCardImage_WithWeekStart", func(t *testing.T) {
		img, err := repo.GetCardImage(ctx, userID, chatID, &weekStart, theme)
		if err != nil {
			t.Fatalf("GetCardImage failed: %v", err)
		}

		// Verify image properties
		if img.UserID != userID {
			t.Errorf("Expected user_id=%d, got %d", userID, img.UserID)
		}
		if img.ChatID != chatID {
			t.Errorf("Expected chat_id=%d, got %d", chatID, img.ChatID)
		}
		if img.CardID != cardID {
			t.Errorf("Expected card_id=%d, got %d", cardID, img.CardID)
		}
		if img.StoragePath != storagePath {
			t.Errorf("Expected storage_path=%s, got %s", storagePath, img.StoragePath)
		}
		if img.Theme != theme {
			t.Errorf("Expected theme=%s, got %s", theme, img.Theme)
		}
		if img.Width != 400 {
			t.Errorf("Expected width=400, got %d", img.Width)
		}
		if img.Height != 600 {
			t.Errorf("Expected height=600, got %d", img.Height)
		}
		if img.CardDataVersion != 1 {
			t.Errorf("Expected card_data_version=1, got %d", img.CardDataVersion)
		}
	})

	t.Run("GetCardImage_NoWeekStart_LatestImage", func(t *testing.T) {
		img, err := repo.GetCardImage(ctx, userID, chatID, nil, theme)
		if err != nil {
			t.Fatalf("GetCardImage failed: %v", err)
		}

		// Should return the most recent image
		if img.UserID != userID {
			t.Errorf("Expected user_id=%d, got %d", userID, img.UserID)
		}
		if img.StoragePath != storagePath {
			t.Errorf("Expected storage_path=%s, got %s", storagePath, img.StoragePath)
		}
	})

	t.Run("GetCardImage_DifferentTheme_NotFound", func(t *testing.T) {
		differentTheme := "clean"
		img, err := repo.GetCardImage(ctx, userID, chatID, &weekStart, differentTheme)
		if err == nil {
			t.Error("Expected error for non-existent theme")
		}
		if img != nil {
			t.Errorf("Expected nil image for different theme, got %v", img)
		}
		if err != sql.ErrNoRows {
			t.Errorf("Expected sql.ErrNoRows, got %v", err)
		}
	})

	t.Run("GetCardImage_NonExistentUser_NotFound", func(t *testing.T) {
		nonExistentUserID := int64(99999)
		img, err := repo.GetCardImage(ctx, nonExistentUserID, chatID, &weekStart, theme)
		if err == nil {
			t.Error("Expected error for non-existent user")
		}
		if img != nil {
			t.Errorf("Expected nil image for non-existent user, got %v", img)
		}
		if err != sql.ErrNoRows {
			t.Errorf("Expected sql.ErrNoRows, got %v", err)
		}
	})
}
