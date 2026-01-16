package repository

import (
	"context"
	"database/sql"
	"fmt"

	"beef-briefing/apps/api-service/internal/models"
	"beef-briefing/apps/api-service/internal/nrutil"

	"github.com/newrelic/go-agent/v3/newrelic"
)

type ProfilePhotoRepository struct {
	db    *sql.DB
	nrApp *newrelic.Application
}

func NewProfilePhotoRepository(db *sql.DB, nrApp *newrelic.Application) *ProfilePhotoRepository {
	return &ProfilePhotoRepository{db: db, nrApp: nrApp}
}

// ReplaceUserPhotos atomically replaces all profile photos for a user
// Deletes existing photos and inserts new ones in a single transaction
func (r *ProfilePhotoRepository) ReplaceUserPhotos(ctx context.Context, tx *sql.Tx, userID int64, photos []models.DBUserProfilePhoto) error {
	defer nrutil.StartSegment(ctx, "db:replace-user-photos")()

	// Delete existing photos
	deleteQuery := `DELETE FROM user_profile_photos WHERE user_id = $1`
	_, err := tx.ExecContext(ctx, deleteQuery, userID)
	if err != nil {
		return fmt.Errorf("failed to delete existing user photos: %w", err)
	}

	// Insert new photos
	insertQuery := `
		INSERT INTO user_profile_photos (
			user_id, telegram_file_id, telegram_file_unique_id,
			minio_object_key, file_hash, width, height, file_size
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	for _, photo := range photos {
		_, err := tx.ExecContext(ctx, insertQuery,
			userID,
			photo.TelegramFileID,
			photo.TelegramFileUniqueID,
			photo.MinIOObjectKey,
			photo.FileHash,
			photo.Width,
			photo.Height,
			photo.FileSize,
		)
		if err != nil {
			return fmt.Errorf("failed to insert user photo: %w", err)
		}
	}

	return nil
}

// ReplaceChatPhotos atomically replaces all profile photos for a chat
// Deletes existing photos and inserts new ones in a single transaction
func (r *ProfilePhotoRepository) ReplaceChatPhotos(ctx context.Context, tx *sql.Tx, chatID int64, photos []models.DBChatProfilePhoto) error {
	defer nrutil.StartSegment(ctx, "db:replace-chat-photos")()

	// Delete existing photos
	deleteQuery := `DELETE FROM chat_profile_photos WHERE chat_id = $1`
	_, err := tx.ExecContext(ctx, deleteQuery, chatID)
	if err != nil {
		return fmt.Errorf("failed to delete existing chat photos: %w", err)
	}

	// Insert new photos
	insertQuery := `
		INSERT INTO chat_profile_photos (
			chat_id, photo_size, telegram_file_id, telegram_file_unique_id,
			minio_object_key, file_hash, width, height, file_size
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	for _, photo := range photos {
		_, err := tx.ExecContext(ctx, insertQuery,
			chatID,
			photo.PhotoSize,
			photo.TelegramFileID,
			photo.TelegramFileUniqueID,
			photo.MinIOObjectKey,
			photo.FileHash,
			photo.Width,
			photo.Height,
			photo.FileSize,
		)
		if err != nil {
			return fmt.Errorf("failed to insert chat photo: %w", err)
		}
	}

	return nil
}

// GetUserPhotos returns all profile photos for a user
func (r *ProfilePhotoRepository) GetUserPhotos(ctx context.Context, userID int64) ([]models.DBUserProfilePhoto, error) {
	defer nrutil.StartSegment(ctx, "db:get-user-photos")()

	query := `
		SELECT id, user_id, telegram_file_id, telegram_file_unique_id,
			minio_object_key, file_hash, width, height, file_size, created_at
		FROM user_profile_photos
		WHERE user_id = $1
		ORDER BY width DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user photos: %w", err)
	}
	defer rows.Close()

	var photos []models.DBUserProfilePhoto
	for rows.Next() {
		var photo models.DBUserProfilePhoto
		err := rows.Scan(
			&photo.ID,
			&photo.UserID,
			&photo.TelegramFileID,
			&photo.TelegramFileUniqueID,
			&photo.MinIOObjectKey,
			&photo.FileHash,
			&photo.Width,
			&photo.Height,
			&photo.FileSize,
			&photo.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user photo: %w", err)
		}
		photos = append(photos, photo)
	}

	return photos, nil
}

// GetChatPhotos returns all profile photos for a chat
func (r *ProfilePhotoRepository) GetChatPhotos(ctx context.Context, chatID int64) ([]models.DBChatProfilePhoto, error) {
	defer nrutil.StartSegment(ctx, "db:get-chat-photos")()

	query := `
		SELECT id, chat_id, photo_size, telegram_file_id, telegram_file_unique_id,
			minio_object_key, file_hash, width, height, file_size, created_at
		FROM chat_profile_photos
		WHERE chat_id = $1
		ORDER BY photo_size
	`

	rows, err := r.db.QueryContext(ctx, query, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to query chat photos: %w", err)
	}
	defer rows.Close()

	var photos []models.DBChatProfilePhoto
	for rows.Next() {
		var photo models.DBChatProfilePhoto
		err := rows.Scan(
			&photo.ID,
			&photo.ChatID,
			&photo.PhotoSize,
			&photo.TelegramFileID,
			&photo.TelegramFileUniqueID,
			&photo.MinIOObjectKey,
			&photo.FileHash,
			&photo.Width,
			&photo.Height,
			&photo.FileSize,
			&photo.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan chat photo: %w", err)
		}
		photos = append(photos, photo)
	}

	return photos, nil
}

// GetAllUserIDs returns all user IDs from the database
func (r *ProfilePhotoRepository) GetAllUserIDs(ctx context.Context) ([]int64, error) {
	defer nrutil.StartSegment(ctx, "db:get-all-user-ids")()

	query := `SELECT id FROM users ORDER BY id`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query user ids: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan user id: %w", err)
		}
		ids = append(ids, id)
	}

	return ids, nil
}

// GetAllChatIDs returns all chat IDs from the database
func (r *ProfilePhotoRepository) GetAllChatIDs(ctx context.Context) ([]int64, error) {
	defer nrutil.StartSegment(ctx, "db:get-all-chat-ids")()

	query := `SELECT id FROM chats ORDER BY id`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query chat ids: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan chat id: %w", err)
		}
		ids = append(ids, id)
	}

	return ids, nil
}

// GetUserPhotoBySize returns a user photo by size (small/medium/large)
// Photos are ordered by width DESC, so:
//   - large = index 0 (largest)
//   - medium = index len/2 (middle)
//   - small = index len-1 (smallest)
//
// Returns nil if user has no photos.
func (r *ProfilePhotoRepository) GetUserPhotoBySize(ctx context.Context, userID int64, size string) (*models.DBUserProfilePhoto, error) {
	photos, err := r.GetUserPhotos(ctx, userID)
	if err != nil {
		return nil, err
	}

	if len(photos) == 0 {
		return nil, nil
	}

	var index int
	switch size {
	case "large":
		index = 0
	case "medium":
		index = len(photos) / 2
	case "small":
		index = len(photos) - 1
	default:
		index = 0 // default to large
	}

	return &photos[index], nil
}

// GetChatPhotoBySize returns a chat photo by size
// Maps large/medium → "big", small → "small"
// Falls back to available size if requested not found
// Returns nil if chat has no photos.
func (r *ProfilePhotoRepository) GetChatPhotoBySize(ctx context.Context, chatID int64, size string) (*models.DBChatProfilePhoto, error) {
	defer nrutil.StartSegment(ctx, "db:get-chat-photo-by-size")()

	// Map size to database photo_size
	dbSize := "big"
	if size == "small" {
		dbSize = "small"
	}

	query := `
		SELECT id, chat_id, photo_size, telegram_file_id, telegram_file_unique_id,
			minio_object_key, file_hash, width, height, file_size, created_at
		FROM chat_profile_photos
		WHERE chat_id = $1 AND photo_size = $2
	`

	var photo models.DBChatProfilePhoto
	err := r.db.QueryRowContext(ctx, query, chatID, dbSize).Scan(
		&photo.ID,
		&photo.ChatID,
		&photo.PhotoSize,
		&photo.TelegramFileID,
		&photo.TelegramFileUniqueID,
		&photo.MinIOObjectKey,
		&photo.FileHash,
		&photo.Width,
		&photo.Height,
		&photo.FileSize,
		&photo.CreatedAt,
	)

	if err == sql.ErrNoRows {
		// Try fallback to the other size
		fallbackSize := "small"
		if dbSize == "small" {
			fallbackSize = "big"
		}

		err = r.db.QueryRowContext(ctx, query, chatID, fallbackSize).Scan(
			&photo.ID,
			&photo.ChatID,
			&photo.PhotoSize,
			&photo.TelegramFileID,
			&photo.TelegramFileUniqueID,
			&photo.MinIOObjectKey,
			&photo.FileHash,
			&photo.Width,
			&photo.Height,
			&photo.FileSize,
			&photo.CreatedAt,
		)

		if err == sql.ErrNoRows {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("failed to query chat photo fallback: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to query chat photo: %w", err)
	}

	return &photo, nil
}
