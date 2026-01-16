package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"beef-briefing/apps/api-service/internal/models"
)

// setupUserTestDB creates a new database connection for testing
func setupUserTestDB(t *testing.T) *sql.DB {
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

// teardownUserTestDB closes the test database connection
func teardownUserTestDB(t *testing.T, db *sql.DB) {
	t.Helper()

	if db != nil {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close test database: %v", err)
		}
	}
}

// cleanupUserTables truncates the users table for test isolation
func cleanupUserTables(t *testing.T, db *sql.DB) {
	t.Helper()

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "TRUNCATE TABLE users CASCADE"); err != nil {
		t.Fatalf("failed to truncate users table: %v", err)
	}
}

// sampleUser creates a test user with standard values
func sampleUser() models.User {
	return models.User{
		ID:           123456789,
		IsBot:        false,
		FirstName:    "Test",
		LastName:     "User",
		Username:     "testuser",
		LanguageCode: "en",
		IsPremium:    false,
	}
}

// TestUpsertUserInsertNew tests inserting a new user into the database
func TestUpsertUserInsertNew(t *testing.T) {
	db := setupUserTestDB(t)
	defer teardownUserTestDB(t, db)
	defer cleanupUserTables(t, db)

	ctx := context.Background()
	cleanupUserTables(t, db)

	// Create a user repository without NewRelic (nil is safe)
	repo := NewUserRepository(db, nil)

	// Create a sample user
	user := sampleUser()

	// Begin transaction for test isolation
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Upsert the user
	err = repo.UpsertUser(ctx, tx, &user)
	if err != nil {
		t.Fatalf("UpsertUser failed: %v", err)
	}

	// Verify the user was inserted correctly
	var dbUser struct {
		ID           int64
		IsBot        bool
		FirstName    string
		LastName     sql.NullString
		Username     sql.NullString
		LanguageCode sql.NullString
		IsPremium    bool
	}

	query := `SELECT id, is_bot, first_name, last_name, username, language_code, is_premium FROM users WHERE id = $1`
	err = tx.QueryRowContext(ctx, query, user.ID).Scan(
		&dbUser.ID, &dbUser.IsBot, &dbUser.FirstName, &dbUser.LastName,
		&dbUser.Username, &dbUser.LanguageCode, &dbUser.IsPremium,
	)
	if err != nil {
		t.Fatalf("failed to query user: %v", err)
	}

	// Assert all fields were inserted correctly
	if dbUser.ID != user.ID {
		t.Errorf("ID mismatch: got %d, want %d", dbUser.ID, user.ID)
	}
	if dbUser.IsBot != user.IsBot {
		t.Errorf("IsBot mismatch: got %v, want %v", dbUser.IsBot, user.IsBot)
	}
	if dbUser.FirstName != user.FirstName {
		t.Errorf("FirstName mismatch: got %q, want %q", dbUser.FirstName, user.FirstName)
	}
	if dbUser.LastName.String != user.LastName || !dbUser.LastName.Valid {
		t.Errorf("LastName mismatch: got %q (valid=%v), want %q", dbUser.LastName.String, dbUser.LastName.Valid, user.LastName)
	}
	if dbUser.Username.String != user.Username || !dbUser.Username.Valid {
		t.Errorf("Username mismatch: got %q (valid=%v), want %q", dbUser.Username.String, dbUser.Username.Valid, user.Username)
	}
	if dbUser.LanguageCode.String != user.LanguageCode || !dbUser.LanguageCode.Valid {
		t.Errorf("LanguageCode mismatch: got %q (valid=%v), want %q", dbUser.LanguageCode.String, dbUser.LanguageCode.Valid, user.LanguageCode)
	}
	if dbUser.IsPremium != user.IsPremium {
		t.Errorf("IsPremium mismatch: got %v, want %v", dbUser.IsPremium, user.IsPremium)
	}
}

// TestUpsertUserUpdateExisting tests updating an existing user in the database
func TestUpsertUserUpdateExisting(t *testing.T) {
	db := setupUserTestDB(t)
	defer teardownUserTestDB(t, db)
	defer cleanupUserTables(t, db)

	ctx := context.Background()
	cleanupUserTables(t, db)

	// Create a user repository without NewRelic (nil is safe)
	repo := NewUserRepository(db, nil)

	// Create initial user
	user := sampleUser()

	// Begin transaction for test isolation
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	err = repo.UpsertUser(ctx, tx, &user)
	if err != nil {
		t.Fatalf("initial UpsertUser failed: %v", err)
	}

	// Update the user with new values
	user.FirstName = "UpdatedFirstName"
	user.LastName = "UpdatedLastName"
	user.Username = "updatedusername"
	user.IsBot = true
	user.IsPremium = true

	err = repo.UpsertUser(ctx, tx, &user)
	if err != nil {
		t.Fatalf("update UpsertUser failed: %v", err)
	}

	// Verify the user was updated correctly
	var dbUser struct {
		FirstName string
		LastName  sql.NullString
		Username  sql.NullString
		IsBot     bool
		IsPremium bool
	}

	query := `SELECT first_name, last_name, username, is_bot, is_premium FROM users WHERE id = $1`
	err = tx.QueryRowContext(ctx, query, user.ID).Scan(
		&dbUser.FirstName, &dbUser.LastName, &dbUser.Username, &dbUser.IsBot, &dbUser.IsPremium,
	)
	if err != nil {
		t.Fatalf("failed to query user: %v", err)
	}

	// Assert all fields were updated correctly
	if dbUser.FirstName != user.FirstName {
		t.Errorf("FirstName not updated: got %q, want %q", dbUser.FirstName, user.FirstName)
	}
	if dbUser.LastName.String != user.LastName {
		t.Errorf("LastName not updated: got %q, want %q", dbUser.LastName.String, user.LastName)
	}
	if dbUser.Username.String != user.Username {
		t.Errorf("Username not updated: got %q, want %q", dbUser.Username.String, user.Username)
	}
	if dbUser.IsBot != user.IsBot {
		t.Errorf("IsBot not updated: got %v, want %v", dbUser.IsBot, user.IsBot)
	}
	if dbUser.IsPremium != user.IsPremium {
		t.Errorf("IsPremium not updated: got %v, want %v", dbUser.IsPremium, user.IsPremium)
	}
}

// TestUpsertUserNullFieldHandling tests NULL field handling for optional fields
func TestUpsertUserNullFieldHandling(t *testing.T) {
	db := setupUserTestDB(t)
	defer teardownUserTestDB(t, db)
	defer cleanupUserTables(t, db)

	ctx := context.Background()
	cleanupUserTables(t, db)

	// Create a user repository without NewRelic (nil is safe)
	repo := NewUserRepository(db, nil)

	// Create user with all optional fields empty/null
	user := models.User{
		ID:           999888777,
		IsBot:        false,
		FirstName:    "OnlyFirstName",
		LastName:     "",        // empty - should be NULL
		Username:     "",        // empty - should be NULL
		LanguageCode: "",        // empty - should be NULL
		IsPremium:    false,
	}

	// Begin transaction for test isolation
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	err = repo.UpsertUser(ctx, tx, &user)
	if err != nil {
		t.Fatalf("UpsertUser failed: %v", err)
	}

	// Verify NULL fields are properly stored
	var dbUser struct {
		LastName     sql.NullString
		Username     sql.NullString
		LanguageCode sql.NullString
	}

	query := `SELECT last_name, username, language_code FROM users WHERE id = $1`
	err = tx.QueryRowContext(ctx, query, user.ID).Scan(
		&dbUser.LastName, &dbUser.Username, &dbUser.LanguageCode,
	)
	if err != nil {
		t.Fatalf("failed to query user: %v", err)
	}

	// Assert all optional fields are NULL
	if dbUser.LastName.Valid {
		t.Errorf("LastName should be NULL but got %q", dbUser.LastName.String)
	}
	if dbUser.Username.Valid {
		t.Errorf("Username should be NULL but got %q", dbUser.Username.String)
	}
	if dbUser.LanguageCode.Valid {
		t.Errorf("LanguageCode should be NULL but got %q", dbUser.LanguageCode.String)
	}
}

// TestUpsertUserMultipleUsers tests inserting multiple users
func TestUpsertUserMultipleUsers(t *testing.T) {
	db := setupUserTestDB(t)
	defer teardownUserTestDB(t, db)
	defer cleanupUserTables(t, db)

	ctx := context.Background()
	cleanupUserTables(t, db)

	// Create a user repository without NewRelic (nil is safe)
	repo := NewUserRepository(db, nil)

	// Begin transaction for test isolation
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Create multiple users
	users := []models.User{
		{
			ID:           100001,
			IsBot:        false,
			FirstName:    "Alice",
			LastName:     "Smith",
			Username:     "alice_smith",
			LanguageCode: "en",
			IsPremium:    false,
		},
		{
			ID:           100002,
			IsBot:        false,
			FirstName:    "Bob",
			LastName:     "Jones",
			Username:     "bob_jones",
			LanguageCode: "es",
			IsPremium:    true,
		},
		{
			ID:           100003,
			IsBot:        true,
			FirstName:    "BotUser",
			Username:     "bot_user",
			LanguageCode: "en",
			IsPremium:    false,
		},
	}

	// Insert all users
	for _, user := range users {
		err := repo.UpsertUser(ctx, tx, &user)
		if err != nil {
			t.Fatalf("UpsertUser failed for user %d: %v", user.ID, err)
		}
	}

	// Verify all users were inserted
	var count int
	err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE id IN ($1, $2, $3)", 100001, 100002, 100003).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count users: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 users, got %d", count)
	}
}

// TestUpsertUserBotUser tests inserting a bot user
func TestUpsertUserBotUser(t *testing.T) {
	db := setupUserTestDB(t)
	defer teardownUserTestDB(t, db)
	defer cleanupUserTables(t, db)

	ctx := context.Background()
	cleanupUserTables(t, db)

	// Create a user repository without NewRelic (nil is safe)
	repo := NewUserRepository(db, nil)

	// Create a bot user
	bot := models.User{
		ID:        555666777,
		IsBot:     true,
		FirstName: "MyBot",
		Username:  "my_bot",
	}

	// Begin transaction for test isolation
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	err = repo.UpsertUser(ctx, tx, &bot)
	if err != nil {
		t.Fatalf("UpsertUser failed: %v", err)
	}

	// Verify the bot user was inserted
	var dbBot struct {
		IsBot     bool
		FirstName string
		Username  sql.NullString
	}

	query := `SELECT is_bot, first_name, username FROM users WHERE id = $1`
	err = tx.QueryRowContext(ctx, query, bot.ID).Scan(&dbBot.IsBot, &dbBot.FirstName, &dbBot.Username)
	if err != nil {
		t.Fatalf("failed to query bot: %v", err)
	}

	// Assert bot fields
	if !dbBot.IsBot {
		t.Errorf("IsBot should be true, got %v", dbBot.IsBot)
	}
	if dbBot.FirstName != bot.FirstName {
		t.Errorf("FirstName mismatch: got %q, want %q", dbBot.FirstName, bot.FirstName)
	}
	if dbBot.Username.String != bot.Username {
		t.Errorf("Username mismatch: got %q, want %q", dbBot.Username.String, bot.Username)
	}
}

// TestUpsertUserPremiumUser tests inserting a premium user
func TestUpsertUserPremiumUser(t *testing.T) {
	db := setupUserTestDB(t)
	defer teardownUserTestDB(t, db)
	defer cleanupUserTables(t, db)

	ctx := context.Background()
	cleanupUserTables(t, db)

	// Create a user repository without NewRelic (nil is safe)
	repo := NewUserRepository(db, nil)

	// Create a premium user
	premium := models.User{
		ID:           777888999,
		IsBot:        false,
		FirstName:    "Premium",
		LastName:     "User",
		Username:     "premium_user",
		LanguageCode: "en",
		IsPremium:    true,
	}

	// Begin transaction for test isolation
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	err = repo.UpsertUser(ctx, tx, &premium)
	if err != nil {
		t.Fatalf("UpsertUser failed: %v", err)
	}

	// Verify the premium user was inserted
	var isPremium bool
	query := `SELECT is_premium FROM users WHERE id = $1`
	err = tx.QueryRowContext(ctx, query, premium.ID).Scan(&isPremium)
	if err != nil {
		t.Fatalf("failed to query user: %v", err)
	}

	if !isPremium {
		t.Errorf("IsPremium should be true, got %v", isPremium)
	}
}
