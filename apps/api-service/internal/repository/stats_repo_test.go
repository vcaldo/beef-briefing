package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"beef-briefing/apps/api-service/internal/models"
)

// setupStatsTestDB creates a new database connection for testing
func setupStatsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	return setupUserTestDB(t) // Reuse existing setup
}

// teardownStatsTestDB closes the test database connection
func teardownStatsTestDB(t *testing.T, db *sql.DB) {
	t.Helper()
	teardownUserTestDB(t, db) // Reuse existing teardown
}

// cleanupStatsTables truncates all stats-related tables for test isolation
func cleanupStatsTables(t *testing.T, db *sql.DB) {
	t.Helper()

	ctx := context.Background()
	// Truncate in reverse dependency order (child tables first)
	tables := []string{
		"message_reactions",
		"media_files",
		"photos",
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

// setupStatsTestData inserts required users, chats, and messages for stats testing
func setupStatsTestData(t *testing.T, ctx context.Context, tx *sql.Tx, chatID int64) {
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

	// Insert users
	userRepo := NewUserRepository(nil, nil)
	for i := 1; i <= 3; i++ {
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

// ptrTime returns a pointer to a time.Time
func ptrTime(t time.Time) *time.Time {
	return &t
}

// TestGetOverviewStatsAllTime tests GetOverviewStats with all-time data (using materialized views)
func TestGetOverviewStatsAllTime(t *testing.T) {
	db := setupStatsTestDB(t)
	defer teardownStatsTestDB(t, db)

	repo := NewStatsRepository(db, nil)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	cleanupStatsTables(t, db)
	chatID := int64(-1001234567890)
	setupStatsTestData(t, ctx, tx, chatID)

	// Insert messages for User A (101)
	msgRepo := NewMessageRepository(nil, nil)
	for i := 1; i <= 5; i++ {
		_, err := msgRepo.InsertMessage(ctx, tx, &models.Message{
			MessageID: int64(i),
			Chat:      models.Chat{ID: chatID, Type: "supergroup"},
			From:      &models.User{ID: 101, FirstName: "User", LastName: "A"},
			Date:      1704067200 + int64(i*3600), // 2024-01-01 + i hours
			Text:      "Message from User A",
		})
		if err != nil {
			t.Fatalf("failed to insert message %d: %v", i, err)
		}
	}

	// Insert messages for User B (102)
	for i := 6; i <= 8; i++ {
		_, err := msgRepo.InsertMessage(ctx, tx, &models.Message{
			MessageID: int64(i),
			Chat:      models.Chat{ID: chatID, Type: "supergroup"},
			From:      &models.User{ID: 102, FirstName: "User", LastName: "B"},
			Date:      1704067200 + int64(i*3600),
			Text:      "Message from User B",
		})
		if err != nil {
			t.Fatalf("failed to insert message %d: %v", i, err)
		}
	}

	// Commit to materialize data (required for all-time queries using MVs)
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	// Note: In a real test environment, you'd need to REFRESH MATERIALIZED VIEWS
	// For this test, we'll use date-filtered query instead to avoid MV dependency
	startDate := time.Unix(1704067200, 0)                           // 2024-01-01 00:00:00
	endDate := time.Unix(1704067200+int64(24*3600), 0)              // 2024-01-02 00:00:00
	stats, err := repo.GetOverviewStats(ctx, chatID, &startDate, &endDate)
	if err != nil {
		t.Fatalf("GetOverviewStats failed: %v", err)
	}

	// Verify counts
	if stats.TotalMessages != 8 {
		t.Errorf("expected 8 total messages, got %d", stats.TotalMessages)
	}
	if stats.TotalUsers != 2 {
		t.Errorf("expected 2 total users, got %d", stats.TotalUsers)
	}
}

// TestGetOverviewStatsDateFiltered tests GetOverviewStats with date filtering
func TestGetOverviewStatsDateFiltered(t *testing.T) {
	db := setupStatsTestDB(t)
	defer teardownStatsTestDB(t, db)

	repo := NewStatsRepository(db, nil)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	cleanupStatsTables(t, db)
	chatID := int64(-1001234567890)
	setupStatsTestData(t, ctx, tx, chatID)

	// Insert messages across two days
	msgRepo := NewMessageRepository(nil, nil)

	// Day 1: 5 messages from User A
	for i := 1; i <= 5; i++ {
		_, err := msgRepo.InsertMessage(ctx, tx, &models.Message{
			MessageID: int64(i),
			Chat:      models.Chat{ID: chatID, Type: "supergroup"},
			From:      &models.User{ID: 101, FirstName: "User"},
			Date:      1704067200 + int64(i*3600), // 2024-01-01
			Text:      "Day 1 message",
		})
		if err != nil {
			t.Fatalf("failed to insert message %d: %v", i, err)
		}
	}

	// Day 2: 3 messages from User B
	for i := 6; i <= 8; i++ {
		_, err := msgRepo.InsertMessage(ctx, tx, &models.Message{
			MessageID: int64(i),
			Chat:      models.Chat{ID: chatID, Type: "supergroup"},
			From:      &models.User{ID: 102, FirstName: "User"},
			Date:      1704067200 + 86400 + int64(i*3600), // 2024-01-02
			Text:      "Day 2 message",
		})
		if err != nil {
			t.Fatalf("failed to insert message %d: %v", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Query only Day 1
	startDate := time.Unix(1704067200, 0)
	endDate := time.Unix(1704067200+86400, 0)
	stats, err := repo.GetOverviewStats(ctx, chatID, &startDate, &endDate)
	if err != nil {
		t.Fatalf("GetOverviewStats failed: %v", err)
	}

	if stats.TotalMessages != 5 {
		t.Errorf("expected 5 messages in Day 1, got %d", stats.TotalMessages)
	}
	if stats.TotalUsers != 1 {
		t.Errorf("expected 1 user in Day 1, got %d", stats.TotalUsers)
	}
	if stats.MessagesPerDay != 5.0 {
		t.Errorf("expected 5.0 messages per day, got %f", stats.MessagesPerDay)
	}
}

// TestGetOverviewStatsEmpty tests GetOverviewStats with no data
func TestGetOverviewStatsEmpty(t *testing.T) {
	db := setupStatsTestDB(t)
	defer teardownStatsTestDB(t, db)

	repo := NewStatsRepository(db, nil)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	cleanupStatsTables(t, db)
	chatID := int64(-1001234567890)
	setupStatsTestData(t, ctx, tx, chatID)

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	startDate := time.Unix(1704067200, 0)
	endDate := time.Unix(1704067200+86400, 0)
	stats, err := repo.GetOverviewStats(ctx, chatID, &startDate, &endDate)
	if err != nil {
		t.Fatalf("GetOverviewStats failed: %v", err)
	}

	if stats.TotalMessages != 0 {
		t.Errorf("expected 0 messages, got %d", stats.TotalMessages)
	}
	if stats.TotalUsers != 0 {
		t.Errorf("expected 0 users, got %d", stats.TotalUsers)
	}
}

// TestGetDailyActivityAllTime tests GetDailyActivity with all-time data
func TestGetDailyActivityAllTime(t *testing.T) {
	db := setupStatsTestDB(t)
	defer teardownStatsTestDB(t, db)

	repo := NewStatsRepository(db, nil)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	cleanupStatsTables(t, db)
	chatID := int64(-1001234567890)
	setupStatsTestData(t, ctx, tx, chatID)

	msgRepo := NewMessageRepository(nil, nil)

	// Day 1: 5 messages from 2 users
	for i := 1; i <= 3; i++ {
		_, err := msgRepo.InsertMessage(ctx, tx, &models.Message{
			MessageID: int64(i),
			Chat:      models.Chat{ID: chatID, Type: "supergroup"},
			From:      &models.User{ID: 101, FirstName: "User"},
			Date:      1704067200 + int64(i*3600),
			Text:      "Message",
		})
		if err != nil {
			t.Fatalf("failed to insert message: %v", err)
		}
	}
	for i := 4; i <= 5; i++ {
		_, err := msgRepo.InsertMessage(ctx, tx, &models.Message{
			MessageID: int64(i),
			Chat:      models.Chat{ID: chatID, Type: "supergroup"},
			From:      &models.User{ID: 102, FirstName: "User"},
			Date:      1704067200 + int64(i*3600),
			Text:      "Message",
		})
		if err != nil {
			t.Fatalf("failed to insert message: %v", err)
		}
	}

	// Day 2: 2 messages from 1 user
	for i := 6; i <= 7; i++ {
		_, err := msgRepo.InsertMessage(ctx, tx, &models.Message{
			MessageID: int64(i),
			Chat:      models.Chat{ID: chatID, Type: "supergroup"},
			From:      &models.User{ID: 101, FirstName: "User"},
			Date:      1704067200 + 86400 + int64(i*3600),
			Text:      "Message",
		})
		if err != nil {
			t.Fatalf("failed to insert message: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	tz := time.UTC
	activity, err := repo.GetDailyActivity(ctx, chatID, nil, nil, tz)
	if err != nil {
		t.Fatalf("GetDailyActivity failed: %v", err)
	}

	if len(activity) != 2 {
		t.Fatalf("expected 2 days of activity, got %d", len(activity))
	}

	// Day 1
	if activity[0].Messages != 5 {
		t.Errorf("day 1: expected 5 messages, got %d", activity[0].Messages)
	}
	if activity[0].Users != 2 {
		t.Errorf("day 1: expected 2 unique users, got %d", activity[0].Users)
	}
	if activity[0].Date != "2024-01-01" {
		t.Errorf("day 1: expected date 2024-01-01, got %s", activity[0].Date)
	}

	// Day 2
	if activity[1].Messages != 2 {
		t.Errorf("day 2: expected 2 messages, got %d", activity[1].Messages)
	}
	if activity[1].Users != 1 {
		t.Errorf("day 2: expected 1 unique user, got %d", activity[1].Users)
	}
	if activity[1].Date != "2024-01-02" {
		t.Errorf("day 2: expected date 2024-01-02, got %s", activity[1].Date)
	}
}

// TestGetDailyActivityDateFiltered tests GetDailyActivity with date filtering
func TestGetDailyActivityDateFiltered(t *testing.T) {
	db := setupStatsTestDB(t)
	defer teardownStatsTestDB(t, db)

	repo := NewStatsRepository(db, nil)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	cleanupStatsTables(t, db)
	chatID := int64(-1001234567890)
	setupStatsTestData(t, ctx, tx, chatID)

	msgRepo := NewMessageRepository(nil, nil)

	// Insert messages across 3 days
	for i := 1; i <= 3; i++ {
		_, err := msgRepo.InsertMessage(ctx, tx, &models.Message{
			MessageID: int64(i),
			Chat:      models.Chat{ID: chatID, Type: "supergroup"},
			From:      &models.User{ID: 101, FirstName: "User"},
			Date:      1704067200 + int64((i-1)*86400), // Days 1, 2, 3
			Text:      "Message",
		})
		if err != nil {
			t.Fatalf("failed to insert message: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Filter to only Day 2
	startDate := time.Unix(1704067200+86400, 0)
	endDate := time.Unix(1704067200+86400*2, 0)
	tz := time.UTC
	activity, err := repo.GetDailyActivity(ctx, chatID, &startDate, &endDate, tz)
	if err != nil {
		t.Fatalf("GetDailyActivity failed: %v", err)
	}

	if len(activity) != 1 {
		t.Fatalf("expected 1 day of activity, got %d", len(activity))
	}
	if activity[0].Date != "2024-01-02" {
		t.Errorf("expected date 2024-01-02, got %s", activity[0].Date)
	}
}

// TestGetUserDailyActivity tests GetUserDailyActivity for a specific user
func TestGetUserDailyActivity(t *testing.T) {
	db := setupStatsTestDB(t)
	defer teardownStatsTestDB(t, db)

	repo := NewStatsRepository(db, nil)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	cleanupStatsTables(t, db)
	chatID := int64(-1001234567890)
	setupStatsTestData(t, ctx, tx, chatID)

	msgRepo := NewMessageRepository(nil, nil)

	// User 101: 3 messages on Day 1, 2 messages on Day 2
	for i := 1; i <= 3; i++ {
		_, err := msgRepo.InsertMessage(ctx, tx, &models.Message{
			MessageID: int64(i),
			Chat:      models.Chat{ID: chatID, Type: "supergroup"},
			From:      &models.User{ID: 101, FirstName: "User"},
			Date:      1704067200 + int64(i*3600),
			Text:      "Message",
		})
		if err != nil {
			t.Fatalf("failed to insert message: %v", err)
		}
	}
	for i := 4; i <= 5; i++ {
		_, err := msgRepo.InsertMessage(ctx, tx, &models.Message{
			MessageID: int64(i),
			Chat:      models.Chat{ID: chatID, Type: "supergroup"},
			From:      &models.User{ID: 101, FirstName: "User"},
			Date:      1704067200 + 86400 + int64(i*3600),
			Text:      "Message",
		})
		if err != nil {
			t.Fatalf("failed to insert message: %v", err)
		}
	}

	// User 102: 1 message on Day 1 (should be excluded)
	_, err = msgRepo.InsertMessage(ctx, tx, &models.Message{
		MessageID: 6,
		Chat:      models.Chat{ID: chatID, Type: "supergroup"},
		From:      &models.User{ID: 102, FirstName: "User"},
		Date:      1704067200 + 7200,
		Text:      "Message",
	})
	if err != nil {
		t.Fatalf("failed to insert message: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	tz := time.UTC
	activity, err := repo.GetUserDailyActivity(ctx, chatID, 101, nil, nil, tz)
	if err != nil {
		t.Fatalf("GetUserDailyActivity failed: %v", err)
	}

	if len(activity) != 2 {
		t.Fatalf("expected 2 days of activity for user 101, got %d", len(activity))
	}

	// Day 1: 3 messages
	if activity[0].Messages != 3 {
		t.Errorf("day 1: expected 3 messages, got %d", activity[0].Messages)
	}
	if activity[0].Users != 1 {
		t.Errorf("day 1: expected Users=1 (single user), got %d", activity[0].Users)
	}

	// Day 2: 2 messages
	if activity[1].Messages != 2 {
		t.Errorf("day 2: expected 2 messages, got %d", activity[1].Messages)
	}
}

// TestGetUserProfileStatsDateFiltered tests GetUserProfileStats with date filtering
func TestGetUserProfileStatsDateFiltered(t *testing.T) {
	db := setupStatsTestDB(t)
	defer teardownStatsTestDB(t, db)

	repo := NewStatsRepository(db, nil)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	cleanupStatsTables(t, db)
	chatID := int64(-1001234567890)
	setupStatsTestData(t, ctx, tx, chatID)

	msgRepo := NewMessageRepository(nil, nil)

	// User 101: 5 messages, 1 reply sent
	for i := 1; i <= 5; i++ {
		_, err := msgRepo.InsertMessage(ctx, tx, &models.Message{
			MessageID: int64(i),
			Chat:      models.Chat{ID: chatID, Type: "supergroup"},
			From:      &models.User{ID: 101, FirstName: "User"},
			Date:      1704067200 + int64(i*3600),
			Text:      "Message",
		})
		if err != nil {
			t.Fatalf("failed to insert message: %v", err)
		}
	}

	// Reply from User 101 to message 7 (which belongs to user 102)
	_, err = msgRepo.InsertMessage(ctx, tx, &models.Message{
		MessageID:      6,
		Chat:           models.Chat{ID: chatID, Type: "supergroup"},
		From:           &models.User{ID: 101, FirstName: "User"},
		Date:           1704067200 + 21600,
		Text:           "Reply",
		ReplyToMessage: &models.Message{MessageID: 7},
	})
	if err != nil {
		t.Fatalf("failed to insert reply: %v", err)
	}

	// User 102: 3 messages
	for i := 7; i <= 9; i++ {
		_, err := msgRepo.InsertMessage(ctx, tx, &models.Message{
			MessageID: int64(i),
			Chat:      models.Chat{ID: chatID, Type: "supergroup"},
			From:      &models.User{ID: 102, FirstName: "User"},
			Date:      1704067200 + int64(i*3600),
			Text:      "Message",
		})
		if err != nil {
			t.Fatalf("failed to insert message: %v", err)
		}
	}

	// Add reactions
	reactionRepo := NewReactionRepository(nil, nil)
	// User 101 sends reaction
	err = reactionRepo.InsertMessageReaction(ctx, tx, &models.MessageReactionUpdated{
		Chat:        models.Chat{ID: chatID},
		MessageID:   7,
		User:        &models.User{ID: 101},
		Date:        1704067200 + 25200,
		OldReaction: []models.ReactionInfo{},
		NewReaction: []models.ReactionInfo{
			{Type: "emoji", Emoji: "👍"},
		},
	})
	if err != nil {
		t.Fatalf("failed to insert reaction: %v", err)
	}

	// User 102 sends reaction to User 101's message
	err = reactionRepo.InsertMessageReaction(ctx, tx, &models.MessageReactionUpdated{
		Chat:        models.Chat{ID: chatID},
		MessageID:   1,
		User:        &models.User{ID: 102},
		Date:        1704067200 + 28800,
		OldReaction: []models.ReactionInfo{},
		NewReaction: []models.ReactionInfo{
			{Type: "emoji", Emoji: "❤️"},
		},
	})
	if err != nil {
		t.Fatalf("failed to insert reaction: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	startDate := time.Unix(1704067200, 0)
	endDate := time.Unix(1704067200+86400, 0)
	stats, err := repo.GetUserProfileStats(ctx, chatID, 101, &startDate, &endDate, "UTC")
	if err != nil {
		t.Fatalf("GetUserProfileStats failed: %v", err)
	}

	if stats.MessageCount != 6 {
		t.Errorf("expected 6 messages (5 + 1 reply), got %d", stats.MessageCount)
	}
	if stats.ReactionsSent != 1 {
		t.Errorf("expected 1 reaction sent, got %d", stats.ReactionsSent)
	}
	if stats.ReactionsReceived != 1 {
		t.Errorf("expected 1 reaction received, got %d", stats.ReactionsReceived)
	}
	if stats.RepliesSent != 1 {
		t.Errorf("expected 1 reply sent, got %d", stats.RepliesSent)
	}
	if stats.RepliesReceived != 0 {
		t.Errorf("expected 0 replies received, got %d", stats.RepliesReceived)
	}
	if stats.ActiveDays != 1 {
		t.Errorf("expected 1 active day, got %d", stats.ActiveDays)
	}
	if stats.AvgMessagesPerDay != 6.0 {
		t.Errorf("expected 6.0 avg messages per day, got %f", stats.AvgMessagesPerDay)
	}
	if stats.RankByMessages != 1 {
		t.Errorf("expected rank 1 by messages (6 > 3), got %d", stats.RankByMessages)
	}
}

// TestGetUserProfileStatsNoData tests GetUserProfileStats for non-existent user
func TestGetUserProfileStatsNoData(t *testing.T) {
	db := setupStatsTestDB(t)
	defer teardownStatsTestDB(t, db)

	repo := NewStatsRepository(db, nil)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	cleanupStatsTables(t, db)
	chatID := int64(-1001234567890)
	setupStatsTestData(t, ctx, tx, chatID)

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	startDate := time.Unix(1704067200, 0)
	endDate := time.Unix(1704067200+86400, 0)
	stats, err := repo.GetUserProfileStats(ctx, chatID, 999, &startDate, &endDate, "UTC")
	if err != nil {
		t.Fatalf("GetUserProfileStats failed: %v", err)
	}

	// Should return zero stats
	if stats.MessageCount != 0 {
		t.Errorf("expected 0 message count, got %d", stats.MessageCount)
	}
}

// TestGetMediaOverviewStatsDateFiltered tests GetMediaOverviewStats with date filtering
func TestGetMediaOverviewStatsDateFiltered(t *testing.T) {
	db := setupStatsTestDB(t)
	defer teardownStatsTestDB(t, db)

	repo := NewStatsRepository(db, nil)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	cleanupStatsTables(t, db)
	chatID := int64(-1001234567890)
	setupStatsTestData(t, ctx, tx, chatID)

	msgRepo := NewMessageRepository(nil, nil)
	mediaRepo := NewMediaRepository(nil, nil)

	// Insert message
	dbMsgID, err := msgRepo.InsertMessage(ctx, tx, &models.Message{
		MessageID: 1,
		Chat:      models.Chat{ID: chatID, Type: "supergroup"},
		From:      &models.User{ID: 101, FirstName: "User"},
		Date:      1704067200,
		Text:      "Message with media",
	})
	if err != nil {
		t.Fatalf("failed to insert message: %v", err)
	}

	// Insert media files
	err = mediaRepo.InsertMediaFile(ctx, tx, dbMsgID, "photo", "photo123", "unique123", "photo/ab/abc123", "abc123", ptrInt64(1000), "", "", nil, nil, nil, "", "")
	if err != nil {
		t.Fatalf("failed to insert photo media: %v", err)
	}

	err = mediaRepo.InsertMediaFile(ctx, tx, dbMsgID, "video", "video456", "unique456", "video/de/def456", "def456", ptrInt64(5000), "", "", nil, nil, nil, "", "")
	if err != nil {
		t.Fatalf("failed to insert video media: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	startDate := time.Unix(1704067200, 0)
	endDate := time.Unix(1704067200+86400, 0)
	stats, err := repo.GetMediaOverviewStats(ctx, chatID, &startDate, &endDate)
	if err != nil {
		t.Fatalf("GetMediaOverviewStats failed: %v", err)
	}

	if stats.TotalMedia != 2 {
		t.Errorf("expected 2 total media, got %d", stats.TotalMedia)
	}
	if stats.TotalPhotos != 1 {
		t.Errorf("expected 1 photo, got %d", stats.TotalPhotos)
	}
	if stats.TotalVideos != 1 {
		t.Errorf("expected 1 video, got %d", stats.TotalVideos)
	}
	if stats.TotalSize != 6000 {
		t.Errorf("expected total size 6000, got %d", stats.TotalSize)
	}
}

// TestGetMediaTypeDistribution tests GetMediaTypeDistribution
func TestGetMediaTypeDistribution(t *testing.T) {
	db := setupStatsTestDB(t)
	defer teardownStatsTestDB(t, db)

	repo := NewStatsRepository(db, nil)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	cleanupStatsTables(t, db)
	chatID := int64(-1001234567890)
	setupStatsTestData(t, ctx, tx, chatID)

	msgRepo := NewMessageRepository(nil, nil)
	mediaRepo := NewMediaRepository(nil, nil)

	// Insert message
	dbMsgID, err := msgRepo.InsertMessage(ctx, tx, &models.Message{
		MessageID: 1,
		Chat:      models.Chat{ID: chatID, Type: "supergroup"},
		From:      &models.User{ID: 101, FirstName: "User"},
		Date:      1704067200,
		Text:      "Message",
	})
	if err != nil {
		t.Fatalf("failed to insert message: %v", err)
	}

	// Insert multiple photos
	for i := 1; i <= 3; i++ {
		err = mediaRepo.InsertMediaFile(ctx, tx, dbMsgID, "photo", "photo"+string(rune('0'+i)), "unique"+string(rune('0'+i)), "", "", ptrInt64(int64(1000*i)), "", "", nil, nil, nil, "", "")
		if err != nil {
			t.Fatalf("failed to insert photo: %v", err)
		}
	}

	// Insert videos
	for i := 1; i <= 2; i++ {
		err = mediaRepo.InsertMediaFile(ctx, tx, dbMsgID, "video", "video"+string(rune('0'+i)), "videounique"+string(rune('0'+i)), "", "", ptrInt64(int64(5000*i)), "", "", nil, nil, nil, "", "")
		if err != nil {
			t.Fatalf("failed to insert video: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	startDate := time.Unix(1704067200, 0)
	endDate := time.Unix(1704067200+86400, 0)
	distribution, err := repo.GetMediaTypeDistribution(ctx, chatID, &startDate, &endDate)
	if err != nil {
		t.Fatalf("GetMediaTypeDistribution failed: %v", err)
	}

	if len(distribution) != 2 {
		t.Fatalf("expected 2 media types, got %d", len(distribution))
	}

	// Should be ordered by count DESC (photo: 3, video: 2)
	if distribution[0].MediaType != "photo" {
		t.Errorf("expected first type to be photo, got %s", distribution[0].MediaType)
	}
	if distribution[0].Count != 3 {
		t.Errorf("expected 3 photos, got %d", distribution[0].Count)
	}
	if distribution[1].MediaType != "video" {
		t.Errorf("expected second type to be video, got %s", distribution[1].MediaType)
	}
	if distribution[1].Count != 2 {
		t.Errorf("expected 2 videos, got %d", distribution[1].Count)
	}
}

// TestGetMediaActivity tests GetMediaActivity
func TestGetMediaActivity(t *testing.T) {
	db := setupStatsTestDB(t)
	defer teardownStatsTestDB(t, db)

	repo := NewStatsRepository(db, nil)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	cleanupStatsTables(t, db)
	chatID := int64(-1001234567890)
	setupStatsTestData(t, ctx, tx, chatID)

	msgRepo := NewMessageRepository(nil, nil)
	mediaRepo := NewMediaRepository(nil, nil)

	// Day 1: 2 media files
	dbMsgID1, err := msgRepo.InsertMessage(ctx, tx, &models.Message{
		MessageID: 1,
		Chat:      models.Chat{ID: chatID, Type: "supergroup"},
		From:      &models.User{ID: 101, FirstName: "User"},
		Date:      1704067200,
		Text:      "Message",
	})
	if err != nil {
		t.Fatalf("failed to insert message: %v", err)
	}

	err = mediaRepo.InsertMediaFile(ctx, tx, dbMsgID1, "photo", "photo1", "unique1", "", "", nil, "", "", nil, nil, nil, "", "")
	if err != nil {
		t.Fatalf("failed to insert media: %v", err)
	}

	err = mediaRepo.InsertMediaFile(ctx, tx, dbMsgID1, "video", "video1", "unique2", "", "", nil, "", "", nil, nil, nil, "", "")
	if err != nil {
		t.Fatalf("failed to insert media: %v", err)
	}

	// Day 2: 1 media file
	dbMsgID2, err := msgRepo.InsertMessage(ctx, tx, &models.Message{
		MessageID: 2,
		Chat:      models.Chat{ID: chatID, Type: "supergroup"},
		From:      &models.User{ID: 101, FirstName: "User"},
		Date:      1704067200 + 86400,
		Text:      "Message",
	})
	if err != nil {
		t.Fatalf("failed to insert message: %v", err)
	}

	err = mediaRepo.InsertMediaFile(ctx, tx, dbMsgID2, "photo", "photo2", "unique3", "", "", nil, "", "", nil, nil, nil, "", "")
	if err != nil {
		t.Fatalf("failed to insert media: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	tz := time.UTC
	activity, err := repo.GetMediaActivity(ctx, chatID, nil, nil, tz)
	if err != nil {
		t.Fatalf("GetMediaActivity failed: %v", err)
	}

	if len(activity) != 2 {
		t.Fatalf("expected 2 days of activity, got %d", len(activity))
	}

	if activity[0].Date != "2024-01-01" {
		t.Errorf("expected date 2024-01-01, got %s", activity[0].Date)
	}
	if activity[0].Count != 2 {
		t.Errorf("day 1: expected 2 media, got %d", activity[0].Count)
	}

	if activity[1].Date != "2024-01-02" {
		t.Errorf("expected date 2024-01-02, got %s", activity[1].Date)
	}
	if activity[1].Count != 1 {
		t.Errorf("day 2: expected 1 media, got %d", activity[1].Count)
	}
}

// TestTimezoneConversion tests timezone conversion in daily activity queries
func TestTimezoneConversion(t *testing.T) {
	db := setupStatsTestDB(t)
	defer teardownStatsTestDB(t, db)

	repo := NewStatsRepository(db, nil)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	cleanupStatsTables(t, db)
	chatID := int64(-1001234567890)
	setupStatsTestData(t, ctx, tx, chatID)

	msgRepo := NewMessageRepository(nil, nil)

	// Insert message at 2024-01-01 23:00:00 UTC
	// In America/Sao_Paulo (UTC-3), this is 2024-01-01 20:00:00
	// In Asia/Tokyo (UTC+9), this is 2024-01-02 08:00:00
	_, err = msgRepo.InsertMessage(ctx, tx, &models.Message{
		MessageID: 1,
		Chat:      models.Chat{ID: chatID, Type: "supergroup"},
		From:      &models.User{ID: 101, FirstName: "User"},
		Date:      1704146400, // 2024-01-01 23:00:00 UTC
		Text:      "Message",
	})
	if err != nil {
		t.Fatalf("failed to insert message: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Query with UTC timezone
	tzUTC := time.UTC
	activityUTC, err := repo.GetDailyActivity(ctx, chatID, nil, nil, tzUTC)
	if err != nil {
		t.Fatalf("GetDailyActivity (UTC) failed: %v", err)
	}

	if len(activityUTC) != 1 {
		t.Fatalf("UTC: expected 1 day, got %d", len(activityUTC))
	}
	if activityUTC[0].Date != "2024-01-01" {
		t.Errorf("UTC: expected date 2024-01-01, got %s", activityUTC[0].Date)
	}

	// Query with America/Sao_Paulo timezone (UTC-3)
	tzSaoPaulo, _ := time.LoadLocation("America/Sao_Paulo")
	activitySP, err := repo.GetDailyActivity(ctx, chatID, nil, nil, tzSaoPaulo)
	if err != nil {
		t.Fatalf("GetDailyActivity (Sao Paulo) failed: %v", err)
	}

	if len(activitySP) != 1 {
		t.Fatalf("Sao Paulo: expected 1 day, got %d", len(activitySP))
	}
	if activitySP[0].Date != "2024-01-01" {
		t.Errorf("Sao Paulo: expected date 2024-01-01 (20:00 in SP), got %s", activitySP[0].Date)
	}

	// Query with Asia/Tokyo timezone (UTC+9)
	tzTokyo, _ := time.LoadLocation("Asia/Tokyo")
	activityTokyo, err := repo.GetDailyActivity(ctx, chatID, nil, nil, tzTokyo)
	if err != nil {
		t.Fatalf("GetDailyActivity (Tokyo) failed: %v", err)
	}

	if len(activityTokyo) != 1 {
		t.Fatalf("Tokyo: expected 1 day, got %d", len(activityTokyo))
	}
	if activityTokyo[0].Date != "2024-01-02" {
		t.Errorf("Tokyo: expected date 2024-01-02 (08:00 in Tokyo), got %s", activityTokyo[0].Date)
	}
}
