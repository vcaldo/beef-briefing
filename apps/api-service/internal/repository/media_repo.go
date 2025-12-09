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

// GetObjectKeyByHash checks if a file with the given hash already exists in any media table.
// Returns the existing object key if found, empty string if not found.
// This enables complete deduplication across all media types (photos, media_files, stickers, game_photos).
func (r *MediaRepository) GetObjectKeyByHash(ctx context.Context, tx *sql.Tx, fileHash string) (string, error) {
	query := `
		SELECT minio_object_key FROM media_files WHERE file_hash = $1
		UNION
		SELECT minio_object_key FROM photos WHERE file_hash = $1
		UNION
		SELECT minio_object_key FROM game_photos WHERE file_hash = $1
		LIMIT 1
	`

	var objectKey string
	err := tx.QueryRowContext(ctx, query, fileHash).Scan(&objectKey)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to query object key by hash: %w", err)
	}

	return objectKey, nil
}

// InsertMediaFile inserts a media file record
// objectKey and fileHash can be empty strings if the file was not downloaded/stored
func (r *MediaRepository) InsertMediaFile(ctx context.Context, tx *sql.Tx, messageID int64, mediaType, fileID, fileUniqueID, objectKey, fileHash string, fileSize *int64, mimeType, fileName string, duration, width, height *int, performer, title string) error {
	query := `
		INSERT INTO media_files (
			message_id, media_type, telegram_file_id, telegram_file_unique_id,
			minio_object_key, file_hash, file_size, mime_type, file_name,
			duration, width, height, performer, title
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err := tx.ExecContext(ctx, query,
		messageID,
		mediaType,
		fileID,
		fileUniqueID,
		NullString(objectKey), // NULL if file wasn't downloaded
		NullString(fileHash),  // NULL if file wasn't downloaded
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
func (r *MediaRepository) InsertPhoto(ctx context.Context, tx *sql.Tx, messageID int64, photo *models.PhotoSize, objectKey, fileHash string) error {
	query := `
		INSERT INTO photos (
			message_id, telegram_file_id, telegram_file_unique_id,
			minio_object_key, file_hash, width, height, file_size
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := tx.ExecContext(ctx, query,
		messageID,
		photo.FileID,
		photo.FileUniqueID,
		objectKey,
		fileHash,
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

// InsertLocationReturningID inserts a location record and returns the generated ID
func (r *MediaRepository) InsertLocationReturningID(ctx context.Context, tx *sql.Tx, messageID int64, location *models.Location) (int64, error) {
	query := `
		INSERT INTO locations (
			message_id, latitude, longitude, horizontal_accuracy,
			live_period, heading, proximity_alert_radius
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	var locationID int64
	err := tx.QueryRowContext(ctx, query,
		messageID,
		location.Latitude,
		location.Longitude,
		NullFloat64(location.HorizontalAccuracy),
		NullInt32(location.LivePeriod),
		NullInt32(location.Heading),
		NullInt32(location.ProximityAlertRadius),
	).Scan(&locationID)
	if err != nil {
		return 0, fmt.Errorf("failed to insert location: %w", err)
	}

	return locationID, nil
}

// InsertSticker inserts a sticker record (sticker file goes in media_files, this stores sticker-specific metadata)
func (r *MediaRepository) InsertSticker(ctx context.Context, tx *sql.Tx, messageID, mediaFileID int64, sticker *models.Sticker) error {
	query := `
		INSERT INTO stickers (
			message_id, media_file_id, sticker_type, emoji, set_name,
			is_animated, is_video, custom_emoji_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := tx.ExecContext(ctx, query,
		messageID,
		mediaFileID,
		sticker.Type,
		NullString(sticker.Emoji),
		NullString(sticker.SetName),
		sticker.IsAnimated,
		sticker.IsVideo,
		NullString(sticker.CustomEmojiID),
	)
	if err != nil {
		return fmt.Errorf("failed to insert sticker: %w", err)
	}

	return nil
}

// InsertMediaFileReturningID inserts a media file record and returns the generated ID
func (r *MediaRepository) InsertMediaFileReturningID(ctx context.Context, tx *sql.Tx, messageID int64, mediaType, fileID, fileUniqueID, objectKey, fileHash string, fileSize *int64, mimeType, fileName string, duration, width, height *int, performer, title string) (int64, error) {
	query := `
		INSERT INTO media_files (
			message_id, media_type, telegram_file_id, telegram_file_unique_id,
			minio_object_key, file_hash, file_size, mime_type, file_name,
			duration, width, height, performer, title
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id
	`

	var mediaFileID int64
	err := tx.QueryRowContext(ctx, query,
		messageID,
		mediaType,
		fileID,
		fileUniqueID,
		objectKey,
		fileHash,
		NullInt64(fileSize),
		NullString(mimeType),
		NullString(fileName),
		NullInt32(duration),
		NullInt32(width),
		NullInt32(height),
		NullString(performer),
		NullString(title),
	).Scan(&mediaFileID)
	if err != nil {
		return 0, fmt.Errorf("failed to insert media file: %w", err)
	}

	return mediaFileID, nil
}

// InsertGame inserts a game record and returns the generated ID
func (r *MediaRepository) InsertGame(ctx context.Context, tx *sql.Tx, messageID int64, game *models.Game) (int64, error) {
	query := `
		INSERT INTO games (message_id, title, description, text)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`

	var gameID int64
	err := tx.QueryRowContext(ctx, query,
		messageID,
		game.Title,
		game.Description,
		NullString(game.Text),
	).Scan(&gameID)
	if err != nil {
		return 0, fmt.Errorf("failed to insert game: %w", err)
	}

	return gameID, nil
}

// InsertGamePhoto inserts a game photo record
func (r *MediaRepository) InsertGamePhoto(ctx context.Context, tx *sql.Tx, gameID int64, photo *models.PhotoSize, objectKey, fileHash string) error {
	query := `
		INSERT INTO game_photos (
			game_id, telegram_file_id, telegram_file_unique_id,
			minio_object_key, file_hash, width, height, file_size
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := tx.ExecContext(ctx, query,
		gameID,
		photo.FileID,
		photo.FileUniqueID,
		objectKey,
		fileHash,
		photo.Width,
		photo.Height,
		NullInt64(photo.FileSize),
	)
	if err != nil {
		return fmt.Errorf("failed to insert game photo: %w", err)
	}

	return nil
}

// InsertPoll inserts a poll record and returns the generated ID
func (r *MediaRepository) InsertPoll(ctx context.Context, tx *sql.Tx, messageID int64, poll *models.Poll) (int64, error) {
	query := `
		INSERT INTO polls (
			message_id, telegram_poll_id, question, poll_type,
			total_voter_count, is_closed, is_anonymous,
			allows_multiple_answers, correct_option_id, explanation,
			open_period, close_date
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id
	`

	var pollID int64
	err := tx.QueryRowContext(ctx, query,
		messageID,
		poll.ID,
		poll.Question,
		poll.Type,
		poll.TotalVoterCount,
		poll.IsClosed,
		poll.IsAnonymous,
		poll.AllowsMultipleAnswers,
		NullInt32(poll.CorrectOptionID),
		NullString(poll.Explanation),
		NullInt32(poll.OpenPeriod),
		NullTimeFromUnix(poll.CloseDate),
	).Scan(&pollID)
	if err != nil {
		return 0, fmt.Errorf("failed to insert poll: %w", err)
	}

	return pollID, nil
}

// InsertPollOption inserts a poll option record
func (r *MediaRepository) InsertPollOption(ctx context.Context, tx *sql.Tx, pollID int64, index int, option *models.PollOption) error {
	query := `
		INSERT INTO poll_options (poll_id, option_index, option_text, voter_count)
		VALUES ($1, $2, $3, $4)
	`

	_, err := tx.ExecContext(ctx, query,
		pollID,
		index,
		option.Text,
		option.VoterCount,
	)
	if err != nil {
		return fmt.Errorf("failed to insert poll option: %w", err)
	}

	return nil
}

// InsertContact inserts a contact record
func (r *MediaRepository) InsertContact(ctx context.Context, tx *sql.Tx, messageID int64, contact *models.Contact) error {
	query := `
		INSERT INTO contacts (
			message_id, phone_number, first_name, last_name, user_id, vcard
		) VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := tx.ExecContext(ctx, query,
		messageID,
		contact.PhoneNumber,
		contact.FirstName,
		NullString(contact.LastName),
		NullInt64(contact.UserID),
		NullString(contact.VCard),
	)
	if err != nil {
		return fmt.Errorf("failed to insert contact: %w", err)
	}

	return nil
}

// InsertVenue inserts a venue record
func (r *MediaRepository) InsertVenue(ctx context.Context, tx *sql.Tx, messageID, locationID int64, venue *models.Venue) error {
	query := `
		INSERT INTO venues (
			message_id, location_id, title, address,
			foursquare_id, foursquare_type, google_place_id, google_place_type
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := tx.ExecContext(ctx, query,
		messageID,
		locationID,
		venue.Title,
		venue.Address,
		NullString(venue.FoursquareID),
		NullString(venue.FoursquareType),
		NullString(venue.GooglePlaceID),
		NullString(venue.GooglePlaceType),
	)
	if err != nil {
		return fmt.Errorf("failed to insert venue: %w", err)
	}

	return nil
}

// InsertDice inserts a dice record
func (r *MediaRepository) InsertDice(ctx context.Context, tx *sql.Tx, messageID int64, dice *models.Dice) error {
	query := `
		INSERT INTO dice (message_id, emoji, dice_value)
		VALUES ($1, $2, $3)
	`

	_, err := tx.ExecContext(ctx, query,
		messageID,
		dice.Emoji,
		dice.Value,
	)
	if err != nil {
		return fmt.Errorf("failed to insert dice: %w", err)
	}

	return nil
}
