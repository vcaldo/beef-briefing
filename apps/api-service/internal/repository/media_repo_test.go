package repository

import (
	"context"
	"database/sql"
	"testing"

	"beef-briefing/apps/api-service/internal/models"
)

// setupMediaTestDB creates a new database connection for testing
func setupMediaTestDB(t *testing.T) *sql.DB {
	t.Helper()
	return setupUserTestDB(t) // Reuse existing setup
}

// teardownMediaTestDB closes the test database connection
func teardownMediaTestDB(t *testing.T, db *sql.DB) {
	t.Helper()
	teardownUserTestDB(t, db) // Reuse existing teardown
}

// cleanupMediaTables truncates all media-related tables for test isolation
func cleanupMediaTables(t *testing.T, db *sql.DB) {
	t.Helper()

	ctx := context.Background()
	// Truncate in reverse dependency order (child tables first)
	tables := []string{
		"message_entities",
		"message_edits",
		"stickers",
		"poll_options",
		"polls",
		"game_photos",
		"games",
		"photos",
		"venues",
		"locations",
		"contacts",
		"dice",
		"media_files",
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

// setupMediaTestData inserts required users, chats, and messages for media testing
// Returns the database message ID (auto-incremented primary key)
func setupMediaTestData(t *testing.T, ctx context.Context, tx *sql.Tx, chatID, userID, messageID int64) int64 {
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

	// Insert message
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
		Text: "Test message",
	}
	dbMessageID, err := msgRepo.InsertMessage(ctx, tx, msg)
	if err != nil {
		t.Fatalf("failed to setup message: %v", err)
	}

	return dbMessageID
}

// TestGetObjectKeyByHashMediaFiles tests hash lookup in media_files table
func TestGetObjectKeyByHashMediaFiles(t *testing.T) {
	db := setupMediaTestDB(t)
	defer teardownMediaTestDB(t, db)
	defer cleanupMediaTables(t, db)

	ctx := context.Background()
	cleanupMediaTables(t, db)

	repo := NewMediaRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Setup test data
	chatID := int64(-1003280306634)
	userID := int64(123456789)
	telegramMessageID := int64(1)
	dbMessageID := setupMediaTestData(t, ctx, tx, chatID, userID, telegramMessageID)

	// Insert a media file with a known hash
	fileHash := "abc123def456"
	objectKey := "video/ab/abc123def456"
	fileSize := int64(1024000)

	err = repo.InsertMediaFile(ctx, tx, dbMessageID, "video", "file_id_123", "file_unique_id_123",
		objectKey, fileHash, &fileSize, "video/mp4", "test_video.mp4", nil, nil, nil, "", "")
	if err != nil {
		t.Fatalf("failed to insert media file: %v", err)
	}

	// Query by hash
	foundKey, err := repo.GetObjectKeyByHash(ctx, tx, fileHash)
	if err != nil {
		t.Fatalf("failed to get object key by hash: %v", err)
	}

	if foundKey != objectKey {
		t.Errorf("expected object key %q, got %q", objectKey, foundKey)
	}
}

// TestGetObjectKeyByHashPhotos tests hash lookup in photos table
func TestGetObjectKeyByHashPhotos(t *testing.T) {
	db := setupMediaTestDB(t)
	defer teardownMediaTestDB(t, db)
	defer cleanupMediaTables(t, db)

	ctx := context.Background()
	cleanupMediaTables(t, db)

	repo := NewMediaRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Setup test data
	chatID := int64(-1003280306634)
	userID := int64(123456789)
	telegramMessageID := int64(1)
	dbMessageID := setupMediaTestData(t, ctx, tx, chatID, userID, telegramMessageID)

	// Insert a photo with a known hash
	fileHash := "photo_hash_123"
	objectKey := "photo/ph/photo_hash_123"
	fileSize := int64(512000)

	photo := &models.PhotoSize{
		FileID:       "photo_file_id_123",
		FileUniqueID: "photo_file_unique_id_123",
		Width:        1920,
		Height:       1080,
		FileSize:     &fileSize,
	}

	err = repo.InsertPhoto(ctx, tx, dbMessageID, photo, objectKey, fileHash)
	if err != nil {
		t.Fatalf("failed to insert photo: %v", err)
	}

	// Query by hash
	foundKey, err := repo.GetObjectKeyByHash(ctx, tx, fileHash)
	if err != nil {
		t.Fatalf("failed to get object key by hash: %v", err)
	}

	if foundKey != objectKey {
		t.Errorf("expected object key %q, got %q", objectKey, foundKey)
	}
}

// TestGetObjectKeyByHashGamePhotos tests hash lookup in game_photos table
func TestGetObjectKeyByHashGamePhotos(t *testing.T) {
	db := setupMediaTestDB(t)
	defer teardownMediaTestDB(t, db)
	defer cleanupMediaTables(t, db)

	ctx := context.Background()
	cleanupMediaTables(t, db)

	repo := NewMediaRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Setup test data
	chatID := int64(-1003280306634)
	userID := int64(123456789)
	telegramMessageID := int64(1)
	dbMessageID := setupMediaTestData(t, ctx, tx, chatID, userID, telegramMessageID)

	// Insert a game first
	game := &models.Game{
		Title:       "Test Game",
		Description: "A test game",
		Text:        "Game text",
	}

	gameID, err := repo.InsertGame(ctx, tx, dbMessageID, game)
	if err != nil {
		t.Fatalf("failed to insert game: %v", err)
	}

	// Insert a game photo with a known hash
	fileHash := "game_photo_hash_123"
	objectKey := "game_photo/ga/game_photo_hash_123"
	fileSize := int64(256000)

	gamePhoto := &models.PhotoSize{
		FileID:       "game_photo_file_id_123",
		FileUniqueID: "game_photo_file_unique_id_123",
		Width:        800,
		Height:       600,
		FileSize:     &fileSize,
	}

	err = repo.InsertGamePhoto(ctx, tx, gameID, gamePhoto, objectKey, fileHash)
	if err != nil {
		t.Fatalf("failed to insert game photo: %v", err)
	}

	// Query by hash
	foundKey, err := repo.GetObjectKeyByHash(ctx, tx, fileHash)
	if err != nil {
		t.Fatalf("failed to get object key by hash: %v", err)
	}

	if foundKey != objectKey {
		t.Errorf("expected object key %q, got %q", objectKey, foundKey)
	}
}

// TestGetObjectKeyByHashNotFound tests hash lookup when file does not exist
func TestGetObjectKeyByHashNotFound(t *testing.T) {
	db := setupMediaTestDB(t)
	defer teardownMediaTestDB(t, db)
	defer cleanupMediaTables(t, db)

	ctx := context.Background()
	cleanupMediaTables(t, db)

	repo := NewMediaRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Query for a non-existent hash
	foundKey, err := repo.GetObjectKeyByHash(ctx, tx, "nonexistent_hash")
	if err != nil {
		t.Fatalf("expected no error for non-existent hash, got: %v", err)
	}

	if foundKey != "" {
		t.Errorf("expected empty object key for non-existent hash, got %q", foundKey)
	}
}

// TestGetObjectKeyByHashUNIONCorrectness tests that UNION query returns the first match
func TestGetObjectKeyByHashUNIONCorrectness(t *testing.T) {
	db := setupMediaTestDB(t)
	defer teardownMediaTestDB(t, db)
	defer cleanupMediaTables(t, db)

	ctx := context.Background()
	cleanupMediaTables(t, db)

	repo := NewMediaRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Setup test data
	chatID := int64(-1003280306634)
	userID := int64(123456789)
	telegramMessageID := int64(1)
	dbMessageID := setupMediaTestData(t, ctx, tx, chatID, userID, telegramMessageID)

	// Insert the same hash in multiple tables
	sharedHash := "shared_hash_123"
	mediaFileKey := "video/sh/shared_hash_123"
	fileSize := int64(1024000)

	// Insert into media_files
	err = repo.InsertMediaFile(ctx, tx, dbMessageID, "video", "file_id_1", "file_unique_id_1",
		mediaFileKey, sharedHash, &fileSize, "video/mp4", "video.mp4", nil, nil, nil, "", "")
	if err != nil {
		t.Fatalf("failed to insert media file: %v", err)
	}

	// Insert into photos with the same hash but different object key
	photoKey := "photo/sh/shared_hash_123"
	photo := &models.PhotoSize{
		FileID:       "photo_file_id_1",
		FileUniqueID: "photo_file_unique_id_1",
		Width:        1920,
		Height:       1080,
		FileSize:     &fileSize,
	}

	err = repo.InsertPhoto(ctx, tx, dbMessageID, photo, photoKey, sharedHash)
	if err != nil {
		t.Fatalf("failed to insert photo: %v", err)
	}

	// Query by hash - should return one of the keys (UNION LIMIT 1)
	foundKey, err := repo.GetObjectKeyByHash(ctx, tx, sharedHash)
	if err != nil {
		t.Fatalf("failed to get object key by hash: %v", err)
	}

	// Verify we got one of the valid keys
	if foundKey != mediaFileKey && foundKey != photoKey {
		t.Errorf("expected object key %q or %q, got %q", mediaFileKey, photoKey, foundKey)
	}

	// Most importantly, verify we got a non-empty result
	if foundKey == "" {
		t.Errorf("expected non-empty object key, got empty string")
	}
}

// TestInsertMediaFileVideo tests inserting a video media file
func TestInsertMediaFileVideo(t *testing.T) {
	db := setupMediaTestDB(t)
	defer teardownMediaTestDB(t, db)
	defer cleanupMediaTables(t, db)

	ctx := context.Background()
	cleanupMediaTables(t, db)

	repo := NewMediaRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Setup test data
	chatID := int64(-1003280306634)
	userID := int64(123456789)
	telegramMessageID := int64(1)
	dbMessageID := setupMediaTestData(t, ctx, tx, chatID, userID, telegramMessageID)

	// Insert video media file
	fileHash := "video_hash_123"
	objectKey := "video/vi/video_hash_123"
	fileSize := int64(2048000)
	duration := 120
	width := 1920
	height := 1080

	err = repo.InsertMediaFile(ctx, tx, dbMessageID, "video", "video_file_id_123", "video_file_unique_id_123",
		objectKey, fileHash, &fileSize, "video/mp4", "sample_video.mp4", &duration, &width, &height, "", "")
	if err != nil {
		t.Fatalf("failed to insert video media file: %v", err)
	}

	// Verify insertion
	var storedHash, storedKey string
	var storedSize int64
	err = tx.QueryRowContext(ctx,
		"SELECT file_hash, minio_object_key, file_size FROM media_files WHERE message_id = $1",
		dbMessageID,
	).Scan(&storedHash, &storedKey, &storedSize)
	if err != nil {
		t.Fatalf("failed to query media file: %v", err)
	}

	if storedHash != fileHash {
		t.Errorf("expected file hash %q, got %q", fileHash, storedHash)
	}
	if storedKey != objectKey {
		t.Errorf("expected object key %q, got %q", objectKey, storedKey)
	}
	if storedSize != fileSize {
		t.Errorf("expected file size %d, got %d", fileSize, storedSize)
	}
}

// TestInsertMediaFileAudio tests inserting an audio media file
func TestInsertMediaFileAudio(t *testing.T) {
	db := setupMediaTestDB(t)
	defer teardownMediaTestDB(t, db)
	defer cleanupMediaTables(t, db)

	ctx := context.Background()
	cleanupMediaTables(t, db)

	repo := NewMediaRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Setup test data
	chatID := int64(-1003280306634)
	userID := int64(123456789)
	telegramMessageID := int64(1)
	dbMessageID := setupMediaTestData(t, ctx, tx, chatID, userID, telegramMessageID)

	// Insert audio media file with performer and title
	fileHash := "audio_hash_123"
	objectKey := "audio/au/audio_hash_123"
	fileSize := int64(5120000)
	duration := 240

	err = repo.InsertMediaFile(ctx, tx, dbMessageID, "audio", "audio_file_id_123", "audio_file_unique_id_123",
		objectKey, fileHash, &fileSize, "audio/mpeg", "song.mp3", &duration, nil, nil, "Artist Name", "Song Title")
	if err != nil {
		t.Fatalf("failed to insert audio media file: %v", err)
	}

	// Verify insertion including audio-specific fields
	var storedPerformer, storedTitle string
	err = tx.QueryRowContext(ctx,
		"SELECT performer, title FROM media_files WHERE message_id = $1",
		dbMessageID,
	).Scan(&storedPerformer, &storedTitle)
	if err != nil {
		t.Fatalf("failed to query media file: %v", err)
	}

	if storedPerformer != "Artist Name" {
		t.Errorf("expected performer %q, got %q", "Artist Name", storedPerformer)
	}
	if storedTitle != "Song Title" {
		t.Errorf("expected title %q, got %q", "Song Title", storedTitle)
	}
}

// TestInsertMediaFileDocument tests inserting a document media file
func TestInsertMediaFileDocument(t *testing.T) {
	db := setupMediaTestDB(t)
	defer teardownMediaTestDB(t, db)
	defer cleanupMediaTables(t, db)

	ctx := context.Background()
	cleanupMediaTables(t, db)

	repo := NewMediaRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Setup test data
	chatID := int64(-1003280306634)
	userID := int64(123456789)
	telegramMessageID := int64(1)
	dbMessageID := setupMediaTestData(t, ctx, tx, chatID, userID, telegramMessageID)

	// Insert document media file
	fileHash := "doc_hash_123"
	objectKey := "document/do/doc_hash_123"
	fileSize := int64(10240000)

	err = repo.InsertMediaFile(ctx, tx, dbMessageID, "document", "doc_file_id_123", "doc_file_unique_id_123",
		objectKey, fileHash, &fileSize, "application/pdf", "report.pdf", nil, nil, nil, "", "")
	if err != nil {
		t.Fatalf("failed to insert document media file: %v", err)
	}

	// Verify insertion
	var storedMediaType, storedMimeType, storedFileName string
	err = tx.QueryRowContext(ctx,
		"SELECT media_type, mime_type, file_name FROM media_files WHERE message_id = $1",
		dbMessageID,
	).Scan(&storedMediaType, &storedMimeType, &storedFileName)
	if err != nil {
		t.Fatalf("failed to query media file: %v", err)
	}

	if storedMediaType != "document" {
		t.Errorf("expected media type %q, got %q", "document", storedMediaType)
	}
	if storedMimeType != "application/pdf" {
		t.Errorf("expected mime type %q, got %q", "application/pdf", storedMimeType)
	}
	if storedFileName != "report.pdf" {
		t.Errorf("expected file name %q, got %q", "report.pdf", storedFileName)
	}
}

// TestInsertMediaFileWithNullFields tests inserting media file without download (NULL object key and hash)
func TestInsertMediaFileWithNullFields(t *testing.T) {
	db := setupMediaTestDB(t)
	defer teardownMediaTestDB(t, db)
	defer cleanupMediaTables(t, db)

	ctx := context.Background()
	cleanupMediaTables(t, db)

	repo := NewMediaRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Setup test data
	chatID := int64(-1003280306634)
	userID := int64(123456789)
	telegramMessageID := int64(1)
	dbMessageID := setupMediaTestData(t, ctx, tx, chatID, userID, telegramMessageID)

	// Insert media file without object key and hash (file wasn't downloaded)
	err = repo.InsertMediaFile(ctx, tx, dbMessageID, "photo", "photo_file_id_123", "photo_file_unique_id_123",
		"", "", nil, "", "", nil, nil, nil, "", "")
	if err != nil {
		t.Fatalf("failed to insert media file with NULL fields: %v", err)
	}

	// Verify NULL fields are stored correctly
	var objectKey, fileHash sql.NullString
	err = tx.QueryRowContext(ctx,
		"SELECT minio_object_key, file_hash FROM media_files WHERE message_id = $1",
		dbMessageID,
	).Scan(&objectKey, &fileHash)
	if err != nil {
		t.Fatalf("failed to query media file: %v", err)
	}

	if objectKey.Valid {
		t.Errorf("expected NULL object key, got %q", objectKey.String)
	}
	if fileHash.Valid {
		t.Errorf("expected NULL file hash, got %q", fileHash.String)
	}
}

// TestInsertPhotoSingleSize tests inserting a single photo size
func TestInsertPhotoSingleSize(t *testing.T) {
	db := setupMediaTestDB(t)
	defer teardownMediaTestDB(t, db)
	defer cleanupMediaTables(t, db)

	ctx := context.Background()
	cleanupMediaTables(t, db)

	repo := NewMediaRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Setup test data
	chatID := int64(-1003280306634)
	userID := int64(123456789)
	telegramMessageID := int64(1)
	dbMessageID := setupMediaTestData(t, ctx, tx, chatID, userID, telegramMessageID)

	// Insert a single photo size
	fileHash := "photo_hash_single"
	objectKey := "photo/ph/photo_hash_single"
	fileSize := int64(256000)

	photo := &models.PhotoSize{
		FileID:       "photo_file_id_single",
		FileUniqueID: "photo_file_unique_id_single",
		Width:        800,
		Height:       600,
		FileSize:     &fileSize,
	}

	err = repo.InsertPhoto(ctx, tx, dbMessageID, photo, objectKey, fileHash)
	if err != nil {
		t.Fatalf("failed to insert photo: %v", err)
	}

	// Verify insertion
	var storedWidth, storedHeight int
	var storedSize int64
	err = tx.QueryRowContext(ctx,
		"SELECT width, height, file_size FROM photos WHERE message_id = $1",
		dbMessageID,
	).Scan(&storedWidth, &storedHeight, &storedSize)
	if err != nil {
		t.Fatalf("failed to query photo: %v", err)
	}

	if storedWidth != photo.Width {
		t.Errorf("expected width %d, got %d", photo.Width, storedWidth)
	}
	if storedHeight != photo.Height {
		t.Errorf("expected height %d, got %d", photo.Height, storedHeight)
	}
	if storedSize != fileSize {
		t.Errorf("expected file size %d, got %d", fileSize, storedSize)
	}
}

// TestInsertPhotoMultipleSizes tests inserting multiple photo sizes for the same message
func TestInsertPhotoMultipleSizes(t *testing.T) {
	db := setupMediaTestDB(t)
	defer teardownMediaTestDB(t, db)
	defer cleanupMediaTables(t, db)

	ctx := context.Background()
	cleanupMediaTables(t, db)

	repo := NewMediaRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Setup test data
	chatID := int64(-1003280306634)
	userID := int64(123456789)
	telegramMessageID := int64(1)
	dbMessageID := setupMediaTestData(t, ctx, tx, chatID, userID, telegramMessageID)

	// Insert three photo sizes (small, medium, large)
	photoSizes := []struct {
		photo     *models.PhotoSize
		objectKey string
		fileHash  string
	}{
		{
			photo: &models.PhotoSize{
				FileID:       "photo_small",
				FileUniqueID: "photo_unique_small",
				Width:        320,
				Height:       240,
				FileSize:     ptrInt64(51200),
			},
			objectKey: "photo/sm/small_hash",
			fileHash:  "small_hash",
		},
		{
			photo: &models.PhotoSize{
				FileID:       "photo_medium",
				FileUniqueID: "photo_unique_medium",
				Width:        800,
				Height:       600,
				FileSize:     ptrInt64(204800),
			},
			objectKey: "photo/me/medium_hash",
			fileHash:  "medium_hash",
		},
		{
			photo: &models.PhotoSize{
				FileID:       "photo_large",
				FileUniqueID: "photo_unique_large",
				Width:        1920,
				Height:       1080,
				FileSize:     ptrInt64(512000),
			},
			objectKey: "photo/la/large_hash",
			fileHash:  "large_hash",
		},
	}

	for _, ps := range photoSizes {
		err = repo.InsertPhoto(ctx, tx, dbMessageID, ps.photo, ps.objectKey, ps.fileHash)
		if err != nil {
			t.Fatalf("failed to insert photo size %dx%d: %v", ps.photo.Width, ps.photo.Height, err)
		}
	}

	// Verify all three sizes were inserted
	var count int
	err = tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM photos WHERE message_id = $1",
		dbMessageID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count photos: %v", err)
	}

	if count != 3 {
		t.Errorf("expected 3 photo sizes, got %d", count)
	}
}

// TestInsertStickerRegular tests inserting a regular sticker
func TestInsertStickerRegular(t *testing.T) {
	db := setupMediaTestDB(t)
	defer teardownMediaTestDB(t, db)
	defer cleanupMediaTables(t, db)

	ctx := context.Background()
	cleanupMediaTables(t, db)

	repo := NewMediaRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Setup test data
	chatID := int64(-1003280306634)
	userID := int64(123456789)
	telegramMessageID := int64(1)
	dbMessageID := setupMediaTestData(t, ctx, tx, chatID, userID, telegramMessageID)

	// Insert media file for sticker first
	fileHash := "sticker_hash_123"
	objectKey := "sticker/st/sticker_hash_123"
	fileSize := int64(51200)
	width := 512
	height := 512

	mediaFileID, err := repo.InsertMediaFileReturningID(ctx, tx, dbMessageID, "sticker", "sticker_file_id_123", "sticker_file_unique_id_123",
		objectKey, fileHash, &fileSize, "image/webp", "sticker.webp", nil, &width, &height, "", "")
	if err != nil {
		t.Fatalf("failed to insert media file for sticker: %v", err)
	}

	// Insert sticker metadata
	sticker := &models.Sticker{
		Type:       "regular",
		Emoji:      "😀",
		SetName:    "happy_stickers",
		IsAnimated: false,
		IsVideo:    false,
	}

	err = repo.InsertSticker(ctx, tx, dbMessageID, mediaFileID, sticker)
	if err != nil {
		t.Fatalf("failed to insert sticker: %v", err)
	}

	// Verify sticker insertion
	var storedType, storedEmoji, storedSetName string
	var isAnimated, isVideo bool
	err = tx.QueryRowContext(ctx,
		"SELECT sticker_type, emoji, set_name, is_animated, is_video FROM stickers WHERE message_id = $1",
		dbMessageID,
	).Scan(&storedType, &storedEmoji, &storedSetName, &isAnimated, &isVideo)
	if err != nil {
		t.Fatalf("failed to query sticker: %v", err)
	}

	if storedType != sticker.Type {
		t.Errorf("expected sticker type %q, got %q", sticker.Type, storedType)
	}
	if storedEmoji != sticker.Emoji {
		t.Errorf("expected emoji %q, got %q", sticker.Emoji, storedEmoji)
	}
	if storedSetName != sticker.SetName {
		t.Errorf("expected set name %q, got %q", sticker.SetName, storedSetName)
	}
	if isAnimated != sticker.IsAnimated {
		t.Errorf("expected is_animated %v, got %v", sticker.IsAnimated, isAnimated)
	}
	if isVideo != sticker.IsVideo {
		t.Errorf("expected is_video %v, got %v", sticker.IsVideo, isVideo)
	}
}

// TestInsertStickerAnimated tests inserting an animated sticker
func TestInsertStickerAnimated(t *testing.T) {
	db := setupMediaTestDB(t)
	defer teardownMediaTestDB(t, db)
	defer cleanupMediaTables(t, db)

	ctx := context.Background()
	cleanupMediaTables(t, db)

	repo := NewMediaRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Setup test data
	chatID := int64(-1003280306634)
	userID := int64(123456789)
	telegramMessageID := int64(1)
	dbMessageID := setupMediaTestData(t, ctx, tx, chatID, userID, telegramMessageID)

	// Insert media file for animated sticker
	fileHash := "animated_sticker_hash"
	objectKey := "sticker/an/animated_sticker_hash"
	fileSize := int64(102400)
	width := 512
	height := 512

	mediaFileID, err := repo.InsertMediaFileReturningID(ctx, tx, dbMessageID, "sticker", "animated_file_id", "animated_file_unique_id",
		objectKey, fileHash, &fileSize, "application/x-tgsticker", "animated.tgs", nil, &width, &height, "", "")
	if err != nil {
		t.Fatalf("failed to insert media file for animated sticker: %v", err)
	}

	// Insert animated sticker metadata
	sticker := &models.Sticker{
		Type:       "regular",
		Emoji:      "🎉",
		SetName:    "party_stickers",
		IsAnimated: true,
		IsVideo:    false,
	}

	err = repo.InsertSticker(ctx, tx, dbMessageID, mediaFileID, sticker)
	if err != nil {
		t.Fatalf("failed to insert animated sticker: %v", err)
	}

	// Verify is_animated flag
	var isAnimated bool
	err = tx.QueryRowContext(ctx,
		"SELECT is_animated FROM stickers WHERE message_id = $1",
		dbMessageID,
	).Scan(&isAnimated)
	if err != nil {
		t.Fatalf("failed to query sticker: %v", err)
	}

	if !isAnimated {
		t.Errorf("expected is_animated to be true, got false")
	}
}

// TestInsertGameWithPhotos tests inserting a game with photos
func TestInsertGameWithPhotos(t *testing.T) {
	db := setupMediaTestDB(t)
	defer teardownMediaTestDB(t, db)
	defer cleanupMediaTables(t, db)

	ctx := context.Background()
	cleanupMediaTables(t, db)

	repo := NewMediaRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Setup test data
	chatID := int64(-1003280306634)
	userID := int64(123456789)
	telegramMessageID := int64(1)
	dbMessageID := setupMediaTestData(t, ctx, tx, chatID, userID, telegramMessageID)

	// Insert game
	game := &models.Game{
		Title:       "Test Game",
		Description: "An awesome test game",
		Text:        "Click to play!",
	}

	gameID, err := repo.InsertGame(ctx, tx, dbMessageID, game)
	if err != nil {
		t.Fatalf("failed to insert game: %v", err)
	}

	if gameID == 0 {
		t.Errorf("expected non-zero game ID, got 0")
	}

	// Insert game photo
	fileHash := "game_photo_hash"
	objectKey := "game_photo/ga/game_photo_hash"
	fileSize := int64(256000)

	gamePhoto := &models.PhotoSize{
		FileID:       "game_photo_file_id",
		FileUniqueID: "game_photo_file_unique_id",
		Width:        800,
		Height:       600,
		FileSize:     &fileSize,
	}

	err = repo.InsertGamePhoto(ctx, tx, gameID, gamePhoto, objectKey, fileHash)
	if err != nil {
		t.Fatalf("failed to insert game photo: %v", err)
	}

	// Verify game and photo insertion
	var storedTitle string
	err = tx.QueryRowContext(ctx,
		"SELECT title FROM games WHERE id = $1",
		gameID,
	).Scan(&storedTitle)
	if err != nil {
		t.Fatalf("failed to query game: %v", err)
	}

	if storedTitle != game.Title {
		t.Errorf("expected game title %q, got %q", game.Title, storedTitle)
	}

	// Verify game photo
	var photoCount int
	err = tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM game_photos WHERE game_id = $1",
		gameID,
	).Scan(&photoCount)
	if err != nil {
		t.Fatalf("failed to count game photos: %v", err)
	}

	if photoCount != 1 {
		t.Errorf("expected 1 game photo, got %d", photoCount)
	}
}

// TestInsertGameWithoutPhotos tests inserting a game without photos (NULL)
func TestInsertGameWithoutPhotos(t *testing.T) {
	db := setupMediaTestDB(t)
	defer teardownMediaTestDB(t, db)
	defer cleanupMediaTables(t, db)

	ctx := context.Background()
	cleanupMediaTables(t, db)

	repo := NewMediaRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Setup test data
	chatID := int64(-1003280306634)
	userID := int64(123456789)
	telegramMessageID := int64(1)
	dbMessageID := setupMediaTestData(t, ctx, tx, chatID, userID, telegramMessageID)

	// Insert game without text (NULL)
	game := &models.Game{
		Title:       "Minimal Game",
		Description: "A game without text",
		Text:        "",
	}

	gameID, err := repo.InsertGame(ctx, tx, dbMessageID, game)
	if err != nil {
		t.Fatalf("failed to insert game: %v", err)
	}

	// Verify NULL text field
	var storedText sql.NullString
	err = tx.QueryRowContext(ctx,
		"SELECT text FROM games WHERE id = $1",
		gameID,
	).Scan(&storedText)
	if err != nil {
		t.Fatalf("failed to query game: %v", err)
	}

	if storedText.Valid {
		t.Errorf("expected NULL text, got %q", storedText.String)
	}

	// Verify no game photos exist
	var photoCount int
	err = tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM game_photos WHERE game_id = $1",
		gameID,
	).Scan(&photoCount)
	if err != nil {
		t.Fatalf("failed to count game photos: %v", err)
	}

	if photoCount != 0 {
		t.Errorf("expected 0 game photos, got %d", photoCount)
	}
}

// TestInsertPollWithOptions tests inserting a poll with multiple options
func TestInsertPollWithOptions(t *testing.T) {
	db := setupMediaTestDB(t)
	defer teardownMediaTestDB(t, db)
	defer cleanupMediaTables(t, db)

	ctx := context.Background()
	cleanupMediaTables(t, db)

	repo := NewMediaRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Setup test data
	chatID := int64(-1003280306634)
	userID := int64(123456789)
	telegramMessageID := int64(1)
	dbMessageID := setupMediaTestData(t, ctx, tx, chatID, userID, telegramMessageID)

	// Insert poll
	poll := &models.Poll{
		ID:                    "poll_123",
		Question:              "What is your favorite color?",
		Type:                  "regular",
		TotalVoterCount:       10,
		IsClosed:              false,
		IsAnonymous:           true,
		AllowsMultipleAnswers: false,
		Options: []models.PollOption{
			{Text: "Red", VoterCount: 3},
			{Text: "Blue", VoterCount: 5},
			{Text: "Green", VoterCount: 2},
		},
	}

	pollID, err := repo.InsertPoll(ctx, tx, dbMessageID, poll)
	if err != nil {
		t.Fatalf("failed to insert poll: %v", err)
	}

	if pollID == 0 {
		t.Errorf("expected non-zero poll ID, got 0")
	}

	// Insert poll options
	for i, option := range poll.Options {
		err = repo.InsertPollOption(ctx, tx, pollID, i, &option)
		if err != nil {
			t.Fatalf("failed to insert poll option %d: %v", i, err)
		}
	}

	// Verify poll insertion
	var storedQuestion string
	var totalVoters int
	err = tx.QueryRowContext(ctx,
		"SELECT question, total_voter_count FROM polls WHERE id = $1",
		pollID,
	).Scan(&storedQuestion, &totalVoters)
	if err != nil {
		t.Fatalf("failed to query poll: %v", err)
	}

	if storedQuestion != poll.Question {
		t.Errorf("expected question %q, got %q", poll.Question, storedQuestion)
	}
	if totalVoters != poll.TotalVoterCount {
		t.Errorf("expected total voter count %d, got %d", poll.TotalVoterCount, totalVoters)
	}

	// Verify poll options count
	var optionCount int
	err = tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM poll_options WHERE poll_id = $1",
		pollID,
	).Scan(&optionCount)
	if err != nil {
		t.Fatalf("failed to count poll options: %v", err)
	}

	if optionCount != len(poll.Options) {
		t.Errorf("expected %d poll options, got %d", len(poll.Options), optionCount)
	}
}

// TestInsertPollQuizType tests inserting a quiz type poll
func TestInsertPollQuizType(t *testing.T) {
	db := setupMediaTestDB(t)
	defer teardownMediaTestDB(t, db)
	defer cleanupMediaTables(t, db)

	ctx := context.Background()
	cleanupMediaTables(t, db)

	repo := NewMediaRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Setup test data
	chatID := int64(-1003280306634)
	userID := int64(123456789)
	telegramMessageID := int64(1)
	dbMessageID := setupMediaTestData(t, ctx, tx, chatID, userID, telegramMessageID)

	// Insert quiz poll with correct option
	correctOptionID := 1
	poll := &models.Poll{
		ID:                    "quiz_123",
		Question:              "What is 2 + 2?",
		Type:                  "quiz",
		TotalVoterCount:       5,
		IsClosed:              false,
		IsAnonymous:           true,
		AllowsMultipleAnswers: false,
		CorrectOptionID:       &correctOptionID,
		Explanation:           "The answer is 4",
		Options: []models.PollOption{
			{Text: "3", VoterCount: 1},
			{Text: "4", VoterCount: 4},
			{Text: "5", VoterCount: 0},
		},
	}

	pollID, err := repo.InsertPoll(ctx, tx, dbMessageID, poll)
	if err != nil {
		t.Fatalf("failed to insert quiz poll: %v", err)
	}

	// Verify quiz-specific fields
	var storedType, storedExplanation string
	var storedCorrectOption int
	err = tx.QueryRowContext(ctx,
		"SELECT poll_type, correct_option_id, explanation FROM polls WHERE id = $1",
		pollID,
	).Scan(&storedType, &storedCorrectOption, &storedExplanation)
	if err != nil {
		t.Fatalf("failed to query quiz poll: %v", err)
	}

	if storedType != "quiz" {
		t.Errorf("expected poll type %q, got %q", "quiz", storedType)
	}
	if storedCorrectOption != correctOptionID {
		t.Errorf("expected correct option ID %d, got %d", correctOptionID, storedCorrectOption)
	}
	if storedExplanation != poll.Explanation {
		t.Errorf("expected explanation %q, got %q", poll.Explanation, storedExplanation)
	}
}

// TestInsertContactAllFields tests inserting a contact with all fields
func TestInsertContactAllFields(t *testing.T) {
	db := setupMediaTestDB(t)
	defer teardownMediaTestDB(t, db)
	defer cleanupMediaTables(t, db)

	ctx := context.Background()
	cleanupMediaTables(t, db)

	repo := NewMediaRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Setup test data
	chatID := int64(-1003280306634)
	userID := int64(123456789)
	telegramMessageID := int64(1)
	dbMessageID := setupMediaTestData(t, ctx, tx, chatID, userID, telegramMessageID)

	// Insert contact with all fields populated
	contactUserID := int64(987654321)
	contact := &models.Contact{
		PhoneNumber: "+1234567890",
		FirstName:   "John",
		LastName:    "Doe",
		UserID:      &contactUserID,
		VCard:       "BEGIN:VCARD\nVERSION:3.0\nFN:John Doe\nEND:VCARD",
	}

	err = repo.InsertContact(ctx, tx, dbMessageID, contact)
	if err != nil {
		t.Fatalf("failed to insert contact: %v", err)
	}

	// Verify contact insertion
	var storedFirstName, storedLastName, storedPhone string
	var storedUserID int64
	err = tx.QueryRowContext(ctx,
		"SELECT first_name, last_name, phone_number, user_id FROM contacts WHERE message_id = $1",
		dbMessageID,
	).Scan(&storedFirstName, &storedLastName, &storedPhone, &storedUserID)
	if err != nil {
		t.Fatalf("failed to query contact: %v", err)
	}

	if storedFirstName != contact.FirstName {
		t.Errorf("expected first name %q, got %q", contact.FirstName, storedFirstName)
	}
	if storedLastName != contact.LastName {
		t.Errorf("expected last name %q, got %q", contact.LastName, storedLastName)
	}
	if storedPhone != contact.PhoneNumber {
		t.Errorf("expected phone number %q, got %q", contact.PhoneNumber, storedPhone)
	}
	if storedUserID != contactUserID {
		t.Errorf("expected user ID %d, got %d", contactUserID, storedUserID)
	}
}

// TestInsertLocationCoordinatesOnly tests inserting a location with coordinates only
func TestInsertLocationCoordinatesOnly(t *testing.T) {
	db := setupMediaTestDB(t)
	defer teardownMediaTestDB(t, db)
	defer cleanupMediaTables(t, db)

	ctx := context.Background()
	cleanupMediaTables(t, db)

	repo := NewMediaRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Setup test data
	chatID := int64(-1003280306634)
	userID := int64(123456789)
	telegramMessageID := int64(1)
	dbMessageID := setupMediaTestData(t, ctx, tx, chatID, userID, telegramMessageID)

	// Insert location with only coordinates
	location := &models.Location{
		Latitude:  40.7128,
		Longitude: -74.0060,
	}

	err = repo.InsertLocation(ctx, tx, dbMessageID, location)
	if err != nil {
		t.Fatalf("failed to insert location: %v", err)
	}

	// Verify location insertion
	var storedLat, storedLon float64
	err = tx.QueryRowContext(ctx,
		"SELECT latitude, longitude FROM locations WHERE message_id = $1",
		dbMessageID,
	).Scan(&storedLat, &storedLon)
	if err != nil {
		t.Fatalf("failed to query location: %v", err)
	}

	if storedLat != location.Latitude {
		t.Errorf("expected latitude %f, got %f", location.Latitude, storedLat)
	}
	if storedLon != location.Longitude {
		t.Errorf("expected longitude %f, got %f", location.Longitude, storedLon)
	}
}

// TestInsertVenueWithLocation tests inserting a venue with location
func TestInsertVenueWithLocation(t *testing.T) {
	db := setupMediaTestDB(t)
	defer teardownMediaTestDB(t, db)
	defer cleanupMediaTables(t, db)

	ctx := context.Background()
	cleanupMediaTables(t, db)

	repo := NewMediaRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Setup test data
	chatID := int64(-1003280306634)
	userID := int64(123456789)
	telegramMessageID := int64(1)
	dbMessageID := setupMediaTestData(t, ctx, tx, chatID, userID, telegramMessageID)

	// Insert location first
	location := &models.Location{
		Latitude:  40.7128,
		Longitude: -74.0060,
	}

	locationID, err := repo.InsertLocationReturningID(ctx, tx, dbMessageID, location)
	if err != nil {
		t.Fatalf("failed to insert location: %v", err)
	}

	// Insert venue with location reference
	venue := &models.Venue{
		Title:   "Test Venue",
		Address: "123 Test Street, Test City",
	}

	err = repo.InsertVenue(ctx, tx, dbMessageID, locationID, venue)
	if err != nil {
		t.Fatalf("failed to insert venue: %v", err)
	}

	// Verify venue insertion
	var storedTitle, storedAddress string
	var storedLocationID int64
	err = tx.QueryRowContext(ctx,
		"SELECT title, address, location_id FROM venues WHERE message_id = $1",
		dbMessageID,
	).Scan(&storedTitle, &storedAddress, &storedLocationID)
	if err != nil {
		t.Fatalf("failed to query venue: %v", err)
	}

	if storedTitle != venue.Title {
		t.Errorf("expected title %q, got %q", venue.Title, storedTitle)
	}
	if storedAddress != venue.Address {
		t.Errorf("expected address %q, got %q", venue.Address, storedAddress)
	}
	if storedLocationID != locationID {
		t.Errorf("expected location ID %d, got %d", locationID, storedLocationID)
	}
}

// TestInsertDiceValidEmojiAndValue tests inserting a dice with valid emoji and value
func TestInsertDiceValidEmojiAndValue(t *testing.T) {
	db := setupMediaTestDB(t)
	defer teardownMediaTestDB(t, db)
	defer cleanupMediaTables(t, db)

	ctx := context.Background()
	cleanupMediaTables(t, db)

	repo := NewMediaRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Setup test data
	chatID := int64(-1003280306634)
	userID := int64(123456789)
	telegramMessageID := int64(1)
	dbMessageID := setupMediaTestData(t, ctx, tx, chatID, userID, telegramMessageID)

	// Insert dice
	dice := &models.Dice{
		Emoji: "🎲",
		Value: 4,
	}

	err = repo.InsertDice(ctx, tx, dbMessageID, dice)
	if err != nil {
		t.Fatalf("failed to insert dice: %v", err)
	}

	// Verify dice insertion
	var storedEmoji string
	var storedValue int
	err = tx.QueryRowContext(ctx,
		"SELECT emoji, dice_value FROM dice WHERE message_id = $1",
		dbMessageID,
	).Scan(&storedEmoji, &storedValue)
	if err != nil {
		t.Fatalf("failed to query dice: %v", err)
	}

	if storedEmoji != dice.Emoji {
		t.Errorf("expected emoji %q, got %q", dice.Emoji, storedEmoji)
	}
	if storedValue != dice.Value {
		t.Errorf("expected value %d, got %d", dice.Value, storedValue)
	}
}

// TestContentAddressableStorage tests that files with the same hash use the same path
func TestContentAddressableStorage(t *testing.T) {
	db := setupMediaTestDB(t)
	defer teardownMediaTestDB(t, db)
	defer cleanupMediaTables(t, db)

	ctx := context.Background()
	cleanupMediaTables(t, db)

	repo := NewMediaRepository(db, nil)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Setup test data for two messages
	chatID := int64(-1003280306634)
	userID := int64(123456789)
	telegramMessageID1 := int64(1)
	telegramMessageID2 := int64(2)

	dbMessageID1 := setupMediaTestData(t, ctx, tx, chatID, userID, telegramMessageID1)

	// Insert second message
	msgRepo := NewMessageRepository(nil, nil)
	msg2 := &models.Message{
		MessageID: telegramMessageID2,
		Chat: models.Chat{
			ID:   chatID,
			Type: "supergroup",
		},
		From: &models.User{
			ID:        userID,
			FirstName: "Test User",
		},
		Date: 1704067260, // 2024-01-01 00:01:00 UTC
		Text: "Second test message",
	}
	dbMessageID2, err := msgRepo.InsertMessage(ctx, tx, msg2)
	if err != nil {
		t.Fatalf("failed to setup second message: %v", err)
	}

	// Test scenario: Same file shared in two different messages
	// Both should use the same object key (content-addressable storage)
	sharedHash := "content_addressable_hash"
	sharedObjectKey := "photo/co/content_addressable_hash"
	fileSize := int64(256000)

	// Insert first photo
	photo1 := &models.PhotoSize{
		FileID:       "photo_file_id_1",
		FileUniqueID: "photo_file_unique_id_1",
		Width:        800,
		Height:       600,
		FileSize:     &fileSize,
	}

	err = repo.InsertPhoto(ctx, tx, dbMessageID1, photo1, sharedObjectKey, sharedHash)
	if err != nil {
		t.Fatalf("failed to insert first photo: %v", err)
	}

	// Insert second photo with same hash
	photo2 := &models.PhotoSize{
		FileID:       "photo_file_id_2",
		FileUniqueID: "photo_file_unique_id_2",
		Width:        800,
		Height:       600,
		FileSize:     &fileSize,
	}

	err = repo.InsertPhoto(ctx, tx, dbMessageID2, photo2, sharedObjectKey, sharedHash)
	if err != nil {
		t.Fatalf("failed to insert second photo: %v", err)
	}

	// Verify both photos use the same object key
	rows, err := tx.QueryContext(ctx,
		"SELECT minio_object_key, file_hash FROM photos WHERE file_hash = $1 ORDER BY message_id",
		sharedHash,
	)
	if err != nil {
		t.Fatalf("failed to query photos: %v", err)
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var objectKey, hash string
		err = rows.Scan(&objectKey, &hash)
		if err != nil {
			t.Fatalf("failed to scan photo row: %v", err)
		}

		if objectKey != sharedObjectKey {
			t.Errorf("expected object key %q, got %q", sharedObjectKey, objectKey)
		}
		if hash != sharedHash {
			t.Errorf("expected hash %q, got %q", sharedHash, hash)
		}
		count++
	}

	if count != 2 {
		t.Errorf("expected 2 photos with same hash, got %d", count)
	}

	// Test deduplication check
	foundKey, err := repo.GetObjectKeyByHash(ctx, tx, sharedHash)
	if err != nil {
		t.Fatalf("failed to get object key by hash: %v", err)
	}

	if foundKey != sharedObjectKey {
		t.Errorf("deduplication check failed: expected %q, got %q", sharedObjectKey, foundKey)
	}
}
