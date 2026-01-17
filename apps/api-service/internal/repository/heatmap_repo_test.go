package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"beef-briefing/apps/api-service/internal/models"
)

// setupHeatmapTestDB creates a new database connection for testing
func setupHeatmapTestDB(t *testing.T) *sql.DB {
	t.Helper()
	return setupUserTestDB(t) // Reuse existing setup
}

// teardownHeatmapTestDB closes the test database connection
func teardownHeatmapTestDB(t *testing.T, db *sql.DB) {
	t.Helper()
	teardownUserTestDB(t, db) // Reuse existing teardown
}

// cleanupHeatmapTables truncates all heatmap-related tables for test isolation
func cleanupHeatmapTables(t *testing.T, db *sql.DB) {
	t.Helper()

	ctx := context.Background()
	// Truncate in reverse dependency order (child tables first)
	tables := []string{
		"messages",
		"users",
		"chats",
	}

	for _, table := range tables {
		if _, err := db.ExecContext(ctx, "TRUNCATE TABLE "+table+" CASCADE"); err != nil {
			t.Fatalf("failed to truncate %s table: %v", table, err)
		}
	}
}

// setupHeatmapTestData inserts required users, chats, and messages for heatmap testing
func setupHeatmapTestData(t *testing.T, ctx context.Context, tx *sql.Tx, chatID int64) {
	t.Helper()

	// Insert chat
	chatRepo := NewChatRepository(nil, nil)
	err := chatRepo.UpsertChat(ctx, tx, &models.Chat{
		ID:    chatID,
		Type:  "supergroup",
		Title: "Test Chat",
	})
	if err != nil {
		t.Fatalf("failed to setup chat: %v", err)
	}

	// Insert users (2 users for testing unique_users aggregation)
	userRepo := NewUserRepository(nil, nil)
	for i := 1; i <= 2; i++ {
		err := userRepo.UpsertUser(ctx, tx, &models.User{
			ID:        int64(100 + i),
			FirstName: "User",
			LastName:  string(rune('A' + i - 1)),
			IsBot:     false,
		})
		if err != nil {
			t.Fatalf("failed to setup user %d: %v", i, err)
		}
	}
}

// TestGetGroupHeatmapEmpty tests GetGroupHeatmap with no messages
func TestGetGroupHeatmapEmpty(t *testing.T) {
	db := setupHeatmapTestDB(t)
	defer teardownHeatmapTestDB(t, db)

	repo := NewHeatmapRepository(db, nil)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	cleanupHeatmapTables(t, db)
	chatID := int64(-1001234567890)
	setupHeatmapTestData(t, ctx, tx, chatID)

	// Commit setup transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit setup: %v", err)
	}

	// Get heatmap for chat with no messages
	heatmap, err := repo.GetGroupHeatmap(ctx, chatID, nil, nil, nil)
	if err != nil {
		t.Fatalf("GetGroupHeatmap failed: %v", err)
	}

	// Verify empty result
	if heatmap == nil {
		t.Fatal("expected non-nil heatmap")
	}
	if len(heatmap.Data) != 0 {
		t.Errorf("expected 0 cells, got %d", len(heatmap.Data))
	}
	if heatmap.TotalMessages != 0 {
		t.Errorf("expected TotalMessages=0, got %d", heatmap.TotalMessages)
	}
	if heatmap.MaxCount != 0 {
		t.Errorf("expected MaxCount=0, got %d", heatmap.MaxCount)
	}
}

// TestGetGroupHeatmapAllTime tests GetGroupHeatmap with all-time data (no date filtering)
func TestGetGroupHeatmapAllTime(t *testing.T) {
	db := setupHeatmapTestDB(t)
	defer teardownHeatmapTestDB(t, db)

	repo := NewHeatmapRepository(db, nil)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	cleanupHeatmapTables(t, db)
	chatID := int64(-1001234567890)
	setupHeatmapTestData(t, ctx, tx, chatID)

	// Insert messages at different times (UTC)
	msgRepo := NewMessageRepository(nil, nil)

	// Monday 2025-01-13 10:00 UTC (3 messages from 2 users)
	monday10am := time.Date(2025, 1, 13, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 2; i++ {
		msg := &models.Message{
			MessageID: int64(100 + i),
			Chat: models.Chat{
				ID: chatID,
			},
			From: &models.User{
				ID: 101,
			},
			Date: monday10am.Unix(),
			Text: "Monday 10am message",
		}
		_, err := msgRepo.InsertMessage(ctx, tx, msg)
		if err != nil {
			t.Fatalf("failed to insert message: %v", err)
		}
	}
	msg := &models.Message{
		MessageID: 102,
		Chat: models.Chat{
			ID: chatID,
		},
		From: &models.User{
			ID: 102,
		},
		Date: monday10am.Unix(),
		Text: "Monday 10am message from user 2",
	}
	_, err = msgRepo.InsertMessage(ctx, tx, msg)
	if err != nil {
		t.Fatalf("failed to insert message: %v", err)
	}

	// Tuesday 2025-01-14 15:00 UTC (1 message)
	tuesday3pm := time.Date(2025, 1, 14, 15, 0, 0, 0, time.UTC)
	msg = &models.Message{
		MessageID: 103,
		Chat: models.Chat{
			ID: chatID,
		},
		From: &models.User{
			ID: 101,
		},
		Date: tuesday3pm.Unix(),
		Text: "Tuesday 3pm message",
	}
	_, err = msgRepo.InsertMessage(ctx, tx, msg)
	if err != nil {
		t.Fatalf("failed to insert message: %v", err)
	}

	// Commit setup transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit setup: %v", err)
	}

	// Get all-time heatmap (nil timezone defaults to UTC)
	heatmap, err := repo.GetGroupHeatmap(ctx, chatID, nil, nil, nil)
	if err != nil {
		t.Fatalf("GetGroupHeatmap failed: %v", err)
	}

	// Verify result
	if heatmap == nil {
		t.Fatal("expected non-nil heatmap")
	}
	if len(heatmap.Data) != 2 {
		t.Fatalf("expected 2 cells, got %d", len(heatmap.Data))
	}
	if heatmap.TotalMessages != 4 {
		t.Errorf("expected TotalMessages=4, got %d", heatmap.TotalMessages)
	}
	if heatmap.MaxCount != 3 {
		t.Errorf("expected MaxCount=3, got %d", heatmap.MaxCount)
	}

	// Verify cell 1: Monday (DOW=1) at hour 10
	cell1 := heatmap.Data[0]
	if cell1.DayOfWeek != 1 {
		t.Errorf("cell 1: expected DayOfWeek=1 (Monday), got %d", cell1.DayOfWeek)
	}
	if cell1.Hour != 10 {
		t.Errorf("cell 1: expected Hour=10, got %d", cell1.Hour)
	}
	if cell1.MessageCount != 3 {
		t.Errorf("cell 1: expected MessageCount=3, got %d", cell1.MessageCount)
	}
	if cell1.UniqueUsers != 2 {
		t.Errorf("cell 1: expected UniqueUsers=2, got %d", cell1.UniqueUsers)
	}

	// Verify cell 2: Tuesday (DOW=2) at hour 15
	cell2 := heatmap.Data[1]
	if cell2.DayOfWeek != 2 {
		t.Errorf("cell 2: expected DayOfWeek=2 (Tuesday), got %d", cell2.DayOfWeek)
	}
	if cell2.Hour != 15 {
		t.Errorf("cell 2: expected Hour=15, got %d", cell2.Hour)
	}
	if cell2.MessageCount != 1 {
		t.Errorf("cell 2: expected MessageCount=1, got %d", cell2.MessageCount)
	}
	if cell2.UniqueUsers != 1 {
		t.Errorf("cell 2: expected UniqueUsers=1, got %d", cell2.UniqueUsers)
	}
}

// TestGetGroupHeatmapDateFiltered tests GetGroupHeatmap with date range filtering
func TestGetGroupHeatmapDateFiltered(t *testing.T) {
	db := setupHeatmapTestDB(t)
	defer teardownHeatmapTestDB(t, db)

	repo := NewHeatmapRepository(db, nil)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	cleanupHeatmapTables(t, db)
	chatID := int64(-1001234567890)
	setupHeatmapTestData(t, ctx, tx, chatID)

	// Insert messages at different dates
	msgRepo := NewMessageRepository(nil, nil)

	// Message before date range (2025-01-10)
	beforeRange := time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC)
	_, err = msgRepo.InsertMessage(ctx, tx, &models.Message{
		MessageID: 100,
		Chat: models.Chat{
			ID: chatID,
		},
		From: &models.User{
			ID: 101,
		},
		Date: beforeRange.Unix(),
		Text: "Before range",
	})
	if err != nil {
		t.Fatalf("failed to insert message: %v", err)
	}

	// Message within date range (2025-01-13)
	withinRange := time.Date(2025, 1, 13, 14, 0, 0, 0, time.UTC)
	_, err = msgRepo.InsertMessage(ctx, tx, &models.Message{
		MessageID: 101,
		Chat: models.Chat{
			ID: chatID,
		},
		From: &models.User{
			ID: 101,
		},
		Date: withinRange.Unix(),
		Text: "Within range",
	})
	if err != nil {
		t.Fatalf("failed to insert message: %v", err)
	}

	// Message after date range (2025-01-20)
	afterRange := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC)
	_, err = msgRepo.InsertMessage(ctx, tx, &models.Message{
		MessageID: 102,
		Chat: models.Chat{
			ID: chatID,
		},
		From: &models.User{
			ID: 101,
		},
		Date: afterRange.Unix(),
		Text: "After range",
	})
	if err != nil {
		t.Fatalf("failed to insert message: %v", err)
	}

	// Commit setup transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit setup: %v", err)
	}

	// Get heatmap with date filter (2025-01-12 to 2025-01-15)
	startDate := time.Date(2025, 1, 12, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	heatmap, err := repo.GetGroupHeatmap(ctx, chatID, &startDate, &endDate, nil)
	if err != nil {
		t.Fatalf("GetGroupHeatmap failed: %v", err)
	}

	// Verify only the message within range is included
	if len(heatmap.Data) != 1 {
		t.Fatalf("expected 1 cell, got %d", len(heatmap.Data))
	}
	if heatmap.TotalMessages != 1 {
		t.Errorf("expected TotalMessages=1, got %d", heatmap.TotalMessages)
	}
	if heatmap.Data[0].MessageCount != 1 {
		t.Errorf("expected MessageCount=1, got %d", heatmap.Data[0].MessageCount)
	}
}

// TestGetGroupHeatmapTimezoneConversion tests timezone conversion (UTC vs local timezones)
func TestGetGroupHeatmapTimezoneConversion(t *testing.T) {
	db := setupHeatmapTestDB(t)
	defer teardownHeatmapTestDB(t, db)

	repo := NewHeatmapRepository(db, nil)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	cleanupHeatmapTables(t, db)
	chatID := int64(-1001234567890)
	setupHeatmapTestData(t, ctx, tx, chatID)

	// Insert message at 2025-01-13 23:00 UTC (Monday 11pm UTC)
	msgRepo := NewMessageRepository(nil, nil)
	messageTime := time.Date(2025, 1, 13, 23, 0, 0, 0, time.UTC)
	_, err = msgRepo.InsertMessage(ctx, tx, &models.Message{
		MessageID: 100,
		Chat: models.Chat{
			ID: chatID,
		},
		From: &models.User{
			ID: 101,
		},
		Date: messageTime.Unix(),
		Text: "Test timezone conversion",
	})
	if err != nil {
		t.Fatalf("failed to insert message: %v", err)
	}

	// Commit setup transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit setup: %v", err)
	}

	// Test 1: UTC timezone (Monday 23:00)
	heatmapUTC, err := repo.GetGroupHeatmap(ctx, chatID, nil, nil, time.UTC)
	if err != nil {
		t.Fatalf("GetGroupHeatmap (UTC) failed: %v", err)
	}
	if len(heatmapUTC.Data) != 1 {
		t.Fatalf("UTC: expected 1 cell, got %d", len(heatmapUTC.Data))
	}
	if heatmapUTC.Data[0].DayOfWeek != 1 {
		t.Errorf("UTC: expected DayOfWeek=1 (Monday), got %d", heatmapUTC.Data[0].DayOfWeek)
	}
	if heatmapUTC.Data[0].Hour != 23 {
		t.Errorf("UTC: expected Hour=23, got %d", heatmapUTC.Data[0].Hour)
	}

	// Test 2: America/Sao_Paulo timezone (UTC-3, so 23:00 UTC = 20:00 local, still Monday)
	saoPauloLoc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatalf("failed to load Sao Paulo location: %v", err)
	}
	heatmapSP, err := repo.GetGroupHeatmap(ctx, chatID, nil, nil, saoPauloLoc)
	if err != nil {
		t.Fatalf("GetGroupHeatmap (Sao Paulo) failed: %v", err)
	}
	if len(heatmapSP.Data) != 1 {
		t.Fatalf("Sao Paulo: expected 1 cell, got %d", len(heatmapSP.Data))
	}
	if heatmapSP.Data[0].DayOfWeek != 1 {
		t.Errorf("Sao Paulo: expected DayOfWeek=1 (Monday), got %d", heatmapSP.Data[0].DayOfWeek)
	}
	if heatmapSP.Data[0].Hour != 20 {
		t.Errorf("Sao Paulo: expected Hour=20, got %d (23:00 UTC - 3 hours)", heatmapSP.Data[0].Hour)
	}

	// Test 3: Asia/Tokyo timezone (UTC+9, so 23:00 UTC = 08:00+1 day, Tuesday)
	tokyoLoc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("failed to load Tokyo location: %v", err)
	}
	heatmapTokyo, err := repo.GetGroupHeatmap(ctx, chatID, nil, nil, tokyoLoc)
	if err != nil {
		t.Fatalf("GetGroupHeatmap (Tokyo) failed: %v", err)
	}
	if len(heatmapTokyo.Data) != 1 {
		t.Fatalf("Tokyo: expected 1 cell, got %d", len(heatmapTokyo.Data))
	}
	if heatmapTokyo.Data[0].DayOfWeek != 2 {
		t.Errorf("Tokyo: expected DayOfWeek=2 (Tuesday), got %d", heatmapTokyo.Data[0].DayOfWeek)
	}
	if heatmapTokyo.Data[0].Hour != 8 {
		t.Errorf("Tokyo: expected Hour=8, got %d (23:00 UTC + 9 hours = next day 08:00)", heatmapTokyo.Data[0].Hour)
	}
}

// TestGetUserHeatmapEmpty tests GetUserHeatmap with no messages
func TestGetUserHeatmapEmpty(t *testing.T) {
	db := setupHeatmapTestDB(t)
	defer teardownHeatmapTestDB(t, db)

	repo := NewHeatmapRepository(db, nil)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	cleanupHeatmapTables(t, db)
	chatID := int64(-1001234567890)
	setupHeatmapTestData(t, ctx, tx, chatID)

	// Commit setup transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit setup: %v", err)
	}

	// Get heatmap for user with no messages
	heatmap, err := repo.GetUserHeatmap(ctx, chatID, 101, nil, nil, nil)
	if err != nil {
		t.Fatalf("GetUserHeatmap failed: %v", err)
	}

	// Verify empty result
	if heatmap == nil {
		t.Fatal("expected non-nil heatmap")
	}
	if len(heatmap.Data) != 0 {
		t.Errorf("expected 0 cells, got %d", len(heatmap.Data))
	}
	if heatmap.TotalMessages != 0 {
		t.Errorf("expected TotalMessages=0, got %d", heatmap.TotalMessages)
	}
	if heatmap.MaxCount != 0 {
		t.Errorf("expected MaxCount=0, got %d", heatmap.MaxCount)
	}
}

// TestGetUserHeatmapAllTime tests GetUserHeatmap for a specific user with all-time data
func TestGetUserHeatmapAllTime(t *testing.T) {
	db := setupHeatmapTestDB(t)
	defer teardownHeatmapTestDB(t, db)

	repo := NewHeatmapRepository(db, nil)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	cleanupHeatmapTables(t, db)
	chatID := int64(-1001234567890)
	setupHeatmapTestData(t, ctx, tx, chatID)

	// Insert messages from two users
	msgRepo := NewMessageRepository(nil, nil)

	// User 101: 2 messages on Monday 10am
	monday10am := time.Date(2025, 1, 13, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 2; i++ {
		_, err := msgRepo.InsertMessage(ctx, tx, &models.Message{
			MessageID: int64(100 + i),
			Chat: models.Chat{
				ID: chatID,
			},
			From: &models.User{
				ID: 101,
			},
			Date: monday10am.Unix(),
			Text: "User 101 message",
		})
		if err != nil {
			t.Fatalf("failed to insert message: %v", err)
		}
	}

	// User 102: 1 message on Tuesday 3pm (should be filtered out)
	tuesday3pm := time.Date(2025, 1, 14, 15, 0, 0, 0, time.UTC)
	_, err = msgRepo.InsertMessage(ctx, tx, &models.Message{
		MessageID: 102,
		Chat: models.Chat{
			ID: chatID,
		},
		From: &models.User{
			ID: 102,
		},
		Date: tuesday3pm.Unix(),
		Text: "User 102 message",
	})
	if err != nil {
		t.Fatalf("failed to insert message: %v", err)
	}

	// Commit setup transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit setup: %v", err)
	}

	// Get heatmap for user 101 only
	heatmap, err := repo.GetUserHeatmap(ctx, chatID, 101, nil, nil, nil)
	if err != nil {
		t.Fatalf("GetUserHeatmap failed: %v", err)
	}

	// Verify result (should only include user 101's messages)
	if heatmap == nil {
		t.Fatal("expected non-nil heatmap")
	}
	if len(heatmap.Data) != 1 {
		t.Fatalf("expected 1 cell, got %d", len(heatmap.Data))
	}
	if heatmap.TotalMessages != 2 {
		t.Errorf("expected TotalMessages=2, got %d", heatmap.TotalMessages)
	}
	if heatmap.MaxCount != 2 {
		t.Errorf("expected MaxCount=2, got %d", heatmap.MaxCount)
	}

	// Verify cell: Monday (DOW=1) at hour 10
	cell := heatmap.Data[0]
	if cell.DayOfWeek != 1 {
		t.Errorf("expected DayOfWeek=1 (Monday), got %d", cell.DayOfWeek)
	}
	if cell.Hour != 10 {
		t.Errorf("expected Hour=10, got %d", cell.Hour)
	}
	if cell.MessageCount != 2 {
		t.Errorf("expected MessageCount=2, got %d", cell.MessageCount)
	}
	// Note: GetUserHeatmap doesn't populate UniqueUsers field (only for group heatmap)
}

// TestGetUserHeatmapDateFiltered tests GetUserHeatmap with date range filtering
func TestGetUserHeatmapDateFiltered(t *testing.T) {
	db := setupHeatmapTestDB(t)
	defer teardownHeatmapTestDB(t, db)

	repo := NewHeatmapRepository(db, nil)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	cleanupHeatmapTables(t, db)
	chatID := int64(-1001234567890)
	setupHeatmapTestData(t, ctx, tx, chatID)

	// Insert messages at different dates for user 101
	msgRepo := NewMessageRepository(nil, nil)

	// Message before date range
	beforeRange := time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC)
	_, err = msgRepo.InsertMessage(ctx, tx, &models.Message{
		MessageID: 100,
		Chat: models.Chat{
			ID: chatID,
		},
		From: &models.User{
			ID: 101,
		},
		Date: beforeRange.Unix(),
		Text: "Before range",
	})
	if err != nil {
		t.Fatalf("failed to insert message: %v", err)
	}

	// Message within date range
	withinRange := time.Date(2025, 1, 13, 14, 0, 0, 0, time.UTC)
	_, err = msgRepo.InsertMessage(ctx, tx, &models.Message{
		MessageID: 101,
		Chat: models.Chat{
			ID: chatID,
		},
		From: &models.User{
			ID: 101,
		},
		Date: withinRange.Unix(),
		Text: "Within range",
	})
	if err != nil {
		t.Fatalf("failed to insert message: %v", err)
	}

	// Message after date range
	afterRange := time.Date(2025, 1, 20, 10, 0, 0, 0, time.UTC)
	_, err = msgRepo.InsertMessage(ctx, tx, &models.Message{
		MessageID: 102,
		Chat: models.Chat{
			ID: chatID,
		},
		From: &models.User{
			ID: 101,
		},
		Date: afterRange.Unix(),
		Text: "After range",
	})
	if err != nil {
		t.Fatalf("failed to insert message: %v", err)
	}

	// Commit setup transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit setup: %v", err)
	}

	// Get heatmap with date filter (2025-01-12 to 2025-01-15)
	startDate := time.Date(2025, 1, 12, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	heatmap, err := repo.GetUserHeatmap(ctx, chatID, 101, &startDate, &endDate, nil)
	if err != nil {
		t.Fatalf("GetUserHeatmap failed: %v", err)
	}

	// Verify only the message within range is included
	if len(heatmap.Data) != 1 {
		t.Fatalf("expected 1 cell, got %d", len(heatmap.Data))
	}
	if heatmap.TotalMessages != 1 {
		t.Errorf("expected TotalMessages=1, got %d", heatmap.TotalMessages)
	}
	if heatmap.Data[0].MessageCount != 1 {
		t.Errorf("expected MessageCount=1, got %d", heatmap.Data[0].MessageCount)
	}
}

// TestGetUserHeatmapTimezoneConversion tests timezone conversion for user heatmap
func TestGetUserHeatmapTimezoneConversion(t *testing.T) {
	db := setupHeatmapTestDB(t)
	defer teardownHeatmapTestDB(t, db)

	repo := NewHeatmapRepository(db, nil)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	cleanupHeatmapTables(t, db)
	chatID := int64(-1001234567890)
	setupHeatmapTestData(t, ctx, tx, chatID)

	// Insert message at 2025-01-13 23:00 UTC (Monday 11pm UTC)
	msgRepo := NewMessageRepository(nil, nil)
	messageTime := time.Date(2025, 1, 13, 23, 0, 0, 0, time.UTC)
	_, err = msgRepo.InsertMessage(ctx, tx, &models.Message{
		MessageID: 100,
		Chat: models.Chat{
			ID: chatID,
		},
		From: &models.User{
			ID: 101,
		},
		Date: messageTime.Unix(),
		Text: "Test timezone conversion",
	})
	if err != nil {
		t.Fatalf("failed to insert message: %v", err)
	}

	// Commit setup transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit setup: %v", err)
	}

	// Test UTC timezone
	heatmapUTC, err := repo.GetUserHeatmap(ctx, chatID, 101, nil, nil, time.UTC)
	if err != nil {
		t.Fatalf("GetUserHeatmap (UTC) failed: %v", err)
	}
	if heatmapUTC.Data[0].Hour != 23 {
		t.Errorf("UTC: expected Hour=23, got %d", heatmapUTC.Data[0].Hour)
	}

	// Test America/Sao_Paulo timezone (UTC-3, so 23:00 UTC = 20:00 local)
	saoPauloLoc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatalf("failed to load Sao Paulo location: %v", err)
	}
	heatmapSP, err := repo.GetUserHeatmap(ctx, chatID, 101, nil, nil, saoPauloLoc)
	if err != nil {
		t.Fatalf("GetUserHeatmap (Sao Paulo) failed: %v", err)
	}
	if heatmapSP.Data[0].Hour != 20 {
		t.Errorf("Sao Paulo: expected Hour=20, got %d", heatmapSP.Data[0].Hour)
	}
}

// TestHeatmapAggregationByHour tests that messages are correctly aggregated by hour (0-23)
func TestHeatmapAggregationByHour(t *testing.T) {
	db := setupHeatmapTestDB(t)
	defer teardownHeatmapTestDB(t, db)

	repo := NewHeatmapRepository(db, nil)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	cleanupHeatmapTables(t, db)
	chatID := int64(-1001234567890)
	setupHeatmapTestData(t, ctx, tx, chatID)

	// Insert messages at different minutes within the same hour
	msgRepo := NewMessageRepository(nil, nil)
	baseTime := time.Date(2025, 1, 13, 10, 0, 0, 0, time.UTC)

	for i := 0; i < 3; i++ {
		// 10:00, 10:15, 10:45 should all aggregate to hour 10
		messageTime := baseTime.Add(time.Duration(i*15) * time.Minute)
		_, err := msgRepo.InsertMessage(ctx, tx, &models.Message{
			MessageID: int64(100 + i),
			Chat: models.Chat{
				ID: chatID,
			},
			From: &models.User{
				ID: 101,
			},
			Date: messageTime.Unix(),
			Text: "Same hour message",
		})
		if err != nil {
			t.Fatalf("failed to insert message: %v", err)
		}
	}

	// Commit setup transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit setup: %v", err)
	}

	// Get heatmap
	heatmap, err := repo.GetGroupHeatmap(ctx, chatID, nil, nil, nil)
	if err != nil {
		t.Fatalf("GetGroupHeatmap failed: %v", err)
	}

	// Verify all messages aggregated to hour 10
	if len(heatmap.Data) != 1 {
		t.Fatalf("expected 1 cell (hour 10), got %d", len(heatmap.Data))
	}
	if heatmap.Data[0].Hour != 10 {
		t.Errorf("expected Hour=10, got %d", heatmap.Data[0].Hour)
	}
	if heatmap.Data[0].MessageCount != 3 {
		t.Errorf("expected MessageCount=3, got %d", heatmap.Data[0].MessageCount)
	}
}

// TestHeatmapAggregationByDayOfWeek tests that messages are correctly aggregated by day of week (0=Sunday, 6=Saturday)
func TestHeatmapAggregationByDayOfWeek(t *testing.T) {
	db := setupHeatmapTestDB(t)
	defer teardownHeatmapTestDB(t, db)

	repo := NewHeatmapRepository(db, nil)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	cleanupHeatmapTables(t, db)
	chatID := int64(-1001234567890)
	setupHeatmapTestData(t, ctx, tx, chatID)

	// Insert messages on different days of week
	msgRepo := NewMessageRepository(nil, nil)

	// Sunday 2025-01-12 (DOW=0)
	sunday := time.Date(2025, 1, 12, 10, 0, 0, 0, time.UTC)
	_, err = msgRepo.InsertMessage(ctx, tx, &models.Message{
		MessageID: 100,
		Chat: models.Chat{
			ID: chatID,
		},
		From: &models.User{
			ID: 101,
		},
		Date: sunday.Unix(),
		Text: "Sunday message",
	})
	if err != nil {
		t.Fatalf("failed to insert message: %v", err)
	}

	// Monday 2025-01-13 (DOW=1)
	monday := time.Date(2025, 1, 13, 10, 0, 0, 0, time.UTC)
	_, err = msgRepo.InsertMessage(ctx, tx, &models.Message{
		MessageID: 101,
		Chat: models.Chat{
			ID: chatID,
		},
		From: &models.User{
			ID: 101,
		},
		Date: monday.Unix(),
		Text: "Monday message",
	})
	if err != nil {
		t.Fatalf("failed to insert message: %v", err)
	}

	// Saturday 2025-01-18 (DOW=6)
	saturday := time.Date(2025, 1, 18, 10, 0, 0, 0, time.UTC)
	_, err = msgRepo.InsertMessage(ctx, tx, &models.Message{
		MessageID: 102,
		Chat: models.Chat{
			ID: chatID,
		},
		From: &models.User{
			ID: 101,
		},
		Date: saturday.Unix(),
		Text: "Saturday message",
	})
	if err != nil {
		t.Fatalf("failed to insert message: %v", err)
	}

	// Commit setup transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit setup: %v", err)
	}

	// Get heatmap
	heatmap, err := repo.GetGroupHeatmap(ctx, chatID, nil, nil, nil)
	if err != nil {
		t.Fatalf("GetGroupHeatmap failed: %v", err)
	}

	// Verify 3 cells for 3 different days (all at hour 10)
	if len(heatmap.Data) != 3 {
		t.Fatalf("expected 3 cells, got %d", len(heatmap.Data))
	}

	// Verify ordering: should be sorted by day_of_week, hour
	if heatmap.Data[0].DayOfWeek != 0 {
		t.Errorf("cell 0: expected DayOfWeek=0 (Sunday), got %d", heatmap.Data[0].DayOfWeek)
	}
	if heatmap.Data[1].DayOfWeek != 1 {
		t.Errorf("cell 1: expected DayOfWeek=1 (Monday), got %d", heatmap.Data[1].DayOfWeek)
	}
	if heatmap.Data[2].DayOfWeek != 6 {
		t.Errorf("cell 2: expected DayOfWeek=6 (Saturday), got %d", heatmap.Data[2].DayOfWeek)
	}
}
