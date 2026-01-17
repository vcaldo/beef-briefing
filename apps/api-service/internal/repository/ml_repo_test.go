package repository

import (
	"context"
	"database/sql"
	"testing"

	"beef-briefing/apps/api-service/internal/models"
	"github.com/lib/pq"
)

// sqlArray is a type alias for pq.Array to simplify scanning array columns
type sqlArray = pq.StringArray

// setupMLTestDB creates a new database connection for testing
func setupMLTestDB(t *testing.T) *sql.DB {
	t.Helper()
	return setupUserTestDB(t) // Reuse existing setup
}

// teardownMLTestDB closes the test database connection
func teardownMLTestDB(t *testing.T, db *sql.DB) {
	t.Helper()
	teardownUserTestDB(t, db) // Reuse existing teardown
}

// cleanupMLTables truncates all ML-related tables for test isolation
func cleanupMLTables(t *testing.T, db *sql.DB) {
	t.Helper()

	ctx := context.Background()
	// Truncate in reverse dependency order (child tables first)
	tables := []string{
		"ml_message_topics",
		"ml_ner",
		"ml_questions",
		"ml_humor",
		"ml_toxicity",
		"ml_sentiment",
		"ml_processing_state",
		"ml_topics",
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

// setupMLTestMessage inserts required users, chats, and a message for ML testing
// Returns the database message ID (auto-incremented primary key)
func setupMLTestMessage(t *testing.T, ctx context.Context, tx *sql.Tx, chatID, userID, messageID int64, text string) int64 {
	t.Helper()

	// Insert user
	userRepo := NewUserRepository(nil, nil)
	err := userRepo.UpsertUser(ctx, tx, &models.User{
		ID:        userID,
		FirstName: "Test User",
		Username:  "testuser",
	})
	if err != nil {
		t.Fatalf("failed to setup user: %v", err)
	}

	// Insert chat
	chatRepo := NewChatRepository(nil, nil)
	err = chatRepo.UpsertChat(ctx, tx, &models.Chat{
		ID:   chatID,
		Type: "supergroup",
	})
	if err != nil {
		t.Fatalf("failed to setup chat: %v", err)
	}

	// Insert message with text
	msgRepo := NewMessageRepository(nil, nil)
	msg := &models.Message{
		MessageID: messageID,
		Chat: models.Chat{
			ID:   chatID,
			Type: "supergroup",
		},
		From: &models.User{
			ID:        userID,
			FirstName: "Test User",
		},
		Date: 1704067200, // 2024-01-01 00:00:00 UTC
		Text: text,
	}
	dbMessageID, err := msgRepo.InsertMessage(ctx, tx, msg)
	if err != nil {
		t.Fatalf("failed to setup message: %v", err)
	}

	return dbMessageID
}

// TestGetUnprocessedMessagesEmpty tests GetUnprocessedMessages with no messages
func TestGetUnprocessedMessagesEmpty(t *testing.T) {
	db := setupMLTestDB(t)
	defer teardownMLTestDB(t, db)
	defer cleanupMLTables(t, db)

	ctx := context.Background()
	repo := NewMLRepository(db, nil)

	messages, err := repo.GetUnprocessedMessages(ctx, 10)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(messages))
	}
}

// TestGetUnprocessedMessagesSingle tests GetUnprocessedMessages with a single message
func TestGetUnprocessedMessagesSingle(t *testing.T) {
	db := setupMLTestDB(t)
	defer teardownMLTestDB(t, db)
	defer cleanupMLTables(t, db)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	chatID := int64(-1001234567890)
	userID := int64(123456)
	messageID := int64(1)
	text := "This is a test message"

	dbMessageID := setupMLTestMessage(t, ctx, tx, chatID, userID, messageID, text)
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	repo := NewMLRepository(db, nil)
	messages, err := repo.GetUnprocessedMessages(ctx, 10)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	msg := messages[0]
	if msg.ID != dbMessageID {
		t.Errorf("expected ID %d, got %d", dbMessageID, msg.ID)
	}
	if msg.MessageID != messageID {
		t.Errorf("expected MessageID %d, got %d", messageID, msg.MessageID)
	}
	if msg.ChatID != chatID {
		t.Errorf("expected ChatID %d, got %d", chatID, msg.ChatID)
	}
	if msg.UserID == nil || *msg.UserID != userID {
		t.Errorf("expected UserID %d, got %v", userID, msg.UserID)
	}
	if msg.Text != text {
		t.Errorf("expected Text %q, got %q", text, msg.Text)
	}
}

// TestGetUnprocessedMessagesBatch tests GetUnprocessedMessages with limit/offset
func TestGetUnprocessedMessagesBatch(t *testing.T) {
	db := setupMLTestDB(t)
	defer teardownMLTestDB(t, db)
	defer cleanupMLTables(t, db)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	chatID := int64(-1001234567890)
	userID := int64(123456)

	// Insert 5 messages
	for i := int64(1); i <= 5; i++ {
		setupMLTestMessage(t, ctx, tx, chatID, userID, i, "Test message")
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	repo := NewMLRepository(db, nil)

	// Test limit=3
	messages, err := repo.GetUnprocessedMessages(ctx, 3)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(messages) != 3 {
		t.Errorf("expected 3 messages, got %d", len(messages))
	}

	// Verify ordered by date ASC (message_id 1, 2, 3)
	for i := 0; i < 3; i++ {
		if messages[i].MessageID != int64(i+1) {
			t.Errorf("expected MessageID %d, got %d", i+1, messages[i].MessageID)
		}
	}
}

// TestGetUnprocessedMessagesAlreadyProcessed tests that processed messages are excluded
func TestGetUnprocessedMessagesAlreadyProcessed(t *testing.T) {
	db := setupMLTestDB(t)
	defer teardownMLTestDB(t, db)
	defer cleanupMLTables(t, db)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	chatID := int64(-1001234567890)
	userID := int64(123456)

	// Insert 3 messages
	dbMessageID1 := setupMLTestMessage(t, ctx, tx, chatID, userID, 1, "Message 1")
	setupMLTestMessage(t, ctx, tx, chatID, userID, 2, "Message 2")
	setupMLTestMessage(t, ctx, tx, chatID, userID, 3, "Message 3")

	// Mark message 1 as processed
	repo := NewMLRepository(db, nil)
	err = repo.MarkMessagesProcessed(ctx, []int64{dbMessageID1}, []int64{chatID}, "v1.0", tx)
	if err != nil {
		t.Fatalf("failed to mark message as processed: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	// Fetch unprocessed messages (should only get 2 and 3)
	messages, err := repo.GetUnprocessedMessages(ctx, 10)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(messages) != 2 {
		t.Fatalf("expected 2 unprocessed messages, got %d", len(messages))
	}

	// Verify message IDs are 2 and 3
	if messages[0].MessageID != 2 {
		t.Errorf("expected MessageID 2, got %d", messages[0].MessageID)
	}
	if messages[1].MessageID != 3 {
		t.Errorf("expected MessageID 3, got %d", messages[1].MessageID)
	}
}

// TestGetUnprocessedMessagesWithCaption tests that messages with caption (but no text) are included
func TestGetUnprocessedMessagesWithCaption(t *testing.T) {
	db := setupMLTestDB(t)
	defer teardownMLTestDB(t, db)
	defer cleanupMLTables(t, db)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	chatID := int64(-1001234567890)
	userID := int64(123456)
	messageID := int64(1)

	// Insert message with caption instead of text
	userRepo := NewUserRepository(nil, nil)
	err = userRepo.UpsertUser(ctx, tx, &models.User{
		ID:        userID,
		FirstName: "Test User",
		Username:  "testuser",
	})
	if err != nil {
		t.Fatalf("failed to setup user: %v", err)
	}

	chatRepo := NewChatRepository(nil, nil)
	err = chatRepo.UpsertChat(ctx, tx, &models.Chat{
		ID:   chatID,
		Type: "supergroup",
	})
	if err != nil {
		t.Fatalf("failed to setup chat: %v", err)
	}

	msgRepo := NewMessageRepository(nil, nil)
	msg := &models.Message{
		MessageID: messageID,
		Chat: models.Chat{
			ID:   chatID,
			Type: "supergroup",
		},
		From: &models.User{
			ID:        userID,
			FirstName: "Test User",
		},
		Date:    1704067200,
		Caption: "This is a photo caption",
	}
	dbMessageID, err := msgRepo.InsertMessage(ctx, tx, msg)
	if err != nil {
		t.Fatalf("failed to setup message: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	repo := NewMLRepository(db, nil)
	messages, err := repo.GetUnprocessedMessages(ctx, 10)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	// Text should be caption value (COALESCE in query)
	if messages[0].Text != "This is a photo caption" {
		t.Errorf("expected Text to be caption, got %q", messages[0].Text)
	}
	if messages[0].ID != dbMessageID {
		t.Errorf("expected ID %d, got %d", dbMessageID, messages[0].ID)
	}
}

// TestSaveSentimentResultsSingle tests saving a single sentiment result
func TestSaveSentimentResultsSingle(t *testing.T) {
	db := setupMLTestDB(t)
	defer teardownMLTestDB(t, db)
	defer cleanupMLTables(t, db)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	chatID := int64(-1001234567890)
	userID := int64(123456)
	messageID := int64(1)

	dbMessageID := setupMLTestMessage(t, ctx, tx, chatID, userID, messageID, "Test message")
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	repo := NewMLRepository(db, nil)
	result := SentimentResult{
		MessageID:     dbMessageID,
		ChatID:        chatID,
		Label:         "positive",
		ScorePositive: 0.85,
		ScoreNeutral:  0.10,
		ScoreNegative: 0.05,
		Confidence:    0.85,
	}

	err = repo.SaveSentimentResults(ctx, []SentimentResult{result})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify insertion
	var label string
	var scorePositive, scoreNeutral, scoreNegative, confidence float32
	err = db.QueryRowContext(ctx, `
		SELECT label, score_positive, score_neutral, score_negative, confidence
		FROM ml_sentiment
		WHERE message_id = $1
	`, dbMessageID).Scan(&label, &scorePositive, &scoreNeutral, &scoreNegative, &confidence)
	if err != nil {
		t.Fatalf("failed to query sentiment: %v", err)
	}

	if label != "positive" {
		t.Errorf("expected label 'positive', got %q", label)
	}
	if scorePositive != 0.85 {
		t.Errorf("expected scorePositive 0.85, got %f", scorePositive)
	}
	if scoreNeutral != 0.10 {
		t.Errorf("expected scoreNeutral 0.10, got %f", scoreNeutral)
	}
	if scoreNegative != 0.05 {
		t.Errorf("expected scoreNegative 0.05, got %f", scoreNegative)
	}
	if confidence != 0.85 {
		t.Errorf("expected confidence 0.85, got %f", confidence)
	}
}

// TestSaveSentimentResultsBatch tests saving multiple sentiment results
func TestSaveSentimentResultsBatch(t *testing.T) {
	db := setupMLTestDB(t)
	defer teardownMLTestDB(t, db)
	defer cleanupMLTables(t, db)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	chatID := int64(-1001234567890)
	userID := int64(123456)

	dbMessageID1 := setupMLTestMessage(t, ctx, tx, chatID, userID, 1, "Positive message")
	dbMessageID2 := setupMLTestMessage(t, ctx, tx, chatID, userID, 2, "Negative message")
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	repo := NewMLRepository(db, nil)
	results := []SentimentResult{
		{
			MessageID:     dbMessageID1,
			ChatID:        chatID,
			Label:         "positive",
			ScorePositive: 0.90,
			ScoreNeutral:  0.08,
			ScoreNegative: 0.02,
			Confidence:    0.90,
		},
		{
			MessageID:     dbMessageID2,
			ChatID:        chatID,
			Label:         "negative",
			ScorePositive: 0.05,
			ScoreNeutral:  0.15,
			ScoreNegative: 0.80,
			Confidence:    0.80,
		},
	}

	err = repo.SaveSentimentResults(ctx, results)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify count
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ml_sentiment").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count sentiment results: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 sentiment results, got %d", count)
	}
}

// TestSaveSentimentResultsUpsert tests ON CONFLICT upsert behavior
func TestSaveSentimentResultsUpsert(t *testing.T) {
	db := setupMLTestDB(t)
	defer teardownMLTestDB(t, db)
	defer cleanupMLTables(t, db)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	chatID := int64(-1001234567890)
	userID := int64(123456)
	messageID := int64(1)

	dbMessageID := setupMLTestMessage(t, ctx, tx, chatID, userID, messageID, "Test message")
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	repo := NewMLRepository(db, nil)

	// Insert initial sentiment
	result1 := SentimentResult{
		MessageID:     dbMessageID,
		ChatID:        chatID,
		Label:         "positive",
		ScorePositive: 0.70,
		ScoreNeutral:  0.20,
		ScoreNegative: 0.10,
		Confidence:    0.70,
	}
	err = repo.SaveSentimentResults(ctx, []SentimentResult{result1})
	if err != nil {
		t.Fatalf("expected no error on first insert, got: %v", err)
	}

	// Update with new sentiment (ON CONFLICT DO UPDATE)
	result2 := SentimentResult{
		MessageID:     dbMessageID,
		ChatID:        chatID,
		Label:         "neutral",
		ScorePositive: 0.30,
		ScoreNeutral:  0.60,
		ScoreNegative: 0.10,
		Confidence:    0.60,
	}
	err = repo.SaveSentimentResults(ctx, []SentimentResult{result2})
	if err != nil {
		t.Fatalf("expected no error on upsert, got: %v", err)
	}

	// Verify only one row exists with updated values
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ml_sentiment WHERE message_id = $1", dbMessageID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count sentiment results: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 sentiment result, got %d", count)
	}

	var label string
	var confidence float32
	err = db.QueryRowContext(ctx, `
		SELECT label, confidence FROM ml_sentiment WHERE message_id = $1
	`, dbMessageID).Scan(&label, &confidence)
	if err != nil {
		t.Fatalf("failed to query sentiment: %v", err)
	}
	if label != "neutral" {
		t.Errorf("expected label 'neutral', got %q", label)
	}
	if confidence != 0.60 {
		t.Errorf("expected confidence 0.60, got %f", confidence)
	}
}

// TestSaveToxicityResultsSingle tests saving a single toxicity result
func TestSaveToxicityResultsSingle(t *testing.T) {
	db := setupMLTestDB(t)
	defer teardownMLTestDB(t, db)
	defer cleanupMLTables(t, db)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	chatID := int64(-1001234567890)
	userID := int64(123456)
	messageID := int64(1)

	dbMessageID := setupMLTestMessage(t, ctx, tx, chatID, userID, messageID, "Test message")
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	repo := NewMLRepository(db, nil)
	result := ToxicityResult{
		MessageID: dbMessageID,
		ChatID:    chatID,
		IsToxic:   true,
		Label:     "harassment",
		Score:     0.92,
	}

	err = repo.SaveToxicityResults(ctx, []ToxicityResult{result})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify insertion
	var isToxic bool
	var label string
	var score float32
	err = db.QueryRowContext(ctx, `
		SELECT is_toxic, label, score
		FROM ml_toxicity
		WHERE message_id = $1
	`, dbMessageID).Scan(&isToxic, &label, &score)
	if err != nil {
		t.Fatalf("failed to query toxicity: %v", err)
	}

	if !isToxic {
		t.Errorf("expected is_toxic true")
	}
	if label != "harassment" {
		t.Errorf("expected label 'harassment', got %q", label)
	}
	if score != 0.92 {
		t.Errorf("expected score 0.92, got %f", score)
	}
}

// TestSaveToxicityResultsBatch tests saving multiple toxicity results
func TestSaveToxicityResultsBatch(t *testing.T) {
	db := setupMLTestDB(t)
	defer teardownMLTestDB(t, db)
	defer cleanupMLTables(t, db)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	chatID := int64(-1001234567890)
	userID := int64(123456)

	dbMessageID1 := setupMLTestMessage(t, ctx, tx, chatID, userID, 1, "Toxic message")
	dbMessageID2 := setupMLTestMessage(t, ctx, tx, chatID, userID, 2, "Safe message")
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	repo := NewMLRepository(db, nil)
	results := []ToxicityResult{
		{
			MessageID: dbMessageID1,
			ChatID:    chatID,
			IsToxic:   true,
			Label:     "harassment",
			Score:     0.88,
		},
		{
			MessageID: dbMessageID2,
			ChatID:    chatID,
			IsToxic:   false,
			Label:     "safe",
			Score:     0.05,
		},
	}

	err = repo.SaveToxicityResults(ctx, results)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify count
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ml_toxicity").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count toxicity results: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 toxicity results, got %d", count)
	}
}

// TestSaveHumorResultsSingle tests saving a single humor result
func TestSaveHumorResultsSingle(t *testing.T) {
	db := setupMLTestDB(t)
	defer teardownMLTestDB(t, db)
	defer cleanupMLTables(t, db)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	chatID := int64(-1001234567890)
	userID := int64(123456)
	messageID := int64(1)

	dbMessageID := setupMLTestMessage(t, ctx, tx, chatID, userID, messageID, "Funny joke")
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	repo := NewMLRepository(db, nil)
	result := HumorResult{
		MessageID:  dbMessageID,
		ChatID:     chatID,
		IsHumorous: true,
		HumorType:  "wordplay",
		Score:      0.78,
	}

	err = repo.SaveHumorResults(ctx, []HumorResult{result})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify insertion
	var isHumorous bool
	var humorType sql.NullString
	var score float32
	err = db.QueryRowContext(ctx, `
		SELECT is_humorous, humor_type, score
		FROM ml_humor
		WHERE message_id = $1
	`, dbMessageID).Scan(&isHumorous, &humorType, &score)
	if err != nil {
		t.Fatalf("failed to query humor: %v", err)
	}

	if !isHumorous {
		t.Errorf("expected is_humorous true")
	}
	if !humorType.Valid || humorType.String != "wordplay" {
		t.Errorf("expected humor_type 'wordplay', got %v", humorType)
	}
	if score != 0.78 {
		t.Errorf("expected score 0.78, got %f", score)
	}
}

// TestSaveHumorResultsBatch tests saving multiple humor results
func TestSaveHumorResultsBatch(t *testing.T) {
	db := setupMLTestDB(t)
	defer teardownMLTestDB(t, db)
	defer cleanupMLTables(t, db)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	chatID := int64(-1001234567890)
	userID := int64(123456)

	dbMessageID1 := setupMLTestMessage(t, ctx, tx, chatID, userID, 1, "Funny message")
	dbMessageID2 := setupMLTestMessage(t, ctx, tx, chatID, userID, 2, "Serious message")
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	repo := NewMLRepository(db, nil)
	results := []HumorResult{
		{
			MessageID:  dbMessageID1,
			ChatID:     chatID,
			IsHumorous: true,
			HumorType:  "sarcasm",
			Score:      0.82,
		},
		{
			MessageID:  dbMessageID2,
			ChatID:     chatID,
			IsHumorous: false,
			HumorType:  "",
			Score:      0.10,
		},
	}

	err = repo.SaveHumorResults(ctx, results)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify count
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ml_humor").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count humor results: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 humor results, got %d", count)
	}
}

// TestGetProcessingStatsEmpty tests GetProcessingStats with no data
func TestGetProcessingStatsEmpty(t *testing.T) {
	db := setupMLTestDB(t)
	defer teardownMLTestDB(t, db)
	defer cleanupMLTables(t, db)

	ctx := context.Background()
	repo := NewMLRepository(db, nil)

	stats, err := repo.GetProcessingStats(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if stats["total_with_text"] != 0 {
		t.Errorf("expected total_with_text 0, got %d", stats["total_with_text"])
	}
	if stats["processed"] != 0 {
		t.Errorf("expected processed 0, got %d", stats["processed"])
	}
	if stats["unprocessed"] != 0 {
		t.Errorf("expected unprocessed 0, got %d", stats["unprocessed"])
	}
	if stats["sentiment_analyzed"] != 0 {
		t.Errorf("expected sentiment_analyzed 0, got %d", stats["sentiment_analyzed"])
	}
	if stats["toxicity_analyzed"] != 0 {
		t.Errorf("expected toxicity_analyzed 0, got %d", stats["toxicity_analyzed"])
	}
}

// TestGetProcessingStatsWithData tests GetProcessingStats with messages and results
func TestGetProcessingStatsWithData(t *testing.T) {
	db := setupMLTestDB(t)
	defer teardownMLTestDB(t, db)
	defer cleanupMLTables(t, db)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	chatID := int64(-1001234567890)
	userID := int64(123456)

	// Insert 3 messages with text
	dbMessageID1 := setupMLTestMessage(t, ctx, tx, chatID, userID, 1, "Message 1")
	dbMessageID2 := setupMLTestMessage(t, ctx, tx, chatID, userID, 2, "Message 2")
	dbMessageID3 := setupMLTestMessage(t, ctx, tx, chatID, userID, 3, "Message 3")

	repo := NewMLRepository(db, nil)

	// Process 2 messages (sentiment + toxicity)
	err = repo.SaveSentimentResults(ctx, []SentimentResult{
		{MessageID: dbMessageID1, ChatID: chatID, Label: "positive", ScorePositive: 0.9, ScoreNeutral: 0.05, ScoreNegative: 0.05, Confidence: 0.9},
		{MessageID: dbMessageID2, ChatID: chatID, Label: "neutral", ScorePositive: 0.3, ScoreNeutral: 0.6, ScoreNegative: 0.1, Confidence: 0.6},
	}, tx)
	if err != nil {
		t.Fatalf("failed to save sentiment: %v", err)
	}

	err = repo.SaveToxicityResults(ctx, []ToxicityResult{
		{MessageID: dbMessageID1, ChatID: chatID, IsToxic: false, Label: "safe", Score: 0.05},
		{MessageID: dbMessageID2, ChatID: chatID, IsToxic: true, Label: "harassment", Score: 0.88},
	}, tx)
	if err != nil {
		t.Fatalf("failed to save toxicity: %v", err)
	}

	// Mark 2 messages as processed
	err = repo.MarkMessagesProcessed(ctx, []int64{dbMessageID1, dbMessageID2}, []int64{chatID, chatID}, "v1.0", tx)
	if err != nil {
		t.Fatalf("failed to mark processed: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	// Get stats
	stats, err := repo.GetProcessingStats(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify stats
	if stats["total_with_text"] != 3 {
		t.Errorf("expected total_with_text 3, got %d", stats["total_with_text"])
	}
	if stats["processed"] != 2 {
		t.Errorf("expected processed 2, got %d", stats["processed"])
	}
	if stats["unprocessed"] != 1 {
		t.Errorf("expected unprocessed 1, got %d", stats["unprocessed"])
	}
	if stats["sentiment_analyzed"] != 2 {
		t.Errorf("expected sentiment_analyzed 2, got %d", stats["sentiment_analyzed"])
	}
	if stats["toxicity_analyzed"] != 2 {
		t.Errorf("expected toxicity_analyzed 2, got %d", stats["toxicity_analyzed"])
	}
	if stats["toxic_messages"] != 1 {
		t.Errorf("expected toxic_messages 1, got %d", stats["toxic_messages"])
	}

	// Unused analyzer stats should be 0
	_ = dbMessageID3 // Mark as used to avoid compiler warning
}

// TestMarkMessagesProcessedSingle tests marking a single message as processed
func TestMarkMessagesProcessedSingle(t *testing.T) {
	db := setupMLTestDB(t)
	defer teardownMLTestDB(t, db)
	defer cleanupMLTables(t, db)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	chatID := int64(-1001234567890)
	userID := int64(123456)
	messageID := int64(1)

	dbMessageID := setupMLTestMessage(t, ctx, tx, chatID, userID, messageID, "Test message")
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	repo := NewMLRepository(db, nil)
	err = repo.MarkMessagesProcessed(ctx, []int64{dbMessageID}, []int64{chatID}, "v1.0.0")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify insertion
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ml_processing_state WHERE message_id = $1", dbMessageID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query processing state: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 processing state record, got %d", count)
	}

	var version string
	err = db.QueryRowContext(ctx, "SELECT processor_version FROM ml_processing_state WHERE message_id = $1", dbMessageID).Scan(&version)
	if err != nil {
		t.Fatalf("failed to query processor version: %v", err)
	}
	if version != "v1.0.0" {
		t.Errorf("expected version 'v1.0.0', got %q", version)
	}
}

// TestMarkMessagesProcessedBatch tests marking multiple messages as processed
func TestMarkMessagesProcessedBatch(t *testing.T) {
	db := setupMLTestDB(t)
	defer teardownMLTestDB(t, db)
	defer cleanupMLTables(t, db)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	chatID := int64(-1001234567890)
	userID := int64(123456)

	dbMessageID1 := setupMLTestMessage(t, ctx, tx, chatID, userID, 1, "Message 1")
	dbMessageID2 := setupMLTestMessage(t, ctx, tx, chatID, userID, 2, "Message 2")
	dbMessageID3 := setupMLTestMessage(t, ctx, tx, chatID, userID, 3, "Message 3")
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	repo := NewMLRepository(db, nil)
	err = repo.MarkMessagesProcessed(
		ctx,
		[]int64{dbMessageID1, dbMessageID2, dbMessageID3},
		[]int64{chatID, chatID, chatID},
		"v1.2.0",
	)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify count
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ml_processing_state").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count processing state: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 processing state records, got %d", count)
	}
}

// TestSaveQuestionResultsSingle tests saving a single question result
func TestSaveQuestionResultsSingle(t *testing.T) {
	db := setupMLTestDB(t)
	defer teardownMLTestDB(t, db)
	defer cleanupMLTables(t, db)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	chatID := int64(-1001234567890)
	userID := int64(123456)
	messageID := int64(1)

	dbMessageID := setupMLTestMessage(t, ctx, tx, chatID, userID, messageID, "What is this?")
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	repo := NewMLRepository(db, nil)
	result := QuestionResult{
		MessageID:    dbMessageID,
		ChatID:       chatID,
		IsQuestion:   true,
		QuestionType: "what",
		Score:        0.95,
	}

	err = repo.SaveQuestionResults(ctx, []QuestionResult{result})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify insertion
	var isQuestion bool
	var questionType sql.NullString
	var score float32
	err = db.QueryRowContext(ctx, `
		SELECT is_question, question_type, score
		FROM ml_questions
		WHERE message_id = $1
	`, dbMessageID).Scan(&isQuestion, &questionType, &score)
	if err != nil {
		t.Fatalf("failed to query question: %v", err)
	}

	if !isQuestion {
		t.Errorf("expected is_question true")
	}
	if !questionType.Valid || questionType.String != "what" {
		t.Errorf("expected question_type 'what', got %v", questionType)
	}
	if score != 0.95 {
		t.Errorf("expected score 0.95, got %f", score)
	}
}

// TestSaveNERResultsSingle tests saving a single NER result
func TestSaveNERResultsSingle(t *testing.T) {
	db := setupMLTestDB(t)
	defer teardownMLTestDB(t, db)
	defer cleanupMLTables(t, db)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	chatID := int64(-1001234567890)
	userID := int64(123456)
	messageID := int64(1)

	dbMessageID := setupMLTestMessage(t, ctx, tx, chatID, userID, messageID, "John lives in New York")
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	repo := NewMLRepository(db, nil)
	startPos := 0
	endPos := 4
	result := NERResult{
		MessageID:  dbMessageID,
		ChatID:     chatID,
		EntityType: "PERSON",
		EntityText: "John",
		StartPos:   &startPos,
		EndPos:     &endPos,
		Confidence: 0.98,
	}

	err = repo.SaveNERResults(ctx, []NERResult{result})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify insertion
	var entityType, entityText string
	var startPosResult, endPosResult sql.NullInt32
	var confidence float32
	err = db.QueryRowContext(ctx, `
		SELECT entity_type, entity_text, start_pos, end_pos, confidence
		FROM ml_ner
		WHERE message_id = $1
	`, dbMessageID).Scan(&entityType, &entityText, &startPosResult, &endPosResult, &confidence)
	if err != nil {
		t.Fatalf("failed to query NER: %v", err)
	}

	if entityType != "PERSON" {
		t.Errorf("expected entity_type 'PERSON', got %q", entityType)
	}
	if entityText != "John" {
		t.Errorf("expected entity_text 'John', got %q", entityText)
	}
	if !startPosResult.Valid || int(startPosResult.Int32) != startPos {
		t.Errorf("expected start_pos %d, got %v", startPos, startPosResult)
	}
	if !endPosResult.Valid || int(endPosResult.Int32) != endPos {
		t.Errorf("expected end_pos %d, got %v", endPos, endPosResult)
	}
	if confidence != 0.98 {
		t.Errorf("expected confidence 0.98, got %f", confidence)
	}
}

// TestSaveNERResultsBatch tests saving multiple NER results
func TestSaveNERResultsBatch(t *testing.T) {
	db := setupMLTestDB(t)
	defer teardownMLTestDB(t, db)
	defer cleanupMLTables(t, db)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	chatID := int64(-1001234567890)
	userID := int64(123456)
	messageID := int64(1)

	dbMessageID := setupMLTestMessage(t, ctx, tx, chatID, userID, messageID, "John lives in New York")
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	repo := NewMLRepository(db, nil)
	startPos1 := 0
	endPos1 := 4
	startPos2 := 14
	endPos2 := 22

	results := []NERResult{
		{
			MessageID:  dbMessageID,
			ChatID:     chatID,
			EntityType: "PERSON",
			EntityText: "John",
			StartPos:   &startPos1,
			EndPos:     &endPos1,
			Confidence: 0.98,
		},
		{
			MessageID:  dbMessageID,
			ChatID:     chatID,
			EntityType: "LOCATION",
			EntityText: "New York",
			StartPos:   &startPos2,
			EndPos:     &endPos2,
			Confidence: 0.96,
		},
	}

	err = repo.SaveNERResults(ctx, results)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify count
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ml_ner WHERE message_id = $1", dbMessageID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count NER results: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 NER results, got %d", count)
	}
}

// TestSaveTopicsSingle tests saving topics for a chat
func TestSaveTopicsSingle(t *testing.T) {
	db := setupMLTestDB(t)
	defer teardownMLTestDB(t, db)
	defer cleanupMLTables(t, db)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	chatID := int64(-1001234567890)

	// Insert chat
	chatRepo := NewChatRepository(nil, nil)
	err = chatRepo.UpsertChat(ctx, tx, &models.Chat{
		ID:   chatID,
		Type: "supergroup",
	})
	if err != nil {
		t.Fatalf("failed to setup chat: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	repo := NewMLRepository(db, nil)
	topics := map[int][]string{
		0: {"technology", "programming", "software"},
		1: {"sports", "football", "basketball"},
	}

	err = repo.SaveTopics(ctx, chatID, topics)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify count
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ml_topics WHERE chat_id = $1", chatID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count topics: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 topics, got %d", count)
	}

	// Verify topic 0 keywords (need to use pq.Array for scanning)
	var keywords []string
	err = db.QueryRowContext(ctx, "SELECT keywords FROM ml_topics WHERE chat_id = $1 AND topic_id = 0", chatID).Scan((*sqlArray)(&keywords))
	if err != nil {
		t.Fatalf("failed to query topic keywords: %v", err)
	}
	if len(keywords) != 3 {
		t.Errorf("expected 3 keywords, got %d", len(keywords))
	}
}

// TestSaveMessageTopicsSingle tests saving message-to-topic assignments
func TestSaveMessageTopicsSingle(t *testing.T) {
	db := setupMLTestDB(t)
	defer teardownMLTestDB(t, db)
	defer cleanupMLTables(t, db)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	chatID := int64(-1001234567890)
	userID := int64(123456)
	messageID := int64(1)

	dbMessageID := setupMLTestMessage(t, ctx, tx, chatID, userID, messageID, "Test message")
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	repo := NewMLRepository(db, nil)
	result := MessageTopicResult{
		MessageID:  dbMessageID,
		ChatID:     chatID,
		TopicID:    0,
		Similarity: 0.85,
	}

	err = repo.SaveMessageTopics(ctx, []MessageTopicResult{result})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify insertion
	var topicID int
	var similarity float32
	err = db.QueryRowContext(ctx, `
		SELECT topic_id, similarity
		FROM ml_message_topics
		WHERE message_id = $1
	`, dbMessageID).Scan(&topicID, &similarity)
	if err != nil {
		t.Fatalf("failed to query message topic: %v", err)
	}

	if topicID != 0 {
		t.Errorf("expected topic_id 0, got %d", topicID)
	}
	if similarity != 0.85 {
		t.Errorf("expected similarity 0.85, got %f", similarity)
	}
}

// TestSaveMessageTopicsBatch tests saving multiple message-to-topic assignments
func TestSaveMessageTopicsBatch(t *testing.T) {
	db := setupMLTestDB(t)
	defer teardownMLTestDB(t, db)
	defer cleanupMLTables(t, db)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	chatID := int64(-1001234567890)
	userID := int64(123456)

	dbMessageID1 := setupMLTestMessage(t, ctx, tx, chatID, userID, 1, "Tech message")
	dbMessageID2 := setupMLTestMessage(t, ctx, tx, chatID, userID, 2, "Sports message")
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	repo := NewMLRepository(db, nil)
	results := []MessageTopicResult{
		{MessageID: dbMessageID1, ChatID: chatID, TopicID: 0, Similarity: 0.92},
		{MessageID: dbMessageID2, ChatID: chatID, TopicID: 1, Similarity: 0.88},
	}

	err = repo.SaveMessageTopics(ctx, results)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify count
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ml_message_topics").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count message topics: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 message topic assignments, got %d", count)
	}
}

// TestPaginationCorrectness tests that limit/offset work correctly
func TestPaginationCorrectness(t *testing.T) {
	db := setupMLTestDB(t)
	defer teardownMLTestDB(t, db)
	defer cleanupMLTables(t, db)

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	chatID := int64(-1001234567890)
	userID := int64(123456)

	// Insert 10 messages
	for i := int64(1); i <= 10; i++ {
		setupMLTestMessage(t, ctx, tx, chatID, userID, i, "Test message")
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	repo := NewMLRepository(db, nil)

	// Test limit (get first 5)
	page1, err := repo.GetUnprocessedMessages(ctx, 5)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(page1) != 5 {
		t.Errorf("expected 5 messages, got %d", len(page1))
	}

	// Verify first page contains messages 1-5 (ordered by date ASC)
	for i := 0; i < 5; i++ {
		if page1[i].MessageID != int64(i+1) {
			t.Errorf("expected MessageID %d, got %d", i+1, page1[i].MessageID)
		}
	}

	// Note: GetUnprocessedMessages doesn't have offset parameter, so we can't test offset here.
	// The pagination is implicit through marking messages as processed.
}
