package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"beef-briefing/apps/api-service/internal/models"
)

// setupChatTestDB creates a new database connection for testing
func setupChatTestDB(t *testing.T) *sql.DB {
	t.Helper()

	host := getEnvOrDefaultChat("TEST_DB_HOST", "localhost")
	port := getEnvOrDefaultChat("TEST_DB_PORT", "5432")
	user := getEnvOrDefaultChat("TEST_DB_USER", "postgres")
	password := getEnvOrDefaultChat("TEST_DB_PASSWORD", "Vb96vpmAEZVdSpruALAgDmZrHFRDXATg")
	dbname := getEnvOrDefaultChat("TEST_DB_NAME", "beef_briefing")
	sslmode := getEnvOrDefaultChat("TEST_DB_SSLMODE", "disable")

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

// getEnvOrDefaultChat retrieves an environment variable or returns a default value
func getEnvOrDefaultChat(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// teardownChatTestDB closes the test database connection
func teardownChatTestDB(t *testing.T, db *sql.DB) {
	t.Helper()

	if db != nil {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close test database: %v", err)
		}
	}
}

// cleanupChatTables truncates the chats table for test isolation
func cleanupChatTables(t *testing.T, db *sql.DB) {
	t.Helper()

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "TRUNCATE TABLE chats CASCADE"); err != nil {
		t.Fatalf("failed to truncate chats table: %v", err)
	}
}

// samplePrivateChat creates a test private chat
func samplePrivateChat() models.Chat {
	return models.Chat{
		ID:        123456789,
		Type:      "private",
		FirstName: "John",
		LastName:  "Doe",
		Username:  "johndoe",
	}
}

// sampleGroupChat creates a test group chat
func sampleGroupChat() models.Chat {
	return models.Chat{
		ID:    -1001234567890,
		Type:  "group",
		Title: "Test Group",
	}
}

// sampleSupergroupChat creates a test supergroup chat
func sampleSupergroupChat() models.Chat {
	return models.Chat{
		ID:       -1009876543210,
		Type:     "supergroup",
		Title:    "Test Supergroup",
		Username: "testsupergroup",
		IsForum:  false,
	}
}

// sampleChannelChat creates a test channel chat
func sampleChannelChat() models.Chat {
	return models.Chat{
		ID:       -1005555555555,
		Type:     "channel",
		Title:    "Test Channel",
		Username: "testchannel",
	}
}

// TestUpsertChatPrivateType tests inserting a private chat
func TestUpsertChatPrivateType(t *testing.T) {
	db := setupChatTestDB(t)
	defer teardownChatTestDB(t, db)
	defer cleanupChatTables(t, db)

	ctx := context.Background()
	cleanupChatTables(t, db)

	// Create a chat repository without NewRelic (nil is safe)
	repo := NewChatRepository(db, nil)

	// Create a sample private chat
	chat := samplePrivateChat()

	// Begin transaction for test isolation
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Upsert the chat
	err = repo.UpsertChat(ctx, tx, &chat)
	if err != nil {
		t.Fatalf("UpsertChat failed: %v", err)
	}

	// Verify the chat was inserted correctly
	var dbChat struct {
		ID        int64
		Type      string
		FirstName sql.NullString
		LastName  sql.NullString
		Username  sql.NullString
		Title     sql.NullString
		IsForum   bool
	}

	query := `SELECT id, type, first_name, last_name, username, title, is_forum FROM chats WHERE id = $1`
	err = tx.QueryRowContext(ctx, query, chat.ID).Scan(
		&dbChat.ID, &dbChat.Type, &dbChat.FirstName, &dbChat.LastName,
		&dbChat.Username, &dbChat.Title, &dbChat.IsForum,
	)
	if err != nil {
		t.Fatalf("failed to query chat: %v", err)
	}

	// Assert all fields were inserted correctly
	if dbChat.ID != chat.ID {
		t.Errorf("ID mismatch: got %d, want %d", dbChat.ID, chat.ID)
	}
	if dbChat.Type != chat.Type {
		t.Errorf("Type mismatch: got %q, want %q", dbChat.Type, chat.Type)
	}
	if dbChat.FirstName.String != chat.FirstName || !dbChat.FirstName.Valid {
		t.Errorf("FirstName mismatch: got %q (valid=%v), want %q", dbChat.FirstName.String, dbChat.FirstName.Valid, chat.FirstName)
	}
	if dbChat.LastName.String != chat.LastName || !dbChat.LastName.Valid {
		t.Errorf("LastName mismatch: got %q (valid=%v), want %q", dbChat.LastName.String, dbChat.LastName.Valid, chat.LastName)
	}
	if dbChat.Username.String != chat.Username || !dbChat.Username.Valid {
		t.Errorf("Username mismatch: got %q (valid=%v), want %q", dbChat.Username.String, dbChat.Username.Valid, chat.Username)
	}
	if dbChat.Title.Valid {
		t.Errorf("Title should be NULL for private chat but got %q", dbChat.Title.String)
	}
	if dbChat.IsForum != chat.IsForum {
		t.Errorf("IsForum mismatch: got %v, want %v", dbChat.IsForum, chat.IsForum)
	}
}

// TestUpsertChatGroupType tests inserting a group chat
func TestUpsertChatGroupType(t *testing.T) {
	db := setupChatTestDB(t)
	defer teardownChatTestDB(t, db)
	defer cleanupChatTables(t, db)

	ctx := context.Background()
	cleanupChatTables(t, db)

	// Create a chat repository without NewRelic (nil is safe)
	repo := NewChatRepository(db, nil)

	// Create a sample group chat
	chat := sampleGroupChat()

	// Begin transaction for test isolation
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Upsert the chat
	err = repo.UpsertChat(ctx, tx, &chat)
	if err != nil {
		t.Fatalf("UpsertChat failed: %v", err)
	}

	// Verify the chat was inserted correctly
	var dbChat struct {
		ID    int64
		Type  string
		Title sql.NullString
	}

	query := `SELECT id, type, title FROM chats WHERE id = $1`
	err = tx.QueryRowContext(ctx, query, chat.ID).Scan(&dbChat.ID, &dbChat.Type, &dbChat.Title)
	if err != nil {
		t.Fatalf("failed to query chat: %v", err)
	}

	// Assert all fields were inserted correctly
	if dbChat.ID != chat.ID {
		t.Errorf("ID mismatch: got %d, want %d", dbChat.ID, chat.ID)
	}
	if dbChat.Type != chat.Type {
		t.Errorf("Type mismatch: got %q, want %q", dbChat.Type, chat.Type)
	}
	if dbChat.Title.String != chat.Title || !dbChat.Title.Valid {
		t.Errorf("Title mismatch: got %q (valid=%v), want %q", dbChat.Title.String, dbChat.Title.Valid, chat.Title)
	}
}

// TestUpsertChatSupergroupType tests inserting a supergroup chat
func TestUpsertChatSupergroupType(t *testing.T) {
	db := setupChatTestDB(t)
	defer teardownChatTestDB(t, db)
	defer cleanupChatTables(t, db)

	ctx := context.Background()
	cleanupChatTables(t, db)

	// Create a chat repository without NewRelic (nil is safe)
	repo := NewChatRepository(db, nil)

	// Create a sample supergroup chat
	chat := sampleSupergroupChat()

	// Begin transaction for test isolation
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Upsert the chat
	err = repo.UpsertChat(ctx, tx, &chat)
	if err != nil {
		t.Fatalf("UpsertChat failed: %v", err)
	}

	// Verify the chat was inserted correctly
	var dbChat struct {
		ID       int64
		Type     string
		Title    sql.NullString
		Username sql.NullString
		IsForum  bool
	}

	query := `SELECT id, type, title, username, is_forum FROM chats WHERE id = $1`
	err = tx.QueryRowContext(ctx, query, chat.ID).Scan(
		&dbChat.ID, &dbChat.Type, &dbChat.Title, &dbChat.Username, &dbChat.IsForum,
	)
	if err != nil {
		t.Fatalf("failed to query chat: %v", err)
	}

	// Assert all fields were inserted correctly
	if dbChat.ID != chat.ID {
		t.Errorf("ID mismatch: got %d, want %d", dbChat.ID, chat.ID)
	}
	if dbChat.Type != chat.Type {
		t.Errorf("Type mismatch: got %q, want %q", dbChat.Type, chat.Type)
	}
	if dbChat.Title.String != chat.Title || !dbChat.Title.Valid {
		t.Errorf("Title mismatch: got %q (valid=%v), want %q", dbChat.Title.String, dbChat.Title.Valid, chat.Title)
	}
	if dbChat.Username.String != chat.Username || !dbChat.Username.Valid {
		t.Errorf("Username mismatch: got %q (valid=%v), want %q", dbChat.Username.String, dbChat.Username.Valid, chat.Username)
	}
	if dbChat.IsForum != chat.IsForum {
		t.Errorf("IsForum mismatch: got %v, want %v", dbChat.IsForum, chat.IsForum)
	}
}

// TestUpsertChatChannelType tests inserting a channel chat
func TestUpsertChatChannelType(t *testing.T) {
	db := setupChatTestDB(t)
	defer teardownChatTestDB(t, db)
	defer cleanupChatTables(t, db)

	ctx := context.Background()
	cleanupChatTables(t, db)

	// Create a chat repository without NewRelic (nil is safe)
	repo := NewChatRepository(db, nil)

	// Create a sample channel chat
	chat := sampleChannelChat()

	// Begin transaction for test isolation
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Upsert the chat
	err = repo.UpsertChat(ctx, tx, &chat)
	if err != nil {
		t.Fatalf("UpsertChat failed: %v", err)
	}

	// Verify the chat was inserted correctly
	var dbChat struct {
		ID       int64
		Type     string
		Title    sql.NullString
		Username sql.NullString
	}

	query := `SELECT id, type, title, username FROM chats WHERE id = $1`
	err = tx.QueryRowContext(ctx, query, chat.ID).Scan(
		&dbChat.ID, &dbChat.Type, &dbChat.Title, &dbChat.Username,
	)
	if err != nil {
		t.Fatalf("failed to query chat: %v", err)
	}

	// Assert all fields were inserted correctly
	if dbChat.ID != chat.ID {
		t.Errorf("ID mismatch: got %d, want %d", dbChat.ID, chat.ID)
	}
	if dbChat.Type != chat.Type {
		t.Errorf("Type mismatch: got %q, want %q", dbChat.Type, chat.Type)
	}
	if dbChat.Title.String != chat.Title || !dbChat.Title.Valid {
		t.Errorf("Title mismatch: got %q (valid=%v), want %q", dbChat.Title.String, dbChat.Title.Valid, chat.Title)
	}
	if dbChat.Username.String != chat.Username || !dbChat.Username.Valid {
		t.Errorf("Username mismatch: got %q (valid=%v), want %q", dbChat.Username.String, dbChat.Username.Valid, chat.Username)
	}
}

// TestUpsertChatUpdateExisting tests updating an existing chat
func TestUpsertChatUpdateExisting(t *testing.T) {
	db := setupChatTestDB(t)
	defer teardownChatTestDB(t, db)
	defer cleanupChatTables(t, db)

	ctx := context.Background()
	cleanupChatTables(t, db)

	// Create a chat repository without NewRelic (nil is safe)
	repo := NewChatRepository(db, nil)

	// Create initial chat
	chat := sampleSupergroupChat()

	// Begin transaction for test isolation
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Insert initial chat
	err = repo.UpsertChat(ctx, tx, &chat)
	if err != nil {
		t.Fatalf("initial UpsertChat failed: %v", err)
	}

	// Update the chat with new values
	chat.Title = "Updated Supergroup Title"
	chat.Username = "updatedsupergroup"
	chat.IsForum = true

	// Upsert again (should update)
	err = repo.UpsertChat(ctx, tx, &chat)
	if err != nil {
		t.Fatalf("update UpsertChat failed: %v", err)
	}

	// Verify the chat was updated correctly
	var dbChat struct {
		Title    sql.NullString
		Username sql.NullString
		IsForum  bool
	}

	query := `SELECT title, username, is_forum FROM chats WHERE id = $1`
	err = tx.QueryRowContext(ctx, query, chat.ID).Scan(
		&dbChat.Title, &dbChat.Username, &dbChat.IsForum,
	)
	if err != nil {
		t.Fatalf("failed to query chat: %v", err)
	}

	// Assert all fields were updated correctly
	if dbChat.Title.String != chat.Title {
		t.Errorf("Title not updated: got %q, want %q", dbChat.Title.String, chat.Title)
	}
	if dbChat.Username.String != chat.Username {
		t.Errorf("Username not updated: got %q, want %q", dbChat.Username.String, chat.Username)
	}
	if dbChat.IsForum != chat.IsForum {
		t.Errorf("IsForum not updated: got %v, want %v", dbChat.IsForum, chat.IsForum)
	}
}

// TestLinkMigratedChatSuccess tests linking a migrated chat (group → supergroup)
func TestLinkMigratedChatSuccess(t *testing.T) {
	db := setupChatTestDB(t)
	defer teardownChatTestDB(t, db)
	defer cleanupChatTables(t, db)

	ctx := context.Background()
	cleanupChatTables(t, db)

	// Create a chat repository without NewRelic (nil is safe)
	repo := NewChatRepository(db, nil)

	// Begin transaction for test isolation
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Create old group chat
	oldGroupID := int64(-1001111111111)
	oldGroup := models.Chat{
		ID:    oldGroupID,
		Type:  "group",
		Title: "Old Group",
	}
	err = repo.UpsertChat(ctx, tx, &oldGroup)
	if err != nil {
		t.Fatalf("failed to insert old group: %v", err)
	}

	// Create new supergroup chat
	newSupergroupID := int64(-1002222222222)
	newSupergroup := models.Chat{
		ID:    newSupergroupID,
		Type:  "supergroup",
		Title: "Migrated Supergroup",
	}
	err = repo.UpsertChat(ctx, tx, &newSupergroup)
	if err != nil {
		t.Fatalf("failed to insert new supergroup: %v", err)
	}

	// Link the migration
	err = repo.LinkMigratedChat(ctx, tx, newSupergroupID, oldGroupID)
	if err != nil {
		t.Fatalf("LinkMigratedChat failed: %v", err)
	}

	// Verify the migration link was created
	var migratedFromChatID sql.NullInt64
	query := `SELECT migrated_from_chat_id FROM chats WHERE id = $1`
	err = tx.QueryRowContext(ctx, query, newSupergroupID).Scan(&migratedFromChatID)
	if err != nil {
		t.Fatalf("failed to query migrated chat: %v", err)
	}

	// Assert migration link is correct
	if !migratedFromChatID.Valid {
		t.Errorf("migrated_from_chat_id should not be NULL")
	}
	if migratedFromChatID.Int64 != oldGroupID {
		t.Errorf("migrated_from_chat_id mismatch: got %d, want %d", migratedFromChatID.Int64, oldGroupID)
	}
}

// TestLinkMigratedChatNonExistentChat tests linking with a non-existent chat ID
func TestLinkMigratedChatNonExistentChat(t *testing.T) {
	db := setupChatTestDB(t)
	defer teardownChatTestDB(t, db)
	defer cleanupChatTables(t, db)

	ctx := context.Background()
	cleanupChatTables(t, db)

	// Create a chat repository without NewRelic (nil is safe)
	repo := NewChatRepository(db, nil)

	// Begin transaction for test isolation
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Try to link migration for non-existent chat
	// This should succeed but not actually link anything (UPDATE affects 0 rows)
	nonExistentChatID := int64(-1009999999999)
	oldGroupID := int64(-1008888888888)

	err = repo.LinkMigratedChat(ctx, tx, nonExistentChatID, oldGroupID)
	// The function doesn't check rows affected, so this won't error
	// This is actually valid behavior - if the chat doesn't exist yet, we can't link it
	if err != nil {
		t.Fatalf("LinkMigratedChat unexpectedly failed: %v", err)
	}
}

// TestUpsertChatNullFieldHandling tests NULL field handling for optional fields
func TestUpsertChatNullFieldHandling(t *testing.T) {
	db := setupChatTestDB(t)
	defer teardownChatTestDB(t, db)
	defer cleanupChatTables(t, db)

	ctx := context.Background()
	cleanupChatTables(t, db)

	// Create a chat repository without NewRelic (nil is safe)
	repo := NewChatRepository(db, nil)

	// Create a chat with minimal fields (empty optional fields should be NULL)
	chat := models.Chat{
		ID:        -1003333333333,
		Type:      "supergroup",
		Title:     "", // empty - should be NULL
		Username:  "", // empty - should be NULL
		FirstName: "", // empty - should be NULL
		LastName:  "", // empty - should be NULL
		IsForum:   false,
	}

	// Begin transaction for test isolation
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	err = repo.UpsertChat(ctx, tx, &chat)
	if err != nil {
		t.Fatalf("UpsertChat failed: %v", err)
	}

	// Verify NULL fields are properly stored
	var dbChat struct {
		Title     sql.NullString
		Username  sql.NullString
		FirstName sql.NullString
		LastName  sql.NullString
	}

	query := `SELECT title, username, first_name, last_name FROM chats WHERE id = $1`
	err = tx.QueryRowContext(ctx, query, chat.ID).Scan(
		&dbChat.Title, &dbChat.Username, &dbChat.FirstName, &dbChat.LastName,
	)
	if err != nil {
		t.Fatalf("failed to query chat: %v", err)
	}

	// Assert all optional fields are NULL
	if dbChat.Title.Valid {
		t.Errorf("Title should be NULL but got %q", dbChat.Title.String)
	}
	if dbChat.Username.Valid {
		t.Errorf("Username should be NULL but got %q", dbChat.Username.String)
	}
	if dbChat.FirstName.Valid {
		t.Errorf("FirstName should be NULL but got %q", dbChat.FirstName.String)
	}
	if dbChat.LastName.Valid {
		t.Errorf("LastName should be NULL but got %q", dbChat.LastName.String)
	}
}
