package repository

import (
	"context"
	"database/sql"
	"fmt"

	"beef-briefing/apps/api-service/internal/models"
)

type MediaRepository struct {
	db *sql.DB
}

func NewMediaRepository(db *sql.DB) *MediaRepository {
	return &MediaRepository{db: db}
}

// InsertMediaFile inserts a media file record
func (r *MediaRepository) InsertMediaFile(ctx context.Context, tx *sql.Tx, messageID int64, mediaType, fileID, fileUniqueID, objectKey string, fileSize *int64, mimeType, fileName string, duration, width, height *int, performer, title string) error {
	query := `
		INSERT INTO media_files (
			message_id, media_type, telegram_file_id, telegram_file_unique_id,
			minio_object_key, file_size, mime_type, file_name,
			duration, width, height, performer, title
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (telegram_file_id) DO NOTHING
	`

	_, err := tx.ExecContext(ctx, query,
		messageID,
		mediaType,
		fileID,
		fileUniqueID,
		objectKey,
		NullInt64(fileSize),
		NullString(mimeType),
		NullString(fileName),
		NullInt32(duration),
		NullInt32(width),
		NullInt32(height),
		NullString(performer),
		NullString(title),
	)
	if err != nil {
		return fmt.Errorf("failed to insert media file: %w", err)
	}

	return nil
}

// InsertPhoto inserts a photo record
func (r *MediaRepository) InsertPhoto(ctx context.Context, tx *sql.Tx, messageID int64, photo *models.PhotoSize, objectKey string) error {
	query := `
		INSERT INTO photos (
			message_id, telegram_file_id, telegram_file_unique_id,
			minio_object_key, width, height, file_size
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (telegram_file_id) DO NOTHING
	`

	_, err := tx.ExecContext(ctx, query,
		messageID,
		photo.FileID,
		photo.FileUniqueID,
		objectKey,
		photo.Width,
		photo.Height,
		NullInt64(photo.FileSize),
	)
	if err != nil {
		return fmt.Errorf("failed to insert photo: %w", err)
	}

	return nil
}

// InsertLocation inserts a location record
func (r *MediaRepository) InsertLocation(ctx context.Context, tx *sql.Tx, messageID int64, location *models.Location) error {
	query := `
		INSERT INTO locations (
			message_id, latitude, longitude, horizontal_accuracy,
			live_period, heading, proximity_alert_radius
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := tx.ExecContext(ctx, query,
		messageID,
		location.Latitude,
		location.Longitude,
		NullFloat64(location.HorizontalAccuracy),
		NullInt32(location.LivePeriod),
		NullInt32(location.Heading),
		NullInt32(location.ProximityAlertRadius),
	)
	if err != nil {
		return fmt.Errorf("failed to insert location: %w", err)
	}

	return nil
}
