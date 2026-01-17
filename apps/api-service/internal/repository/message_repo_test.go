package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"beef-briefing/apps/api-service/internal/models"
)

// setupMessageTestDB creates a new database connection for testing
func setupMessageTestDB(t *testing.T) *sql.DB {
	t.Helper()
	return setupUserTestDB(t) // Reuse existing setup
}

// teardownMessageTestDB closes the test database connection
func teardownMessageTestDB(t *testing.T, db *sql.DB) {
	t.Helper()
	teardownUserTestDB(t, db) // Reuse existing teardown
}

// cleanupMessageTables truncates the messages, message_edits, and message_entities tables for test isolation
func cleanupMessageTables(t *testing.T, db *sql.DB) {
	t.Helper()

	ctx := context.Background()
	// Truncate in reverse dependency order
	if _, err := db.ExecContext(ctx, "TRUNCATE TABLE message_entities CASCADE"); err != nil {
		t.Fatalf("failed to truncate message_entities table: %v", err)
	}
	if _, err := db.ExecContext(ctx, "TRUNCATE TABLE message_edits CASCADE"); err != nil {
		t.Fatalf("failed to truncate message_edits table: %v", err)
	}
	if _, err := db.ExecContext(ctx, "TRUNCATE TABLE messages CASCADE"); err != nil {
		t.Fatalf("failed to truncate messages table: %v", err)
	}
	if _, err := db.ExecContext(ctx, "TRUNCATE TABLE users CASCADE"); err != nil {
		t.Fatalf("failed to truncate users table: %v", err)
	}
	if _, err := db.ExecContext(ctx, "TRUNCATE TABLE chats CASCADE"); err != nil {
		t.Fatalf("failed to truncate chats table: %v", err)
	}
}

// sampleMessage creates a test message with standard values
func sampleMessage() models.Message {
	userID := int64(123456789)
	return models.Message{
		MessageID: 1,
		Chat: models.Chat{
			ID:   -1003280306634,
			Type: "supergroup",
		},
		From: &models.User{
			ID:        userID,
			FirstName: "Test",
			Username:  "testuser",
		},
		Date: time.Now().Unix(),
		Text: "Hello, World!",
	}
}

// setupMessageTestData inserts required users and chats for message testing
func setupMessageTestData(t *testing.T, ctx context.Context, tx *sql.Tx, msg *models.Message) {
	t.Helper()

	// Insert user if present
	if msg.From != nil {
		userRepo := NewUserRepository(nil, nil)
		err := userRepo.UpsertUser(ctx, tx, msg.From)
		if err != nil {
			t.Fatalf("failed to setup user: %v", err)
		}
	}

	// Insert chat
	chatRepo := NewChatRepository(nil, nil)
	err := chatRepo.UpsertChat(ctx, tx, &msg.Chat)
	if err != nil {
		t.Fatalf("failed to setup chat: %v", err)
	}
}

// TestInsertMessageTextBasic tests inserting a basic text message
func TestInsertMessageTextBasic(t *testing.T) {
	db := setupMessageTestDB(t)
	defer teardownMessageTestDB(t, db)
	defer cleanupMessageTables(t, db)

	ctx := context.Background()
	cleanupMessageTables(t, db)

	// Create a message repository without NewRelic (nil is safe)
	repo := NewMessageRepository(db, nil)

	// Create a sample message
	msg := sampleMessage()

	// Begin transaction for test isolation
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Setup required foreign key data
	setupMessageTestData(t, ctx, tx, &msg)

	// Insert the message
	dbID, err := repo.InsertMessage(ctx, tx, &msg)
	if err != nil {
		t.Fatalf("InsertMessage failed: %v", err)
	}

	if dbID == 0 {
		t.Errorf("expected non-zero database ID, got 0")
	}

	// Verify the message was inserted correctly
	var dbMsg struct {
		ID        int64
		MessageID int64
		ChatID    int64
		UserID    sql.NullInt64
		Text      sql.NullString
	}

	query := `SELECT id, message_id, chat_id, user_id, text FROM messages WHERE id = $1`
	err = tx.QueryRowContext(ctx, query, dbID).Scan(
		&dbMsg.ID, &dbMsg.MessageID, &dbMsg.ChatID, &dbMsg.UserID, &dbMsg.Text,
	)
	if err != nil {
		t.Fatalf("failed to query message: %v", err)
	}

	// Assert all fields were inserted correctly
	if dbMsg.ID != dbID {
		t.Errorf("ID mismatch: got %d, want %d", dbMsg.ID, dbID)
	}
	if dbMsg.MessageID != msg.MessageID {
		t.Errorf("MessageID mismatch: got %d, want %d", dbMsg.MessageID, msg.MessageID)
	}
	if dbMsg.ChatID != msg.Chat.ID {
		t.Errorf("ChatID mismatch: got %d, want %d", dbMsg.ChatID, msg.Chat.ID)
	}
	if !dbMsg.UserID.Valid || dbMsg.UserID.Int64 != msg.From.ID {
		t.Errorf("UserID mismatch: got %d (valid=%v), want %d", dbMsg.UserID.Int64, dbMsg.UserID.Valid, msg.From.ID)
	}
	if !dbMsg.Text.Valid || dbMsg.Text.String != msg.Text {
		t.Errorf("Text mismatch: got %q (valid=%v), want %q", dbMsg.Text.String, dbMsg.Text.Valid, msg.Text)
	}
}

// TestInsertMessageWithMediaAttachment tests inserting a message with caption (media attachment)
func TestInsertMessageWithMediaAttachment(t *testing.T) {
	db := setupMessageTestDB(t)
	defer teardownMessageTestDB(t, db)
	defer cleanupMessageTables(t, db)

	ctx := context.Background()
	cleanupMessageTables(t, db)

	repo := NewMessageRepository(db, nil)

	// Create message with caption (photo message)
	msg := sampleMessage()
	msg.Text = "" // Photos don't have text
	msg.Caption = "Check out this photo!"
	msg.MediaGroupID = "12345678901234567"

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	setupMessageTestData(t, ctx, tx, &msg)

	dbID, err := repo.InsertMessage(ctx, tx, &msg)
	if err != nil {
		t.Fatalf("InsertMessage failed: %v", err)
	}

	// Verify caption and media_group_id were inserted
	var dbMsg struct {
		Caption      sql.NullString
		MediaGroupID sql.NullString
	}

	query := `SELECT caption, media_group_id FROM messages WHERE id = $1`
	err = tx.QueryRowContext(ctx, query, dbID).Scan(&dbMsg.Caption, &dbMsg.MediaGroupID)
	if err != nil {
		t.Fatalf("failed to query message: %v", err)
	}

	if !dbMsg.Caption.Valid || dbMsg.Caption.String != msg.Caption {
		t.Errorf("Caption mismatch: got %q (valid=%v), want %q", dbMsg.Caption.String, dbMsg.Caption.Valid, msg.Caption)
	}
	if !dbMsg.MediaGroupID.Valid || dbMsg.MediaGroupID.String != msg.MediaGroupID {
		t.Errorf("MediaGroupID mismatch: got %q (valid=%v), want %q", dbMsg.MediaGroupID.String, dbMsg.MediaGroupID.Valid, msg.MediaGroupID)
	}
}

// TestInsertMessageReplyToAnother tests inserting a message that replies to another message
func TestInsertMessageReplyToAnother(t *testing.T) {
	db := setupMessageTestDB(t)
	defer teardownMessageTestDB(t, db)
	defer cleanupMessageTables(t, db)

	ctx := context.Background()
	cleanupMessageTables(t, db)

	repo := NewMessageRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Create and insert original message
	originalMsg := sampleMessage()
	originalMsg.MessageID = 100
	setupMessageTestData(t, ctx, tx, &originalMsg)
	originalDBID, err := repo.InsertMessage(ctx, tx, &originalMsg)
	if err != nil {
		t.Fatalf("failed to insert original message: %v", err)
	}

	// Create reply message
	replyMsg := sampleMessage()
	replyMsg.MessageID = 101
	replyMsg.Text = "This is a reply"
	replyMsg.ReplyToMessage = &models.Message{
		MessageID: originalMsg.MessageID,
	}

	replyDBID, err := repo.InsertMessage(ctx, tx, &replyMsg)
	if err != nil {
		t.Fatalf("InsertMessage for reply failed: %v", err)
	}

	// Verify reply_to_message_id was set correctly
	var replyToMessageID sql.NullInt64
	query := `SELECT reply_to_message_id FROM messages WHERE id = $1`
	err = tx.QueryRowContext(ctx, query, replyDBID).Scan(&replyToMessageID)
	if err != nil {
		t.Fatalf("failed to query reply message: %v", err)
	}

	if !replyToMessageID.Valid {
		t.Errorf("reply_to_message_id should be valid")
	}
	if replyToMessageID.Int64 != originalMsg.MessageID {
		t.Errorf("reply_to_message_id mismatch: got %d, want %d", replyToMessageID.Int64, originalMsg.MessageID)
	}

	// Verify both messages exist
	if originalDBID == 0 || replyDBID == 0 {
		t.Errorf("expected non-zero database IDs, got original=%d, reply=%d", originalDBID, replyDBID)
	}
}

// TestInsertMessageWithForwardInfo tests inserting a message with forward information
func TestInsertMessageWithForwardInfo(t *testing.T) {
	db := setupMessageTestDB(t)
	defer teardownMessageTestDB(t, db)
	defer cleanupMessageTables(t, db)

	ctx := context.Background()
	cleanupMessageTables(t, db)

	repo := NewMessageRepository(db, nil)

	msg := sampleMessage()
	msg.IsAutomaticForward = true
	msg.Text = "Forwarded message content"

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	setupMessageTestData(t, ctx, tx, &msg)

	dbID, err := repo.InsertMessage(ctx, tx, &msg)
	if err != nil {
		t.Fatalf("InsertMessage failed: %v", err)
	}

	// Verify is_automatic_forward flag was set
	var isAutomaticForward bool
	query := `SELECT is_automatic_forward FROM messages WHERE id = $1`
	err = tx.QueryRowContext(ctx, query, dbID).Scan(&isAutomaticForward)
	if err != nil {
		t.Fatalf("failed to query message: %v", err)
	}

	if !isAutomaticForward {
		t.Errorf("is_automatic_forward should be true, got %v", isAutomaticForward)
	}
}

// TestInsertMessageOnConflict tests ON CONFLICT behavior (duplicate message_id in same chat)
func TestInsertMessageOnConflict(t *testing.T) {
	db := setupMessageTestDB(t)
	defer teardownMessageTestDB(t, db)
	defer cleanupMessageTables(t, db)

	ctx := context.Background()
	cleanupMessageTables(t, db)

	repo := NewMessageRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Insert original message
	msg := sampleMessage()
	msg.Text = "Original text"
	setupMessageTestData(t, ctx, tx, &msg)

	originalDBID, err := repo.InsertMessage(ctx, tx, &msg)
	if err != nil {
		t.Fatalf("first InsertMessage failed: %v", err)
	}

	// Insert same message with updated text (simulating an edit)
	msg.Text = "Updated text"
	editDate := time.Now().Unix()
	msg.EditDate = &editDate

	updatedDBID, err := repo.InsertMessage(ctx, tx, &msg)
	if err != nil {
		t.Fatalf("second InsertMessage failed: %v", err)
	}

	// Verify the same database ID was returned (ON CONFLICT DO UPDATE)
	if originalDBID != updatedDBID {
		t.Errorf("expected same database ID on conflict, got original=%d, updated=%d", originalDBID, updatedDBID)
	}

	// Verify the text was updated
	var text sql.NullString
	query := `SELECT text FROM messages WHERE id = $1`
	err = tx.QueryRowContext(ctx, query, updatedDBID).Scan(&text)
	if err != nil {
		t.Fatalf("failed to query message: %v", err)
	}

	if !text.Valid || text.String != "Updated text" {
		t.Errorf("text should be updated to %q, got %q (valid=%v)", "Updated text", text.String, text.Valid)
	}
}

// TestInsertMessageForeignKeyConstraints tests foreign key constraints (user, chat must exist)
func TestInsertMessageForeignKeyConstraints(t *testing.T) {
	db := setupMessageTestDB(t)
	defer teardownMessageTestDB(t, db)
	defer cleanupMessageTables(t, db)

	ctx := context.Background()
	cleanupMessageTables(t, db)

	repo := NewMessageRepository(db, nil)

	msg := sampleMessage()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Try to insert message without setting up user/chat (should fail)
	_, err = repo.InsertMessage(ctx, tx, &msg)
	if err == nil {
		t.Errorf("expected foreign key constraint error, got nil")
	}
}

// TestInsertMessageEdit tests inserting a message edit record
func TestInsertMessageEdit(t *testing.T) {
	db := setupMessageTestDB(t)
	defer teardownMessageTestDB(t, db)
	defer cleanupMessageTables(t, db)

	ctx := context.Background()
	cleanupMessageTables(t, db)

	repo := NewMessageRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Insert original message
	msg := sampleMessage()
	msg.Text = "Original text"
	setupMessageTestData(t, ctx, tx, &msg)

	dbID, err := repo.InsertMessage(ctx, tx, &msg)
	if err != nil {
		t.Fatalf("InsertMessage failed: %v", err)
	}

	// Insert message edit
	editDate := time.Now().UTC()
	previousText := "Original text"
	newText := "Edited text"

	err = repo.InsertMessageEdit(ctx, tx, dbID, editDate, previousText, newText, "", "")
	if err != nil {
		t.Fatalf("InsertMessageEdit failed: %v", err)
	}

	// Verify the edit was inserted
	var dbEdit struct {
		MessageID    int64
		PreviousText sql.NullString
		NewText      sql.NullString
	}

	query := `SELECT message_id, previous_text, new_text FROM message_edits WHERE message_id = $1`
	err = tx.QueryRowContext(ctx, query, dbID).Scan(&dbEdit.MessageID, &dbEdit.PreviousText, &dbEdit.NewText)
	if err != nil {
		t.Fatalf("failed to query message edit: %v", err)
	}

	if dbEdit.MessageID != dbID {
		t.Errorf("message_id mismatch: got %d, want %d", dbEdit.MessageID, dbID)
	}
	if !dbEdit.PreviousText.Valid || dbEdit.PreviousText.String != previousText {
		t.Errorf("previous_text mismatch: got %q (valid=%v), want %q", dbEdit.PreviousText.String, dbEdit.PreviousText.Valid, previousText)
	}
	if !dbEdit.NewText.Valid || dbEdit.NewText.String != newText {
		t.Errorf("new_text mismatch: got %q (valid=%v), want %q", dbEdit.NewText.String, dbEdit.NewText.Valid, newText)
	}
}

// TestInsertMessageEditWithNullText tests inserting a message edit with NULL text (deletion)
func TestInsertMessageEditWithNullText(t *testing.T) {
	db := setupMessageTestDB(t)
	defer teardownMessageTestDB(t, db)
	defer cleanupMessageTables(t, db)

	ctx := context.Background()
	cleanupMessageTables(t, db)

	repo := NewMessageRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Insert original message
	msg := sampleMessage()
	msg.Text = "Original text"
	setupMessageTestData(t, ctx, tx, &msg)

	dbID, err := repo.InsertMessage(ctx, tx, &msg)
	if err != nil {
		t.Fatalf("InsertMessage failed: %v", err)
	}

	// Insert edit with empty new text (deletion)
	editDate := time.Now().UTC()
	previousText := "Original text"
	newText := "" // empty - should be NULL

	err = repo.InsertMessageEdit(ctx, tx, dbID, editDate, previousText, newText, "", "")
	if err != nil {
		t.Fatalf("InsertMessageEdit failed: %v", err)
	}

	// Verify new_text is NULL
	var newTextDB sql.NullString
	query := `SELECT new_text FROM message_edits WHERE message_id = $1`
	err = tx.QueryRowContext(ctx, query, dbID).Scan(&newTextDB)
	if err != nil {
		t.Fatalf("failed to query message edit: %v", err)
	}

	if newTextDB.Valid {
		t.Errorf("new_text should be NULL (deletion), got %q", newTextDB.String)
	}
}

// TestInsertEntityMention tests inserting a mention entity
func TestInsertEntityMention(t *testing.T) {
	db := setupMessageTestDB(t)
	defer teardownMessageTestDB(t, db)
	defer cleanupMessageTables(t, db)

	ctx := context.Background()
	cleanupMessageTables(t, db)

	repo := NewMessageRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Insert message with mention
	msg := sampleMessage()
	msg.Text = "Hello @testuser!"
	setupMessageTestData(t, ctx, tx, &msg)

	dbID, err := repo.InsertMessage(ctx, tx, &msg)
	if err != nil {
		t.Fatalf("InsertMessage failed: %v", err)
	}

	// Create mention entity
	entity := &models.MessageEntity{
		Type:   "mention",
		Offset: 6,
		Length: 9, // "@testuser"
		User: &models.User{
			ID:        msg.From.ID,
			FirstName: "Test",
			Username:  "testuser",
		},
	}

	err = repo.InsertEntity(ctx, tx, dbID, entity)
	if err != nil {
		t.Fatalf("InsertEntity failed: %v", err)
	}

	// Verify the entity was inserted
	var dbEntity struct {
		EntityType string
		Offset     int
		Length     int
		UserID     sql.NullInt64
	}

	query := `SELECT entity_type, entity_offset, entity_length, user_id FROM message_entities WHERE message_id = $1`
	err = tx.QueryRowContext(ctx, query, dbID).Scan(&dbEntity.EntityType, &dbEntity.Offset, &dbEntity.Length, &dbEntity.UserID)
	if err != nil {
		t.Fatalf("failed to query entity: %v", err)
	}

	if dbEntity.EntityType != "mention" {
		t.Errorf("entity_type mismatch: got %q, want %q", dbEntity.EntityType, "mention")
	}
	if dbEntity.Offset != entity.Offset {
		t.Errorf("offset mismatch: got %d, want %d", dbEntity.Offset, entity.Offset)
	}
	if dbEntity.Length != entity.Length {
		t.Errorf("length mismatch: got %d, want %d", dbEntity.Length, entity.Length)
	}
	if !dbEntity.UserID.Valid || dbEntity.UserID.Int64 != entity.User.ID {
		t.Errorf("user_id mismatch: got %d (valid=%v), want %d", dbEntity.UserID.Int64, dbEntity.UserID.Valid, entity.User.ID)
	}
}

// TestInsertEntityURL tests inserting a URL entity
func TestInsertEntityURL(t *testing.T) {
	db := setupMessageTestDB(t)
	defer teardownMessageTestDB(t, db)
	defer cleanupMessageTables(t, db)

	ctx := context.Background()
	cleanupMessageTables(t, db)

	repo := NewMessageRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Insert message with URL
	msg := sampleMessage()
	msg.Text = "Check out https://example.com"
	setupMessageTestData(t, ctx, tx, &msg)

	dbID, err := repo.InsertMessage(ctx, tx, &msg)
	if err != nil {
		t.Fatalf("InsertMessage failed: %v", err)
	}

	// Create URL entity
	entity := &models.MessageEntity{
		Type:   "url",
		Offset: 10,
		Length: 19, // "https://example.com"
		URL:    "https://example.com",
	}

	err = repo.InsertEntity(ctx, tx, dbID, entity)
	if err != nil {
		t.Fatalf("InsertEntity failed: %v", err)
	}

	// Verify the entity was inserted with URL
	var url sql.NullString
	query := `SELECT url FROM message_entities WHERE message_id = $1 AND entity_type = 'url'`
	err = tx.QueryRowContext(ctx, query, dbID).Scan(&url)
	if err != nil {
		t.Fatalf("failed to query entity: %v", err)
	}

	if !url.Valid || url.String != entity.URL {
		t.Errorf("url mismatch: got %q (valid=%v), want %q", url.String, url.Valid, entity.URL)
	}
}

// TestInsertEntityHashtag tests inserting a hashtag entity
func TestInsertEntityHashtag(t *testing.T) {
	db := setupMessageTestDB(t)
	defer teardownMessageTestDB(t, db)
	defer cleanupMessageTables(t, db)

	ctx := context.Background()
	cleanupMessageTables(t, db)

	repo := NewMessageRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Insert message with hashtag
	msg := sampleMessage()
	msg.Text = "This is #awesome"
	setupMessageTestData(t, ctx, tx, &msg)

	dbID, err := repo.InsertMessage(ctx, tx, &msg)
	if err != nil {
		t.Fatalf("InsertMessage failed: %v", err)
	}

	// Create hashtag entity
	entity := &models.MessageEntity{
		Type:   "hashtag",
		Offset: 8,
		Length: 8, // "#awesome"
	}

	err = repo.InsertEntity(ctx, tx, dbID, entity)
	if err != nil {
		t.Fatalf("InsertEntity failed: %v", err)
	}

	// Verify the entity was inserted
	var entityType string
	query := `SELECT entity_type FROM message_entities WHERE message_id = $1`
	err = tx.QueryRowContext(ctx, query, dbID).Scan(&entityType)
	if err != nil {
		t.Fatalf("failed to query entity: %v", err)
	}

	if entityType != "hashtag" {
		t.Errorf("entity_type mismatch: got %q, want %q", entityType, "hashtag")
	}
}

// TestGetMessageByIDExisting tests retrieving an existing message by ID
func TestGetMessageByIDExisting(t *testing.T) {
	db := setupMessageTestDB(t)
	defer teardownMessageTestDB(t, db)
	defer cleanupMessageTables(t, db)

	ctx := context.Background()
	cleanupMessageTables(t, db)

	repo := NewMessageRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Insert a message
	msg := sampleMessage()
	msg.Text = "Test message for retrieval"
	msg.Caption = "Test caption"
	setupMessageTestData(t, ctx, tx, &msg)

	dbID, err := repo.InsertMessage(ctx, tx, &msg)
	if err != nil {
		t.Fatalf("InsertMessage failed: %v", err)
	}

	// Retrieve the message by chat_id and message_id
	dbMsg, err := repo.GetMessageByID(ctx, tx, msg.Chat.ID, msg.MessageID)
	if err != nil {
		t.Fatalf("GetMessageByID failed: %v", err)
	}

	if dbMsg == nil {
		t.Fatalf("expected message, got nil")
	}

	if dbMsg.ID != dbID {
		t.Errorf("ID mismatch: got %d, want %d", dbMsg.ID, dbID)
	}
	if !dbMsg.Text.Valid || dbMsg.Text.String != msg.Text {
		t.Errorf("Text mismatch: got %q (valid=%v), want %q", dbMsg.Text.String, dbMsg.Text.Valid, msg.Text)
	}
	if !dbMsg.Caption.Valid || dbMsg.Caption.String != msg.Caption {
		t.Errorf("Caption mismatch: got %q (valid=%v), want %q", dbMsg.Caption.String, dbMsg.Caption.Valid, msg.Caption)
	}
}

// TestGetMessageByIDNonExistent tests retrieving a non-existent message
func TestGetMessageByIDNonExistent(t *testing.T) {
	db := setupMessageTestDB(t)
	defer teardownMessageTestDB(t, db)
	defer cleanupMessageTables(t, db)

	ctx := context.Background()
	cleanupMessageTables(t, db)

	repo := NewMessageRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Try to retrieve non-existent message
	_, err = repo.GetMessageByID(ctx, tx, -1003280306634, 99999)
	if err == nil {
		t.Errorf("expected error for non-existent message, got nil")
	}
}
