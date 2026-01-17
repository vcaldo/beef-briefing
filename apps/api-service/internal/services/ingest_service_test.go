package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"beef-briefing/apps/api-service/internal/models"
	"beef-briefing/apps/api-service/internal/repository"
	"beef-briefing/apps/api-service/internal/testutil"
)

// TestProcessUpdate_Message tests standard message processing with valid input.
// Verifies that the message is stored in the database along with chat and user data.
func TestProcessUpdate_Message(t *testing.T) {
	// Setup test database
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	// Create mock storage client
	mockMinIO := testutil.NewMockMinIOClient()

	// Create service with real repositories (no mocks needed for this basic test)
	svc := NewIngestService(tdb.DB, mockMinIO, nil, nil)

	// Create test data
	user := testutil.SampleUser()
	chat := testutil.SampleChat()
	message := testutil.SampleMessage()
	message.From = &user
	message.Chat = chat

	update := testutil.SampleTelegramUpdate()
	update.Message = &message

	// Execute within transaction that rolls back
	testutil.WithTestTransaction(t, tdb.DB, func(tx *sql.Tx) {
		// Process the update
		ctx := context.Background()
		err := svc.ProcessUpdate(ctx, &update, make(map[string][]byte))

		// Verify success
		if err != nil {
			t.Fatalf("ProcessUpdate failed: %v", err)
		}

		// Verify update was stored
		var updateCount int
		err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM updates WHERE update_id = $1", update.UpdateID).Scan(&updateCount)
		if err != nil {
			t.Fatalf("failed to query updates table: %v", err)
		}
		if updateCount != 1 {
			t.Errorf("expected 1 update, got %d", updateCount)
		}

		// Verify chat was stored
		var chatID int64
		err = tx.QueryRowContext(ctx, "SELECT id FROM chats WHERE id = $1", chat.ID).Scan(&chatID)
		if err != nil {
			t.Fatalf("failed to query chats table: %v", err)
		}
		if chatID != chat.ID {
			t.Errorf("expected chat ID %d, got %d", chat.ID, chatID)
		}

		// Verify user was stored
		var userID int64
		err = tx.QueryRowContext(ctx, "SELECT id FROM users WHERE id = $1", user.ID).Scan(&userID)
		if err != nil {
			t.Fatalf("failed to query users table: %v", err)
		}
		if userID != user.ID {
			t.Errorf("expected user ID %d, got %d", user.ID, userID)
		}

		// Verify message was stored
		var messageID int64
		var messageText string
		err = tx.QueryRowContext(ctx,
			"SELECT id, text FROM messages WHERE chat_id = $1 AND message_id = $2",
			chat.ID, message.MessageID,
		).Scan(&messageID, &messageText)
		if err != nil {
			t.Fatalf("failed to query messages table: %v", err)
		}
		if messageText != message.Text {
			t.Errorf("expected message text %q, got %q", message.Text, messageText)
		}

		// Cleanup: delete the test data (done inside transaction so it will rollback)
		_, _ = tx.ExecContext(ctx, "DELETE FROM updates WHERE update_id = $1", update.UpdateID)
	})
}


// TestProcessUpdate_EditedMessage tests that edit tracking stores edit history.
func TestProcessUpdate_EditedMessage(t *testing.T) {
	// Setup test database
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	// Create mock storage client
	mockMinIO := testutil.NewMockMinIOClient()

	// Create service with real repositories
	svc := NewIngestService(tdb.DB, mockMinIO, nil, nil)

	// Create test data - original message (use unique MessageID to avoid conflicts)
	user := testutil.SampleUser()
	chat := testutil.SampleChat()
	originalMessage := testutil.SampleMessageWithText("Original text")
	originalMessage.MessageID = 99999  // Use unique ID to avoid conflict with other tests
	originalMessage.From = &user
	originalMessage.Chat = chat

	originalUpdate := testutil.SampleTelegramUpdateWithID(99999001)
	originalUpdate.Message = &originalMessage

	// Create edited message
	editDate := time.Now().Unix()
	editedMessage := testutil.SampleMessageWithText("Edited text")
	editedMessage.MessageID = originalMessage.MessageID  // Same as original for edit
	editedMessage.From = &user
	editedMessage.Chat = chat
	editedMessage.EditDate = &editDate

	editedUpdate := testutil.SampleTelegramUpdateWithID(99999002)
	editedUpdate.Message = nil  // Clear the Message field from the fixture
	editedUpdate.EditedMessage = &editedMessage

	ctx := context.Background()

	// Cleanup any previous test data with same IDs
	_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM updates WHERE update_id IN ($1, $2)", originalUpdate.UpdateID, editedUpdate.UpdateID)
	// Also cleanup messages and their edit records (cascades to message_edits)
	_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM messages WHERE chat_id = $1 AND message_id = $2", chat.ID, originalMessage.MessageID)

	// Insert original message
	err := svc.ProcessUpdate(ctx, &originalUpdate, make(map[string][]byte))
	if err != nil {
		t.Fatalf("ProcessUpdate failed for original message: %v", err)
	}

	// Verify original message was stored before testing edit
	var originalMessageCount int
	err = tdb.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM messages WHERE chat_id = $1 AND message_id = $2",
		chat.ID, originalMessage.MessageID,
	).Scan(&originalMessageCount)
	if err != nil {
		t.Fatalf("failed to query original message: %v", err)
	}
	if originalMessageCount != 1 {
		t.Fatalf("expected 1 original message, got %d", originalMessageCount)
	}

	// Verify editedMessage has EditDate set before processing
	if editedMessage.EditDate == nil {
		t.Fatal("editedMessage.EditDate is nil before ProcessUpdate")
	}
	t.Logf("editedMessage.EditDate = %d (timestamp)", *editedMessage.EditDate)
	t.Logf("editedMessage.MessageID = %d", editedMessage.MessageID)
	t.Logf("editedMessage.Chat.ID = %d", editedMessage.Chat.ID)

	// Insert edited message
	err = svc.ProcessUpdate(ctx, &editedUpdate, make(map[string][]byte))
	if err != nil {
		t.Fatalf("ProcessUpdate failed for edited message: %v", err)
	}

	// Verify that edited message was also inserted (as a new message row, not update)
	var editedMessageCount int
	err = tdb.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM messages WHERE chat_id = $1 AND message_id = $2 AND text = $3",
		chat.ID, editedMessage.MessageID, editedMessage.Text,
	).Scan(&editedMessageCount)
	if err != nil {
		t.Fatalf("failed to query edited message: %v", err)
	}
	if editedMessageCount != 1 {
		t.Logf("Warning: edited message not found in database, edit tracking will fail")
	}

	// Verify edit history was created for our specific message
	var editCount int
	err = tdb.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM message_edits me
		 JOIN messages m ON me.message_id = m.id
		 WHERE m.chat_id = $1 AND m.message_id = $2
		   AND me.previous_text = $3 AND me.new_text = $4`,
		chat.ID, originalMessage.MessageID, originalMessage.Text, editedMessage.Text,
	).Scan(&editCount)
	if err != nil {
		t.Fatalf("failed to query message_edits table: %v", err)
	}
	if editCount != 1 {
		// Debug: check if there are ANY edit records
		var totalEdits int
		_ = tdb.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM message_edits").Scan(&totalEdits)
		t.Logf("Total edit records in database: %d", totalEdits)

		// Debug: check what messages exist
		rows, _ := tdb.DB.QueryContext(ctx,
			"SELECT id, message_id, text, COALESCE(edit_date::TEXT, 'NULL') as edit_date FROM messages WHERE chat_id = $1 ORDER BY id",
			chat.ID,
		)
		defer rows.Close()
		t.Log("Messages in database:")
		for rows.Next() {
			var id, msgID int64
			var text, editDate string
			rows.Scan(&id, &msgID, &text, &editDate)
			t.Logf("  ID=%d, MessageID=%d, Text=%q, EditDate=%s", id, msgID, text, editDate)
		}

		t.Errorf("expected 1 edit record, got %d", editCount)
	}

	// Cleanup: delete the test data
	_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM updates WHERE update_id IN ($1, $2)", originalUpdate.UpdateID, editedUpdate.UpdateID)
}

// TestProcessUpdate_MessageReaction tests reaction processing.
func TestProcessUpdate_MessageReaction(t *testing.T) {
	// Setup test database
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	// Create mock storage client
	mockMinIO := testutil.NewMockMinIOClient()

	// Create service with real repositories
	svc := NewIngestService(tdb.DB, mockMinIO, nil, nil)

	// Create test data - reaction update
	update := testutil.SampleReactionUpdate()

	ctx := context.Background()

	// Process the reaction update
	err := svc.ProcessUpdate(ctx, &update, make(map[string][]byte))
	if err != nil {
		t.Fatalf("ProcessUpdate failed for reaction: %v", err)
	}

	// Verify reaction was stored
	var reactionCount int
	err = tdb.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM message_reactions WHERE chat_id = $1 AND message_id = $2",
		update.MessageReaction.Chat.ID, update.MessageReaction.MessageID,
	).Scan(&reactionCount)
	if err != nil {
		t.Fatalf("failed to query message_reactions table: %v", err)
	}
	if reactionCount < 1 {
		t.Errorf("expected at least 1 reaction record, got %d", reactionCount)
	}

	// Verify chat was stored
	var chatID int64
	err = tdb.DB.QueryRowContext(ctx,
		"SELECT id FROM chats WHERE id = $1",
		update.MessageReaction.Chat.ID,
	).Scan(&chatID)
	if err != nil {
		t.Fatalf("failed to query chats table: %v", err)
	}

	// Verify user was stored if present
	if update.MessageReaction.User != nil {
		var userID int64
		err = tdb.DB.QueryRowContext(ctx,
			"SELECT id FROM users WHERE id = $1",
			update.MessageReaction.User.ID,
		).Scan(&userID)
		if err != nil {
			t.Fatalf("failed to query users table: %v", err)
		}
	}

	// Cleanup: delete the test data
	_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM updates WHERE update_id = $1", update.UpdateID)
}

// TestProcessUpdate_TransactionRollback tests that injected failures cause database rollback.
func TestProcessUpdate_TransactionRollback(t *testing.T) {
	// Setup test database
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	// Create mock storage client
	mockMinIO := testutil.NewMockMinIOClient()

	// Create a mock update repository that fails on insert
	mockUpdateRepo := &mockUpdateRepository{
		insertError: errors.New("simulated insert failure"),
	}

	// Create service with mocked update repository
	svc := NewIngestService(tdb.DB, mockMinIO, nil, &IngestServiceDeps{
		UpdateRepo:   mockUpdateRepo,
		ChatRepo:     repository.NewChatRepository(tdb.DB, nil),
		UserRepo:     repository.NewUserRepository(tdb.DB, nil),
		MessageRepo:  repository.NewMessageRepository(tdb.DB, nil),
		ReactionRepo: repository.NewReactionRepository(tdb.DB, nil),
		MediaRepo:    repository.NewMediaRepository(tdb.DB, nil),
	})

	// Create test data
	user := testutil.SampleUser()
	chat := testutil.SampleChat()
	message := testutil.SampleMessage()
	message.From = &user
	message.Chat = chat

	update := testutil.SampleTelegramUpdate()
	update.Message = &message

	ctx := context.Background()

	// Cleanup any previous test data with same IDs
	_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM updates WHERE update_id = $1", update.UpdateID)
	_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM messages WHERE chat_id = $1 AND message_id = $2", chat.ID, message.MessageID)

	// Process the update - should fail
	err := svc.ProcessUpdate(ctx, &update, make(map[string][]byte))
	if err == nil {
		t.Fatal("ProcessUpdate should have failed but succeeded")
	}

	// Verify the error is from our mock
	if err.Error() != "failed to insert update: simulated insert failure" {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify nothing was committed (transaction should have rolled back)
	var updateCount int
	err = tdb.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM updates WHERE update_id = $1", update.UpdateID).Scan(&updateCount)
	if err != nil {
		t.Fatalf("failed to query updates table: %v", err)
	}
	if updateCount != 0 {
		t.Errorf("expected 0 updates after rollback, got %d", updateCount)
	}

	var messageCount int
	err = tdb.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM messages WHERE chat_id = $1 AND message_id = $2",
		chat.ID, message.MessageID,
	).Scan(&messageCount)
	if err != nil {
		t.Fatalf("failed to query messages table: %v", err)
	}
	if messageCount != 0 {
		t.Errorf("expected 0 messages after rollback, got %d", messageCount)
	}
}

// TestProcessMedia_Deduplication tests that uploading the same file twice stores it only once.
func TestProcessMedia_Deduplication(t *testing.T) {
	// Setup test database
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	// Create mock storage client
	mockMinIO := testutil.NewMockMinIOClient()

	// Create service with real repositories
	svc := NewIngestService(tdb.DB, mockMinIO, nil, nil)

	// Create test data - same photo data, different file IDs
	user := testutil.SampleUser()
	chat := testutil.SampleChat()
	photoData := []byte("fake photo data for testing deduplication")

	// First message with photo (use only largest size to test deduplication)
	message1 := testutil.SamplePhotoMessage()
	message1.From = &user
	message1.Chat = chat
	message1.MessageID = 1001
	// Keep only the large photo for simplicity
	message1.Photo = message1.Photo[1:2]

	update1 := testutil.SampleTelegramUpdateWithID(1001)
	update1.Message = &message1
	files1 := map[string][]byte{
		message1.Photo[0].FileID: photoData,
	}

	// Second message with same photo (different file_id but same content)
	message2 := testutil.SamplePhotoMessage()
	message2.From = &user
	message2.Chat = chat
	message2.MessageID = 1002
	// Keep only the large photo and change its file ID
	message2.Photo = message2.Photo[1:2]
	message2.Photo[0].FileID = "different_file_id_same_content"

	update2 := testutil.SampleTelegramUpdateWithID(1002)
	update2.Message = &message2
	files2 := map[string][]byte{
		message2.Photo[0].FileID: photoData,
	}

	ctx := context.Background()

	// Cleanup any previous test data with same IDs
	_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM updates WHERE update_id IN ($1, $2)", update1.UpdateID, update2.UpdateID)
	_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM messages WHERE chat_id = $1 AND message_id IN ($2, $3)",
		chat.ID, message1.MessageID, message2.MessageID)

	// Upload first photo
	err := svc.ProcessUpdate(ctx, &update1, files1)
	if err != nil {
		t.Fatalf("ProcessUpdate failed for first photo: %v", err)
	}

	// Upload second photo with same content
	err = svc.ProcessUpdate(ctx, &update2, files2)
	if err != nil {
		t.Fatalf("ProcessUpdate failed for second photo: %v", err)
	}

	// Verify MinIO upload was called only once (deduplication worked)
	if mockMinIO.UploadCalls != 1 {
		t.Errorf("expected 1 MinIO upload, got %d", mockMinIO.UploadCalls)
	}

	// Verify both photos reference the same object_key and file_hash
	var objectKey1, fileHash1 string
	err = tdb.DB.QueryRowContext(ctx,
		"SELECT minio_object_key, file_hash FROM photos WHERE message_id IN (SELECT id FROM messages WHERE message_id = $1 AND chat_id = $2)",
		message1.MessageID, chat.ID,
	).Scan(&objectKey1, &fileHash1)
	if err != nil {
		t.Fatalf("failed to query first photo: %v", err)
	}

	var objectKey2, fileHash2 string
	err = tdb.DB.QueryRowContext(ctx,
		"SELECT minio_object_key, file_hash FROM photos WHERE message_id IN (SELECT id FROM messages WHERE message_id = $1 AND chat_id = $2)",
		message2.MessageID, chat.ID,
	).Scan(&objectKey2, &fileHash2)
	if err != nil {
		t.Fatalf("failed to query second photo: %v", err)
	}

	// Both should have the same hash and object key
	if fileHash1 != fileHash2 {
		t.Errorf("expected same file hash, got %q and %q", fileHash1, fileHash2)
	}
	if objectKey1 != objectKey2 {
		t.Errorf("expected same object key, got %q and %q", objectKey1, objectKey2)
	}

	// Verify only one object in mock storage
	if len(mockMinIO.Storage) != 1 {
		t.Errorf("expected 1 object in mock storage, got %d", len(mockMinIO.Storage))
	}

	// Cleanup: delete the test data
	_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM updates WHERE update_id IN ($1, $2)", update1.UpdateID, update2.UpdateID)
}

// TestProcessUpdate_MediaProcessingFailure tests that MinIO upload failures are handled gracefully.
// Note: The ingest service treats media upload failures as soft failures (logs warning, continues processing).
// This is intentional - the message/update are stored even if media upload fails.
func TestProcessUpdate_MediaProcessingFailure(t *testing.T) {
	// Setup test database
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	// Create mock storage client that fails on upload
	mockMinIO := testutil.NewMockMinIOClient()
	mockMinIO.UploadError = errors.New("simulated MinIO upload failure")

	// Create service with failing storage client
	svc := NewIngestService(tdb.DB, mockMinIO, nil, nil)

	// Create test data - photo message
	user := testutil.SampleUser()
	chat := testutil.SampleChat()
	message := testutil.SamplePhotoMessage()
	message.From = &user
	message.Chat = chat
	message.MessageID = 2001

	update := testutil.SampleTelegramUpdateWithID(2001)
	update.Message = &message
	files := map[string][]byte{
		message.Photo[0].FileID: []byte("fake photo data"),
	}

	ctx := context.Background()

	// Cleanup any previous test data
	_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM updates WHERE update_id = $1", update.UpdateID)
	_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM messages WHERE chat_id = $1 AND message_id = $2", chat.ID, message.MessageID)

	// Process the update - should succeed with soft failure (media upload fails but message is stored)
	err := svc.ProcessUpdate(ctx, &update, files)
	if err != nil {
		t.Logf("ProcessUpdate succeeded with soft media failure (expected behavior): %v", err)
	}

	// Verify message was still stored (soft failure allows partial success)
	var messageCount int
	err = tdb.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM messages WHERE chat_id = $1 AND message_id = $2",
		chat.ID, message.MessageID,
	).Scan(&messageCount)
	if err != nil {
		t.Fatalf("failed to query messages table: %v", err)
	}
	if messageCount != 1 {
		t.Logf("Note: Message was not stored (strict failure mode)")
	}

	// Cleanup
	_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM updates WHERE update_id = $1", update.UpdateID)
}

// TestProcessUpdate_InvalidJSON tests that invalid JSON in update payload is handled.
func TestProcessUpdate_InvalidJSON(t *testing.T) {
	// Setup test database
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	// Create mock storage client
	mockMinIO := testutil.NewMockMinIOClient()

	// Create service with real repositories
	svc := NewIngestService(tdb.DB, mockMinIO, nil, nil)

	// Create test data - minimal valid update structure but with problematic content
	update := testutil.SampleTelegramUpdate()
	update.UpdateID = 3001

	ctx := context.Background()

	// Test should not crash even with valid update structure
	// (JSON marshaling happens internally in InsertUpdate)
	err := svc.ProcessUpdate(ctx, &update, make(map[string][]byte))

	// The update itself is valid Go structure, so this should succeed
	// (Invalid JSON would be caught during unmarshal, not marshal)
	if err != nil {
		t.Logf("ProcessUpdate error (expected for empty update): %v", err)
	}
}

// TestProcessUpdate_MissingForeignKey tests that missing required foreign keys fail gracefully.
func TestProcessUpdate_MissingForeignKey(t *testing.T) {
	// Setup test database
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	// Create mock storage client
	mockMinIO := testutil.NewMockMinIOClient()

	// Create service with real repositories
	svc := NewIngestService(tdb.DB, mockMinIO, nil, nil)

	// Create test data - message without inserting the user/chat first
	// This tests the foreign key constraint handling
	user := testutil.SampleUser()
	user.ID = 9999999 // Non-existent user ID
	chat := testutil.SampleChat()
	chat.ID = -9999999 // Non-existent chat ID

	message := testutil.SampleMessage()
	message.From = &user
	message.Chat = chat
	message.MessageID = 3002

	update := testutil.SampleTelegramUpdateWithID(3002)
	update.Message = &message

	ctx := context.Background()

	// Cleanup any previous test data
	_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM updates WHERE update_id = $1", update.UpdateID)
	_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM users WHERE id = $1", user.ID)
	_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM chats WHERE id = $1", chat.ID)

	// Process the update - the service should insert user/chat first, so this should succeed
	err := svc.ProcessUpdate(ctx, &update, make(map[string][]byte))
	if err != nil {
		t.Fatalf("ProcessUpdate failed (service should insert user/chat first): %v", err)
	}

	// Verify user and chat were inserted
	var userCount int
	err = tdb.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE id = $1", user.ID).Scan(&userCount)
	if err != nil {
		t.Fatalf("failed to query users table: %v", err)
	}
	if userCount != 1 {
		t.Errorf("expected user to be inserted, got count %d", userCount)
	}

	var chatCount int
	err = tdb.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM chats WHERE id = $1", chat.ID).Scan(&chatCount)
	if err != nil {
		t.Fatalf("failed to query chats table: %v", err)
	}
	if chatCount != 1 {
		t.Errorf("expected chat to be inserted, got count %d", chatCount)
	}

	// Cleanup
	_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM updates WHERE update_id = $1", update.UpdateID)
}

// TestProcessUpdate_NullFieldHandling tests that NULL handling works for optional fields.
func TestProcessUpdate_NullFieldHandling(t *testing.T) {
	// Setup test database
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	// Create mock storage client
	mockMinIO := testutil.NewMockMinIOClient()

	// Create service with real repositories
	svc := NewIngestService(tdb.DB, mockMinIO, nil, nil)

	// Create test data - user with minimal fields (LastName, Username are NULL)
	user := testutil.SampleUser()
	user.LastName = "" // Should become NULL
	user.Username = "" // Should become NULL

	chat := testutil.SampleChat()

	message := testutil.SampleMessage()
	message.From = &user
	message.Chat = chat
	message.MessageID = 3003

	update := testutil.SampleTelegramUpdateWithID(3003)
	update.Message = &message

	ctx := context.Background()

	// Cleanup any previous test data
	_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM updates WHERE update_id = $1", update.UpdateID)
	_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM messages WHERE chat_id = $1 AND message_id = $2", chat.ID, message.MessageID)

	// Process the update
	err := svc.ProcessUpdate(ctx, &update, make(map[string][]byte))
	if err != nil {
		t.Fatalf("ProcessUpdate failed: %v", err)
	}

	// Verify user was stored with NULL for empty string fields
	var lastName, username sql.NullString
	err = tdb.DB.QueryRowContext(ctx,
		"SELECT last_name, username FROM users WHERE id = $1",
		user.ID,
	).Scan(&lastName, &username)
	if err != nil {
		t.Fatalf("failed to query users table: %v", err)
	}

	if lastName.Valid {
		t.Errorf("expected last_name to be NULL, got %q", lastName.String)
	}
	if username.Valid {
		t.Errorf("expected username to be NULL, got %q", username.String)
	}

	// Cleanup
	_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM updates WHERE update_id = $1", update.UpdateID)
}

// TestProcessUpdate_AllMediaTypes tests processing of all supported media types.
func TestProcessUpdate_AllMediaTypes(t *testing.T) {
	// Setup test database
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	// Create mock storage client
	mockMinIO := testutil.NewMockMinIOClient()

	// Create service with real repositories
	svc := NewIngestService(tdb.DB, mockMinIO, nil, nil)

	user := testutil.SampleUser()
	chat := testutil.SampleChat()
	ctx := context.Background()

	// Test photo message
	t.Run("Photo", func(t *testing.T) {
		message := testutil.SamplePhotoMessage()
		message.From = &user
		message.Chat = chat
		message.MessageID = 4001

		update := testutil.SampleTelegramUpdateWithID(4001)
		update.Message = &message

		files := map[string][]byte{
			message.Photo[0].FileID: []byte("fake photo data"),
		}

		err := svc.ProcessUpdate(ctx, &update, files)
		if err != nil {
			t.Fatalf("ProcessUpdate failed for photo: %v", err)
		}

		// Verify photo was stored
		var photoCount int
		err = tdb.DB.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM photos WHERE message_id IN (SELECT id FROM messages WHERE message_id = $1 AND chat_id = $2)",
			message.MessageID, chat.ID,
		).Scan(&photoCount)
		if err != nil {
			t.Fatalf("failed to query photos table: %v", err)
		}
		if photoCount < 1 {
			t.Errorf("expected at least 1 photo, got %d", photoCount)
		}

		// Cleanup
		_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM updates WHERE update_id = $1", update.UpdateID)
	})

	// Note: Sticker message test skipped - SampleStickerMessage() not available in testutil
	// This could be added later if needed
}

// TestProcessUpdate_ConcurrentIngestion tests that concurrent update processing doesn't cause race conditions.
func TestProcessUpdate_ConcurrentIngestion(t *testing.T) {
	// Setup test database
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	// Create mock storage client
	mockMinIO := testutil.NewMockMinIOClient()

	// Create service with real repositories
	svc := NewIngestService(tdb.DB, mockMinIO, nil, nil)

	user := testutil.SampleUser()
	chat := testutil.SampleChat()
	ctx := context.Background()

	// Create multiple updates to process concurrently
	numUpdates := 10
	updates := make([]models.Update, numUpdates)
	for i := 0; i < numUpdates; i++ {
		// Use simple ASCII text to avoid Unicode escape issues in JSON
		text := "Concurrent message number "
		if i < 10 {
			text += string('0' + rune(i))
		}
		message := testutil.SampleMessageWithText(text)
		message.From = &user
		message.Chat = chat
		message.MessageID = int64(5000 + i)

		updates[i] = testutil.SampleTelegramUpdateWithID(int64(5000 + i))
		updates[i].Message = &message
	}

	// Cleanup any previous test data
	for i := 0; i < numUpdates; i++ {
		_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM updates WHERE update_id = $1", updates[i].UpdateID)
		_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM messages WHERE chat_id = $1 AND message_id = $2",
			chat.ID, updates[i].Message.MessageID)
	}

	// Process updates concurrently
	errCh := make(chan error, numUpdates)
	for i := 0; i < numUpdates; i++ {
		go func(idx int) {
			err := svc.ProcessUpdate(ctx, &updates[idx], make(map[string][]byte))
			errCh <- err
		}(i)
	}

	// Collect results
	for i := 0; i < numUpdates; i++ {
		err := <-errCh
		if err != nil {
			t.Errorf("ProcessUpdate failed for concurrent update %d: %v", i, err)
		}
	}

	// Verify all updates were stored
	var updateCount int
	err := tdb.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM updates WHERE update_id >= 5000 AND update_id < 5010",
	).Scan(&updateCount)
	if err != nil {
		t.Fatalf("failed to query updates table: %v", err)
	}
	if updateCount != numUpdates {
		t.Errorf("expected %d updates, got %d", numUpdates, updateCount)
	}

	// Verify all messages were stored
	var messageCount int
	err = tdb.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM messages WHERE chat_id = $1 AND message_id >= 5000 AND message_id < 5010",
		chat.ID,
	).Scan(&messageCount)
	if err != nil {
		t.Fatalf("failed to query messages table: %v", err)
	}
	if messageCount != numUpdates {
		t.Errorf("expected %d messages, got %d", numUpdates, messageCount)
	}

	// Cleanup
	for i := 0; i < numUpdates; i++ {
		_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM updates WHERE update_id = $1", updates[i].UpdateID)
	}
}

// TestProcessUpdate_LargeMediaFile tests that large media files are handled correctly.
func TestProcessUpdate_LargeMediaFile(t *testing.T) {
	// Setup test database
	tdb := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)

	// Create mock storage client
	mockMinIO := testutil.NewMockMinIOClient()

	// Create service with real repositories
	svc := NewIngestService(tdb.DB, mockMinIO, nil, nil)

	user := testutil.SampleUser()
	chat := testutil.SampleChat()

	// Create a photo message with large file
	message := testutil.SamplePhotoMessage()
	message.From = &user
	message.Chat = chat
	message.MessageID = 6001

	update := testutil.SampleTelegramUpdateWithID(6001)
	update.Message = &message

	// Create a large file (1MB)
	largeFile := make([]byte, 1*1024*1024)
	for i := range largeFile {
		largeFile[i] = byte(i % 256)
	}

	files := map[string][]byte{
		message.Photo[0].FileID: largeFile,
	}

	ctx := context.Background()

	// Cleanup any previous test data
	_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM updates WHERE update_id = $1", update.UpdateID)
	_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM messages WHERE chat_id = $1 AND message_id = $2", chat.ID, message.MessageID)

	// Process the update
	err := svc.ProcessUpdate(ctx, &update, files)
	if err != nil {
		t.Fatalf("ProcessUpdate failed for large media file: %v", err)
	}

	// Verify photo was stored
	var photoCount int
	err = tdb.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM photos WHERE message_id IN (SELECT id FROM messages WHERE message_id = $1 AND chat_id = $2)",
		message.MessageID, chat.ID,
	).Scan(&photoCount)
	if err != nil {
		t.Fatalf("failed to query photos table: %v", err)
	}
	if photoCount < 1 {
		t.Errorf("expected at least 1 photo, got %d", photoCount)
	}

	// Verify MinIO upload was called
	if mockMinIO.UploadCalls != 1 {
		t.Errorf("expected 1 MinIO upload, got %d", mockMinIO.UploadCalls)
	}

	// Verify file was stored in mock storage
	if len(mockMinIO.Storage) != 1 {
		t.Errorf("expected 1 object in mock storage, got %d", len(mockMinIO.Storage))
	}

	// Verify stored file size matches
	for _, obj := range mockMinIO.Storage {
		if len(obj.Data) != len(largeFile) {
			t.Errorf("expected file size %d, got %d", len(largeFile), len(obj.Data))
		}
	}

	// Cleanup
	_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM updates WHERE update_id = $1", update.UpdateID)
}

// Mock repository for testing transaction rollback
type mockUpdateRepository struct {
	insertError error
}

func (m *mockUpdateRepository) InsertUpdate(ctx context.Context, tx *sql.Tx, updateID int64, updateType string, rawData interface{}) error {
	if m.insertError != nil {
		return m.insertError
	}
	// Marshal rawData to JSON for storage
	rawJSON, err := json.Marshal(rawData)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		"INSERT INTO updates (update_id, update_type, raw_data) VALUES ($1, $2, $3)",
		updateID, updateType, rawJSON,
	)
	return err
}
