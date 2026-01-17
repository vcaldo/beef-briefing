package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"beef-briefing/apps/api-service/internal/models"
)

// setupProfilePhotoTestDB creates a new database connection for testing
func setupProfilePhotoTestDB(t *testing.T) *sql.DB {
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

// teardownProfilePhotoTestDB closes the test database connection
func teardownProfilePhotoTestDB(t *testing.T, db *sql.DB) {
	t.Helper()

	if db != nil {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close test database: %v", err)
		}
	}
}

// cleanupProfilePhotoTables truncates profile photo tables for test isolation
func cleanupProfilePhotoTables(t *testing.T, db *sql.DB) {
	t.Helper()

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, "TRUNCATE TABLE user_profile_photos, chat_profile_photos, users, chats CASCADE"); err != nil {
		t.Fatalf("failed to truncate profile photo tables: %v", err)
	}
}

// setupProfilePhotoTestData inserts required foreign key data (user and chat)
func setupProfilePhotoTestData(t *testing.T, db *sql.DB) (int64, int64) {
	t.Helper()

	ctx := context.Background()

	// Insert test user
	userID := int64(123456789)
	_, err := db.ExecContext(ctx, `
		INSERT INTO users (id, is_bot, first_name)
		VALUES ($1, false, 'Test User')
	`, userID)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}

	// Insert test chat
	chatID := int64(-1002345678901)
	_, err = db.ExecContext(ctx, `
		INSERT INTO chats (id, type, title)
		VALUES ($1, 'supergroup', 'Test Chat')
	`, chatID)
	if err != nil {
		t.Fatalf("failed to insert test chat: %v", err)
	}

	return userID, chatID
}

// sampleUserPhoto creates a test user profile photo with standard values
func sampleUserPhoto(userID int64) models.DBUserProfilePhoto {
	return models.DBUserProfilePhoto{
		UserID:               userID,
		TelegramFileID:       "AgACAgIAAxkBAAIBXmZkZGRkZGRkZGRkZGRkZGRkZGRkAAI",
		TelegramFileUniqueID: "AQADAgADmQEAAw",
		MinIOObjectKey:       "profile_photos/ab/abc123def456",
		FileHash:             "abc123def456",
		Width:                640,
		Height:               640,
		FileSize:             sql.NullInt64{Int64: 50000, Valid: true},
	}
}

// sampleChatPhoto creates a test chat profile photo with standard values
func sampleChatPhoto(chatID int64, photoSize string) models.DBChatProfilePhoto {
	return models.DBChatProfilePhoto{
		ChatID:               chatID,
		PhotoSize:            photoSize,
		TelegramFileID:       "AgACAgIAAxkBAAIBXmZkZGRkZGRkZGRkZGRkZGRkZGRkAAI",
		TelegramFileUniqueID: "AQADAgADmQEAAw",
		MinIOObjectKey:       "profile_photos/cd/cde456fgh789",
		FileHash:             "cde456fgh789",
		Width:                sql.NullInt32{Int32: 640, Valid: true},
		Height:               sql.NullInt32{Int32: 640, Valid: true},
		FileSize:             sql.NullInt64{Int64: 45000, Valid: true},
	}
}

// TestReplaceUserPhotosNew tests replacing user photos (first time insertion)
func TestReplaceUserPhotosNew(t *testing.T) {
	db := setupProfilePhotoTestDB(t)
	defer teardownProfilePhotoTestDB(t, db)
	defer cleanupProfilePhotoTables(t, db)

	userID, _ := setupProfilePhotoTestData(t, db)
	repo := NewProfilePhotoRepository(db, nil)
	ctx := context.Background()

	// Start transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Create test photos (3 sizes)
	photos := []models.DBUserProfilePhoto{
		{
			UserID:               userID,
			TelegramFileID:       "file_large",
			TelegramFileUniqueID: "unique_large",
			MinIOObjectKey:       "profile_photos/ab/abc123_large",
			FileHash:             "abc123_large",
			Width:                1280,
			Height:               1280,
			FileSize:             sql.NullInt64{Int64: 150000, Valid: true},
		},
		{
			UserID:               userID,
			TelegramFileID:       "file_medium",
			TelegramFileUniqueID: "unique_medium",
			MinIOObjectKey:       "profile_photos/ab/abc123_medium",
			FileHash:             "abc123_medium",
			Width:                640,
			Height:               640,
			FileSize:             sql.NullInt64{Int64: 50000, Valid: true},
		},
		{
			UserID:               userID,
			TelegramFileID:       "file_small",
			TelegramFileUniqueID: "unique_small",
			MinIOObjectKey:       "profile_photos/ab/abc123_small",
			FileHash:             "abc123_small",
			Width:                160,
			Height:               160,
			FileSize:             sql.NullInt64{Int64: 10000, Valid: true},
		},
	}

	// Replace (insert new) user photos
	err = repo.ReplaceUserPhotos(ctx, tx, userID, photos)
	if err != nil {
		t.Fatalf("ReplaceUserPhotos failed: %v", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	// Verify photos were inserted
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_profile_photos WHERE user_id = $1", userID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count user photos: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 user photos, got %d", count)
	}

	// Verify hash deduplication pattern
	var hash string
	err = db.QueryRowContext(ctx, "SELECT file_hash FROM user_profile_photos WHERE user_id = $1 AND width = 1280", userID).Scan(&hash)
	if err != nil {
		t.Fatalf("failed to query file hash: %v", err)
	}
	if hash != "abc123_large" {
		t.Errorf("expected file_hash 'abc123_large', got '%s'", hash)
	}
}

// TestReplaceUserPhotosReplace tests replacing existing user photos
func TestReplaceUserPhotosReplace(t *testing.T) {
	db := setupProfilePhotoTestDB(t)
	defer teardownProfilePhotoTestDB(t, db)
	defer cleanupProfilePhotoTables(t, db)

	userID, _ := setupProfilePhotoTestData(t, db)
	repo := NewProfilePhotoRepository(db, nil)
	ctx := context.Background()

	// Insert initial photos
	tx1, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	initialPhotos := []models.DBUserProfilePhoto{
		{
			UserID:               userID,
			TelegramFileID:       "old_file",
			TelegramFileUniqueID: "old_unique",
			MinIOObjectKey:       "profile_photos/old/old123",
			FileHash:             "old123",
			Width:                640,
			Height:               640,
			FileSize:             sql.NullInt64{Int64: 50000, Valid: true},
		},
	}
	err = repo.ReplaceUserPhotos(ctx, tx1, userID, initialPhotos)
	if err != nil {
		t.Fatalf("initial ReplaceUserPhotos failed: %v", err)
	}
	tx1.Commit()

	// Replace with new photos
	tx2, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx2.Rollback()

	newPhotos := []models.DBUserProfilePhoto{
		{
			UserID:               userID,
			TelegramFileID:       "new_file_1",
			TelegramFileUniqueID: "new_unique_1",
			MinIOObjectKey:       "profile_photos/new/new123_1",
			FileHash:             "new123_1",
			Width:                1280,
			Height:               1280,
			FileSize:             sql.NullInt64{Int64: 120000, Valid: true},
		},
		{
			UserID:               userID,
			TelegramFileID:       "new_file_2",
			TelegramFileUniqueID: "new_unique_2",
			MinIOObjectKey:       "profile_photos/new/new123_2",
			FileHash:             "new123_2",
			Width:                320,
			Height:               320,
			FileSize:             sql.NullInt64{Int64: 30000, Valid: true},
		},
	}
	err = repo.ReplaceUserPhotos(ctx, tx2, userID, newPhotos)
	if err != nil {
		t.Fatalf("replacement ReplaceUserPhotos failed: %v", err)
	}
	tx2.Commit()

	// Verify old photos were deleted and new photos were inserted
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_profile_photos WHERE user_id = $1", userID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count user photos: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 user photos after replacement, got %d", count)
	}

	// Verify old photo is gone
	var oldCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_profile_photos WHERE user_id = $1 AND file_hash = 'old123'", userID).Scan(&oldCount)
	if err != nil {
		t.Fatalf("failed to count old photos: %v", err)
	}
	if oldCount != 0 {
		t.Errorf("expected 0 old photos, got %d", oldCount)
	}

	// Verify new photos are present
	var newCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_profile_photos WHERE user_id = $1 AND file_hash LIKE 'new%'", userID).Scan(&newCount)
	if err != nil {
		t.Fatalf("failed to count new photos: %v", err)
	}
	if newCount != 2 {
		t.Errorf("expected 2 new photos, got %d", newCount)
	}
}

// TestReplaceChatPhotosNew tests replacing chat photos (first time insertion)
func TestReplaceChatPhotosNew(t *testing.T) {
	db := setupProfilePhotoTestDB(t)
	defer teardownProfilePhotoTestDB(t, db)
	defer cleanupProfilePhotoTables(t, db)

	_, chatID := setupProfilePhotoTestData(t, db)
	repo := NewProfilePhotoRepository(db, nil)
	ctx := context.Background()

	// Start transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Create test photos (big and small sizes)
	photos := []models.DBChatProfilePhoto{
		{
			ChatID:               chatID,
			PhotoSize:            "big",
			TelegramFileID:       "file_big",
			TelegramFileUniqueID: "unique_big",
			MinIOObjectKey:       "profile_photos/cd/cde456_big",
			FileHash:             "cde456_big",
			Width:                sql.NullInt32{Int32: 640, Valid: true},
			Height:               sql.NullInt32{Int32: 640, Valid: true},
			FileSize:             sql.NullInt64{Int64: 55000, Valid: true},
		},
		{
			ChatID:               chatID,
			PhotoSize:            "small",
			TelegramFileID:       "file_small",
			TelegramFileUniqueID: "unique_small",
			MinIOObjectKey:       "profile_photos/cd/cde456_small",
			FileHash:             "cde456_small",
			Width:                sql.NullInt32{Int32: 160, Valid: true},
			Height:               sql.NullInt32{Int32: 160, Valid: true},
			FileSize:             sql.NullInt64{Int64: 12000, Valid: true},
		},
	}

	// Replace (insert new) chat photos
	err = repo.ReplaceChatPhotos(ctx, tx, chatID, photos)
	if err != nil {
		t.Fatalf("ReplaceChatPhotos failed: %v", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}

	// Verify photos were inserted
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM chat_profile_photos WHERE chat_id = $1", chatID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count chat photos: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 chat photos, got %d", count)
	}
}

// TestReplaceChatPhotosReplace tests replacing existing chat photos
func TestReplaceChatPhotosReplace(t *testing.T) {
	db := setupProfilePhotoTestDB(t)
	defer teardownProfilePhotoTestDB(t, db)
	defer cleanupProfilePhotoTables(t, db)

	_, chatID := setupProfilePhotoTestData(t, db)
	repo := NewProfilePhotoRepository(db, nil)
	ctx := context.Background()

	// Insert initial photos
	tx1, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	initialPhotos := []models.DBChatProfilePhoto{
		{
			ChatID:               chatID,
			PhotoSize:            "big",
			TelegramFileID:       "old_big_file",
			TelegramFileUniqueID: "old_big_unique",
			MinIOObjectKey:       "profile_photos/old/old_big",
			FileHash:             "old_big_hash",
			Width:                sql.NullInt32{Int32: 640, Valid: true},
			Height:               sql.NullInt32{Int32: 640, Valid: true},
			FileSize:             sql.NullInt64{Int64: 50000, Valid: true},
		},
	}
	err = repo.ReplaceChatPhotos(ctx, tx1, chatID, initialPhotos)
	if err != nil {
		t.Fatalf("initial ReplaceChatPhotos failed: %v", err)
	}
	tx1.Commit()

	// Replace with new photos
	tx2, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx2.Rollback()

	newPhotos := []models.DBChatProfilePhoto{
		{
			ChatID:               chatID,
			PhotoSize:            "big",
			TelegramFileID:       "new_big_file",
			TelegramFileUniqueID: "new_big_unique",
			MinIOObjectKey:       "profile_photos/new/new_big",
			FileHash:             "new_big_hash",
			Width:                sql.NullInt32{Int32: 800, Valid: true},
			Height:               sql.NullInt32{Int32: 800, Valid: true},
			FileSize:             sql.NullInt64{Int64: 75000, Valid: true},
		},
		{
			ChatID:               chatID,
			PhotoSize:            "small",
			TelegramFileID:       "new_small_file",
			TelegramFileUniqueID: "new_small_unique",
			MinIOObjectKey:       "profile_photos/new/new_small",
			FileHash:             "new_small_hash",
			Width:                sql.NullInt32{Int32: 200, Valid: true},
			Height:               sql.NullInt32{Int32: 200, Valid: true},
			FileSize:             sql.NullInt64{Int64: 15000, Valid: true},
		},
	}
	err = repo.ReplaceChatPhotos(ctx, tx2, chatID, newPhotos)
	if err != nil {
		t.Fatalf("replacement ReplaceChatPhotos failed: %v", err)
	}
	tx2.Commit()

	// Verify old photo was deleted and new photos were inserted
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM chat_profile_photos WHERE chat_id = $1", chatID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count chat photos: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 chat photos after replacement, got %d", count)
	}

	// Verify old photo is gone
	var oldCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM chat_profile_photos WHERE chat_id = $1 AND file_hash = 'old_big_hash'", chatID).Scan(&oldCount)
	if err != nil {
		t.Fatalf("failed to count old photos: %v", err)
	}
	if oldCount != 0 {
		t.Errorf("expected 0 old photos, got %d", oldCount)
	}
}

// TestGetUserPhotosSingleSize tests retrieving user photos (single photo)
func TestGetUserPhotosSingleSize(t *testing.T) {
	db := setupProfilePhotoTestDB(t)
	defer teardownProfilePhotoTestDB(t, db)
	defer cleanupProfilePhotoTables(t, db)

	userID, _ := setupProfilePhotoTestData(t, db)
	repo := NewProfilePhotoRepository(db, nil)
	ctx := context.Background()

	// Insert a single photo
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	photos := []models.DBUserProfilePhoto{sampleUserPhoto(userID)}
	err = repo.ReplaceUserPhotos(ctx, tx, userID, photos)
	if err != nil {
		t.Fatalf("ReplaceUserPhotos failed: %v", err)
	}
	tx.Commit()

	// Retrieve photos
	retrievedPhotos, err := repo.GetUserPhotos(ctx, userID)
	if err != nil {
		t.Fatalf("GetUserPhotos failed: %v", err)
	}

	if len(retrievedPhotos) != 1 {
		t.Errorf("expected 1 photo, got %d", len(retrievedPhotos))
	}

	// Verify photo data
	photo := retrievedPhotos[0]
	if photo.UserID != userID {
		t.Errorf("expected user_id %d, got %d", userID, photo.UserID)
	}
	if photo.FileHash != "abc123def456" {
		t.Errorf("expected file_hash 'abc123def456', got '%s'", photo.FileHash)
	}
	if photo.Width != 640 {
		t.Errorf("expected width 640, got %d", photo.Width)
	}
}

// TestGetUserPhotosMultipleSizes tests retrieving user photos (multiple sizes ordered by width DESC)
func TestGetUserPhotosMultipleSizes(t *testing.T) {
	db := setupProfilePhotoTestDB(t)
	defer teardownProfilePhotoTestDB(t, db)
	defer cleanupProfilePhotoTables(t, db)

	userID, _ := setupProfilePhotoTestData(t, db)
	repo := NewProfilePhotoRepository(db, nil)
	ctx := context.Background()

	// Insert photos in random order
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	photos := []models.DBUserProfilePhoto{
		{
			UserID:               userID,
			TelegramFileID:       "file_small",
			TelegramFileUniqueID: "unique_small",
			MinIOObjectKey:       "profile_photos/ab/abc123_small",
			FileHash:             "abc123_small",
			Width:                160,
			Height:               160,
			FileSize:             sql.NullInt64{Int64: 10000, Valid: true},
		},
		{
			UserID:               userID,
			TelegramFileID:       "file_large",
			TelegramFileUniqueID: "unique_large",
			MinIOObjectKey:       "profile_photos/ab/abc123_large",
			FileHash:             "abc123_large",
			Width:                1280,
			Height:               1280,
			FileSize:             sql.NullInt64{Int64: 150000, Valid: true},
		},
		{
			UserID:               userID,
			TelegramFileID:       "file_medium",
			TelegramFileUniqueID: "unique_medium",
			MinIOObjectKey:       "profile_photos/ab/abc123_medium",
			FileHash:             "abc123_medium",
			Width:                640,
			Height:               640,
			FileSize:             sql.NullInt64{Int64: 50000, Valid: true},
		},
	}
	err = repo.ReplaceUserPhotos(ctx, tx, userID, photos)
	if err != nil {
		t.Fatalf("ReplaceUserPhotos failed: %v", err)
	}
	tx.Commit()

	// Retrieve photos (should be ordered by width DESC)
	retrievedPhotos, err := repo.GetUserPhotos(ctx, userID)
	if err != nil {
		t.Fatalf("GetUserPhotos failed: %v", err)
	}

	if len(retrievedPhotos) != 3 {
		t.Errorf("expected 3 photos, got %d", len(retrievedPhotos))
	}

	// Verify ordering (width DESC)
	if retrievedPhotos[0].Width != 1280 {
		t.Errorf("expected first photo width 1280, got %d", retrievedPhotos[0].Width)
	}
	if retrievedPhotos[1].Width != 640 {
		t.Errorf("expected second photo width 640, got %d", retrievedPhotos[1].Width)
	}
	if retrievedPhotos[2].Width != 160 {
		t.Errorf("expected third photo width 160, got %d", retrievedPhotos[2].Width)
	}
}

// TestGetUserPhotosNoPhotos tests retrieving user photos when user has no photos
func TestGetUserPhotosNoPhotos(t *testing.T) {
	db := setupProfilePhotoTestDB(t)
	defer teardownProfilePhotoTestDB(t, db)
	defer cleanupProfilePhotoTables(t, db)

	userID, _ := setupProfilePhotoTestData(t, db)
	repo := NewProfilePhotoRepository(db, nil)
	ctx := context.Background()

	// Retrieve photos (should be empty)
	photos, err := repo.GetUserPhotos(ctx, userID)
	if err != nil {
		t.Fatalf("GetUserPhotos failed: %v", err)
	}

	if len(photos) != 0 {
		t.Errorf("expected 0 photos, got %d", len(photos))
	}
}

// TestGetChatPhotosBySize tests retrieving chat photos (ordered by photo_size)
func TestGetChatPhotosBySize(t *testing.T) {
	db := setupProfilePhotoTestDB(t)
	defer teardownProfilePhotoTestDB(t, db)
	defer cleanupProfilePhotoTables(t, db)

	_, chatID := setupProfilePhotoTestData(t, db)
	repo := NewProfilePhotoRepository(db, nil)
	ctx := context.Background()

	// Insert photos in random order
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	photos := []models.DBChatProfilePhoto{
		{
			ChatID:               chatID,
			PhotoSize:            "small",
			TelegramFileID:       "file_small",
			TelegramFileUniqueID: "unique_small",
			MinIOObjectKey:       "profile_photos/cd/cde456_small",
			FileHash:             "cde456_small",
			Width:                sql.NullInt32{Int32: 160, Valid: true},
			Height:               sql.NullInt32{Int32: 160, Valid: true},
			FileSize:             sql.NullInt64{Int64: 12000, Valid: true},
		},
		{
			ChatID:               chatID,
			PhotoSize:            "big",
			TelegramFileID:       "file_big",
			TelegramFileUniqueID: "unique_big",
			MinIOObjectKey:       "profile_photos/cd/cde456_big",
			FileHash:             "cde456_big",
			Width:                sql.NullInt32{Int32: 640, Valid: true},
			Height:               sql.NullInt32{Int32: 640, Valid: true},
			FileSize:             sql.NullInt64{Int64: 55000, Valid: true},
		},
	}
	err = repo.ReplaceChatPhotos(ctx, tx, chatID, photos)
	if err != nil {
		t.Fatalf("ReplaceChatPhotos failed: %v", err)
	}
	tx.Commit()

	// Retrieve photos (should be ordered by photo_size: "big" < "small" alphabetically)
	retrievedPhotos, err := repo.GetChatPhotos(ctx, chatID)
	if err != nil {
		t.Fatalf("GetChatPhotos failed: %v", err)
	}

	if len(retrievedPhotos) != 2 {
		t.Errorf("expected 2 photos, got %d", len(retrievedPhotos))
	}

	// Verify ordering (photo_size alphabetical: "big" < "small")
	if retrievedPhotos[0].PhotoSize != "big" {
		t.Errorf("expected first photo size 'big', got '%s'", retrievedPhotos[0].PhotoSize)
	}
	if retrievedPhotos[1].PhotoSize != "small" {
		t.Errorf("expected second photo size 'small', got '%s'", retrievedPhotos[1].PhotoSize)
	}
}

// TestGetChatPhotosNoPhotos tests retrieving chat photos when chat has no photos
func TestGetChatPhotosNoPhotos(t *testing.T) {
	db := setupProfilePhotoTestDB(t)
	defer teardownProfilePhotoTestDB(t, db)
	defer cleanupProfilePhotoTables(t, db)

	_, chatID := setupProfilePhotoTestData(t, db)
	repo := NewProfilePhotoRepository(db, nil)
	ctx := context.Background()

	// Retrieve photos (should be empty)
	photos, err := repo.GetChatPhotos(ctx, chatID)
	if err != nil {
		t.Fatalf("GetChatPhotos failed: %v", err)
	}

	if len(photos) != 0 {
		t.Errorf("expected 0 photos, got %d", len(photos))
	}
}

// TestGetUserPhotoBySize tests GetUserPhotoBySize method (large/medium/small selection)
func TestGetUserPhotoBySize(t *testing.T) {
	db := setupProfilePhotoTestDB(t)
	defer teardownProfilePhotoTestDB(t, db)
	defer cleanupProfilePhotoTables(t, db)

	userID, _ := setupProfilePhotoTestData(t, db)
	repo := NewProfilePhotoRepository(db, nil)
	ctx := context.Background()

	// Insert 3 photos
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	photos := []models.DBUserProfilePhoto{
		{
			UserID:               userID,
			TelegramFileID:       "file_large",
			TelegramFileUniqueID: "unique_large",
			MinIOObjectKey:       "profile_photos/ab/abc123_large",
			FileHash:             "abc123_large",
			Width:                1280,
			Height:               1280,
			FileSize:             sql.NullInt64{Int64: 150000, Valid: true},
		},
		{
			UserID:               userID,
			TelegramFileID:       "file_medium",
			TelegramFileUniqueID: "unique_medium",
			MinIOObjectKey:       "profile_photos/ab/abc123_medium",
			FileHash:             "abc123_medium",
			Width:                640,
			Height:               640,
			FileSize:             sql.NullInt64{Int64: 50000, Valid: true},
		},
		{
			UserID:               userID,
			TelegramFileID:       "file_small",
			TelegramFileUniqueID: "unique_small",
			MinIOObjectKey:       "profile_photos/ab/abc123_small",
			FileHash:             "abc123_small",
			Width:                160,
			Height:               160,
			FileSize:             sql.NullInt64{Int64: 10000, Valid: true},
		},
	}
	err = repo.ReplaceUserPhotos(ctx, tx, userID, photos)
	if err != nil {
		t.Fatalf("ReplaceUserPhotos failed: %v", err)
	}
	tx.Commit()

	// Test "large" size (index 0 - largest width)
	largePhoto, err := repo.GetUserPhotoBySize(ctx, userID, "large")
	if err != nil {
		t.Fatalf("GetUserPhotoBySize(large) failed: %v", err)
	}
	if largePhoto == nil {
		t.Fatal("expected large photo, got nil")
	}
	if largePhoto.Width != 1280 {
		t.Errorf("expected large photo width 1280, got %d", largePhoto.Width)
	}

	// Test "medium" size (index len/2 = 1)
	mediumPhoto, err := repo.GetUserPhotoBySize(ctx, userID, "medium")
	if err != nil {
		t.Fatalf("GetUserPhotoBySize(medium) failed: %v", err)
	}
	if mediumPhoto == nil {
		t.Fatal("expected medium photo, got nil")
	}
	if mediumPhoto.Width != 640 {
		t.Errorf("expected medium photo width 640, got %d", mediumPhoto.Width)
	}

	// Test "small" size (index len-1 = 2 - smallest width)
	smallPhoto, err := repo.GetUserPhotoBySize(ctx, userID, "small")
	if err != nil {
		t.Fatalf("GetUserPhotoBySize(small) failed: %v", err)
	}
	if smallPhoto == nil {
		t.Fatal("expected small photo, got nil")
	}
	if smallPhoto.Width != 160 {
		t.Errorf("expected small photo width 160, got %d", smallPhoto.Width)
	}

	// Test default size (should return large)
	defaultPhoto, err := repo.GetUserPhotoBySize(ctx, userID, "unknown")
	if err != nil {
		t.Fatalf("GetUserPhotoBySize(unknown) failed: %v", err)
	}
	if defaultPhoto == nil {
		t.Fatal("expected default photo, got nil")
	}
	if defaultPhoto.Width != 1280 {
		t.Errorf("expected default photo width 1280, got %d", defaultPhoto.Width)
	}
}

// TestGetUserPhotoBySize_NoPhotos tests GetUserPhotoBySize when user has no photos
func TestGetUserPhotoBySize_NoPhotos(t *testing.T) {
	db := setupProfilePhotoTestDB(t)
	defer teardownProfilePhotoTestDB(t, db)
	defer cleanupProfilePhotoTables(t, db)

	userID, _ := setupProfilePhotoTestData(t, db)
	repo := NewProfilePhotoRepository(db, nil)
	ctx := context.Background()

	// Get photo by size (should return nil)
	photo, err := repo.GetUserPhotoBySize(ctx, userID, "large")
	if err != nil {
		t.Fatalf("GetUserPhotoBySize failed: %v", err)
	}
	if photo != nil {
		t.Errorf("expected nil photo for user with no photos, got %+v", photo)
	}
}

// TestGetChatPhotoBySize tests GetChatPhotoBySize method (big/small selection with fallback)
func TestGetChatPhotoBySize(t *testing.T) {
	db := setupProfilePhotoTestDB(t)
	defer teardownProfilePhotoTestDB(t, db)
	defer cleanupProfilePhotoTables(t, db)

	_, chatID := setupProfilePhotoTestData(t, db)
	repo := NewProfilePhotoRepository(db, nil)
	ctx := context.Background()

	// Insert big and small photos
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	photos := []models.DBChatProfilePhoto{
		{
			ChatID:               chatID,
			PhotoSize:            "big",
			TelegramFileID:       "file_big",
			TelegramFileUniqueID: "unique_big",
			MinIOObjectKey:       "profile_photos/cd/cde456_big",
			FileHash:             "cde456_big",
			Width:                sql.NullInt32{Int32: 640, Valid: true},
			Height:               sql.NullInt32{Int32: 640, Valid: true},
			FileSize:             sql.NullInt64{Int64: 55000, Valid: true},
		},
		{
			ChatID:               chatID,
			PhotoSize:            "small",
			TelegramFileID:       "file_small",
			TelegramFileUniqueID: "unique_small",
			MinIOObjectKey:       "profile_photos/cd/cde456_small",
			FileHash:             "cde456_small",
			Width:                sql.NullInt32{Int32: 160, Valid: true},
			Height:               sql.NullInt32{Int32: 160, Valid: true},
			FileSize:             sql.NullInt64{Int64: 12000, Valid: true},
		},
	}
	err = repo.ReplaceChatPhotos(ctx, tx, chatID, photos)
	if err != nil {
		t.Fatalf("ReplaceChatPhotos failed: %v", err)
	}
	tx.Commit()

	// Test "large" size (maps to "big")
	largePhoto, err := repo.GetChatPhotoBySize(ctx, chatID, "large")
	if err != nil {
		t.Fatalf("GetChatPhotoBySize(large) failed: %v", err)
	}
	if largePhoto == nil {
		t.Fatal("expected large photo, got nil")
	}
	if largePhoto.PhotoSize != "big" {
		t.Errorf("expected photo_size 'big', got '%s'", largePhoto.PhotoSize)
	}

	// Test "small" size
	smallPhoto, err := repo.GetChatPhotoBySize(ctx, chatID, "small")
	if err != nil {
		t.Fatalf("GetChatPhotoBySize(small) failed: %v", err)
	}
	if smallPhoto == nil {
		t.Fatal("expected small photo, got nil")
	}
	if smallPhoto.PhotoSize != "small" {
		t.Errorf("expected photo_size 'small', got '%s'", smallPhoto.PhotoSize)
	}
}

// TestGetChatPhotoBySize_FallbackToBig tests fallback when small size is requested but only big exists
func TestGetChatPhotoBySize_FallbackToBig(t *testing.T) {
	db := setupProfilePhotoTestDB(t)
	defer teardownProfilePhotoTestDB(t, db)
	defer cleanupProfilePhotoTables(t, db)

	_, chatID := setupProfilePhotoTestData(t, db)
	repo := NewProfilePhotoRepository(db, nil)
	ctx := context.Background()

	// Insert only "big" photo
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	photos := []models.DBChatProfilePhoto{
		{
			ChatID:               chatID,
			PhotoSize:            "big",
			TelegramFileID:       "file_big",
			TelegramFileUniqueID: "unique_big",
			MinIOObjectKey:       "profile_photos/cd/cde456_big",
			FileHash:             "cde456_big",
			Width:                sql.NullInt32{Int32: 640, Valid: true},
			Height:               sql.NullInt32{Int32: 640, Valid: true},
			FileSize:             sql.NullInt64{Int64: 55000, Valid: true},
		},
	}
	err = repo.ReplaceChatPhotos(ctx, tx, chatID, photos)
	if err != nil {
		t.Fatalf("ReplaceChatPhotos failed: %v", err)
	}
	tx.Commit()

	// Request "small" size (should fallback to "big")
	photo, err := repo.GetChatPhotoBySize(ctx, chatID, "small")
	if err != nil {
		t.Fatalf("GetChatPhotoBySize(small) failed: %v", err)
	}
	if photo == nil {
		t.Fatal("expected fallback photo, got nil")
	}
	if photo.PhotoSize != "big" {
		t.Errorf("expected fallback to 'big', got '%s'", photo.PhotoSize)
	}
}

// TestGetChatPhotoBySize_FallbackToSmall tests fallback when big size is requested but only small exists
func TestGetChatPhotoBySize_FallbackToSmall(t *testing.T) {
	db := setupProfilePhotoTestDB(t)
	defer teardownProfilePhotoTestDB(t, db)
	defer cleanupProfilePhotoTables(t, db)

	_, chatID := setupProfilePhotoTestData(t, db)
	repo := NewProfilePhotoRepository(db, nil)
	ctx := context.Background()

	// Insert only "small" photo
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	photos := []models.DBChatProfilePhoto{
		{
			ChatID:               chatID,
			PhotoSize:            "small",
			TelegramFileID:       "file_small",
			TelegramFileUniqueID: "unique_small",
			MinIOObjectKey:       "profile_photos/cd/cde456_small",
			FileHash:             "cde456_small",
			Width:                sql.NullInt32{Int32: 160, Valid: true},
			Height:               sql.NullInt32{Int32: 160, Valid: true},
			FileSize:             sql.NullInt64{Int64: 12000, Valid: true},
		},
	}
	err = repo.ReplaceChatPhotos(ctx, tx, chatID, photos)
	if err != nil {
		t.Fatalf("ReplaceChatPhotos failed: %v", err)
	}
	tx.Commit()

	// Request "large" size (maps to "big", should fallback to "small")
	photo, err := repo.GetChatPhotoBySize(ctx, chatID, "large")
	if err != nil {
		t.Fatalf("GetChatPhotoBySize(large) failed: %v", err)
	}
	if photo == nil {
		t.Fatal("expected fallback photo, got nil")
	}
	if photo.PhotoSize != "small" {
		t.Errorf("expected fallback to 'small', got '%s'", photo.PhotoSize)
	}
}

// TestGetChatPhotoBySize_NoPhotos tests GetChatPhotoBySize when chat has no photos
func TestGetChatPhotoBySize_NoPhotos(t *testing.T) {
	db := setupProfilePhotoTestDB(t)
	defer teardownProfilePhotoTestDB(t, db)
	defer cleanupProfilePhotoTables(t, db)

	_, chatID := setupProfilePhotoTestData(t, db)
	repo := NewProfilePhotoRepository(db, nil)
	ctx := context.Background()

	// Get photo by size (should return nil)
	photo, err := repo.GetChatPhotoBySize(ctx, chatID, "large")
	if err != nil {
		t.Fatalf("GetChatPhotoBySize failed: %v", err)
	}
	if photo != nil {
		t.Errorf("expected nil photo for chat with no photos, got %+v", photo)
	}
}

// TestGetAllUserIDs tests GetAllUserIDs method
func TestGetAllUserIDs(t *testing.T) {
	db := setupProfilePhotoTestDB(t)
	defer teardownProfilePhotoTestDB(t, db)
	defer cleanupProfilePhotoTables(t, db)

	repo := NewProfilePhotoRepository(db, nil)
	ctx := context.Background()

	// Insert multiple users
	_, err := db.ExecContext(ctx, `
		INSERT INTO users (id, is_bot, first_name) VALUES
		(111, false, 'User 1'),
		(222, false, 'User 2'),
		(333, false, 'User 3')
	`)
	if err != nil {
		t.Fatalf("failed to insert test users: %v", err)
	}

	// Get all user IDs
	ids, err := repo.GetAllUserIDs(ctx)
	if err != nil {
		t.Fatalf("GetAllUserIDs failed: %v", err)
	}

	// Verify count
	if len(ids) != 3 {
		t.Errorf("expected 3 user IDs, got %d", len(ids))
	}

	// Verify ordering (should be sorted by ID)
	if ids[0] != 111 || ids[1] != 222 || ids[2] != 333 {
		t.Errorf("expected user IDs [111, 222, 333], got %v", ids)
	}
}

// TestGetAllChatIDs tests GetAllChatIDs method
func TestGetAllChatIDs(t *testing.T) {
	db := setupProfilePhotoTestDB(t)
	defer teardownProfilePhotoTestDB(t, db)
	defer cleanupProfilePhotoTables(t, db)

	repo := NewProfilePhotoRepository(db, nil)
	ctx := context.Background()

	// Insert multiple chats
	_, err := db.ExecContext(ctx, `
		INSERT INTO chats (id, type, title) VALUES
		(-1001, 'supergroup', 'Chat 1'),
		(-1002, 'supergroup', 'Chat 2'),
		(-1003, 'group', 'Chat 3')
	`)
	if err != nil {
		t.Fatalf("failed to insert test chats: %v", err)
	}

	// Get all chat IDs
	ids, err := repo.GetAllChatIDs(ctx)
	if err != nil {
		t.Fatalf("GetAllChatIDs failed: %v", err)
	}

	// Verify count
	if len(ids) != 3 {
		t.Errorf("expected 3 chat IDs, got %d", len(ids))
	}

	// Verify ordering (should be sorted by ID)
	if ids[0] != -1003 || ids[1] != -1002 || ids[2] != -1001 {
		t.Errorf("expected chat IDs [-1003, -1002, -1001], got %v", ids)
	}
}

// TestProfilePhotoHashDeduplication tests hash-based deduplication across users and chats
func TestProfilePhotoHashDeduplication(t *testing.T) {
	db := setupProfilePhotoTestDB(t)
	defer teardownProfilePhotoTestDB(t, db)
	defer cleanupProfilePhotoTables(t, db)

	ctx := context.Background()

	// Insert two users
	_, err := db.ExecContext(ctx, `
		INSERT INTO users (id, is_bot, first_name) VALUES
		(111, false, 'User 1'),
		(222, false, 'User 2')
	`)
	if err != nil {
		t.Fatalf("failed to insert test users: %v", err)
	}

	repo := NewProfilePhotoRepository(db, nil)

	// Use same hash for both users (content-addressable storage)
	sharedHash := "shared_hash_abc123"
	sharedObjectKey := "profile_photos/sh/shared_hash_abc123"

	// Insert photo for user 1
	tx1, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	user1Photos := []models.DBUserProfilePhoto{
		{
			UserID:               111,
			TelegramFileID:       "file_user1",
			TelegramFileUniqueID: "unique_user1",
			MinIOObjectKey:       sharedObjectKey,
			FileHash:             sharedHash,
			Width:                640,
			Height:               640,
			FileSize:             sql.NullInt64{Int64: 50000, Valid: true},
		},
	}
	err = repo.ReplaceUserPhotos(ctx, tx1, 111, user1Photos)
	if err != nil {
		t.Fatalf("ReplaceUserPhotos user1 failed: %v", err)
	}
	tx1.Commit()

	// Insert photo for user 2 (same hash, same object key)
	tx2, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	user2Photos := []models.DBUserProfilePhoto{
		{
			UserID:               222,
			TelegramFileID:       "file_user2",
			TelegramFileUniqueID: "unique_user2",
			MinIOObjectKey:       sharedObjectKey, // Same object key
			FileHash:             sharedHash,       // Same hash
			Width:                640,
			Height:               640,
			FileSize:             sql.NullInt64{Int64: 50000, Valid: true},
		},
	}
	err = repo.ReplaceUserPhotos(ctx, tx2, 222, user2Photos)
	if err != nil {
		t.Fatalf("ReplaceUserPhotos user2 failed: %v", err)
	}
	tx2.Commit()

	// Verify both users have photos with same hash
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_profile_photos WHERE file_hash = $1", sharedHash).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count photos with shared hash: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 photos with shared hash, got %d", count)
	}

	// Verify both users have photos with same object key
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_profile_photos WHERE minio_object_key = $1", sharedObjectKey).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count photos with shared object key: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 photos with shared object key, got %d", count)
	}
}
