package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"beef-briefing/apps/api-service/internal/models"
	"beef-briefing/apps/api-service/internal/repository"
	"beef-briefing/apps/api-service/internal/storage"

	"github.com/newrelic/go-agent/v3/newrelic"
)

// ErrPhotoNotFound is returned when no photo exists for a user or chat.
var ErrPhotoNotFound = errors.New("photo not found")

// ProfilePhotoService handles profile photo processing for users and chats.
type ProfilePhotoService struct {
	db               *sql.DB
	storageClient    *storage.MinIOClient
	nrApp            *newrelic.Application
	profilePhotoRepo *repository.ProfilePhotoRepository
	mediaRepo        *repository.MediaRepository
}

// NewProfilePhotoService creates a new ProfilePhotoService.
func NewProfilePhotoService(db *sql.DB, storageClient *storage.MinIOClient, nrApp *newrelic.Application) *ProfilePhotoService {
	return &ProfilePhotoService{
		db:               db,
		storageClient:    storageClient,
		nrApp:            nrApp,
		profilePhotoRepo: repository.NewProfilePhotoRepository(db, nrApp),
		mediaRepo:        repository.NewMediaRepository(db, nrApp),
	}
}

// ProcessUserPhotos stores profile photos for a user, replacing any existing photos.
func (s *ProfilePhotoService) ProcessUserPhotos(ctx context.Context, userID int64, photos []models.ProfilePhotoMeta, files map[string][]byte) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:process-user-photos")
		defer segment.End()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var dbPhotos []models.DBUserProfilePhoto

	for _, photo := range photos {
		data, ok := files[photo.FileID]
		if !ok {
			slog.Warn("user profile photo file not found", "user_id", userID, "file_id", photo.FileID)
			continue
		}

		objectKey, fileHash, err := s.uploadOrReuseMedia(ctx, tx, photo.FileID, data, "image/jpeg", "profile_photo")
		if err != nil {
			slog.Error("failed to upload user profile photo", "user_id", userID, "file_id", photo.FileID, "error", err)
			continue
		}

		dbPhoto := models.DBUserProfilePhoto{
			UserID:               userID,
			TelegramFileID:       photo.FileID,
			TelegramFileUniqueID: photo.FileUniqueID,
			MinIOObjectKey:       objectKey,
			FileHash:             fileHash,
			Width:                photo.Width,
			Height:               photo.Height,
		}
		if photo.FileSize != nil {
			dbPhoto.FileSize = sql.NullInt64{Int64: *photo.FileSize, Valid: true}
		}

		dbPhotos = append(dbPhotos, dbPhoto)
	}

	if len(dbPhotos) == 0 {
		slog.Info("no valid photos to store for user", "user_id", userID)
		return nil
	}

	if err := s.profilePhotoRepo.ReplaceUserPhotos(ctx, tx, userID, dbPhotos); err != nil {
		return fmt.Errorf("failed to replace user photos: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	slog.Info("stored user profile photos", "user_id", userID, "count", len(dbPhotos))
	return nil
}

// ProcessChatPhotos stores profile photos for a chat, replacing any existing photos.
func (s *ProfilePhotoService) ProcessChatPhotos(ctx context.Context, chatID int64, photos []models.ProfilePhotoMeta, files map[string][]byte) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:process-chat-photos")
		defer segment.End()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var dbPhotos []models.DBChatProfilePhoto

	for _, photo := range photos {
		data, ok := files[photo.FileID]
		if !ok {
			slog.Warn("chat profile photo file not found", "chat_id", chatID, "file_id", photo.FileID)
			continue
		}

		objectKey, fileHash, err := s.uploadOrReuseMedia(ctx, tx, photo.FileID, data, "image/jpeg", "chat_photo")
		if err != nil {
			slog.Error("failed to upload chat profile photo", "chat_id", chatID, "file_id", photo.FileID, "error", err)
			continue
		}

		dbPhoto := models.DBChatProfilePhoto{
			ChatID:               chatID,
			PhotoSize:            photo.PhotoSize,
			TelegramFileID:       photo.FileID,
			TelegramFileUniqueID: photo.FileUniqueID,
			MinIOObjectKey:       objectKey,
			FileHash:             fileHash,
		}
		if photo.Width > 0 {
			dbPhoto.Width = sql.NullInt32{Int32: int32(photo.Width), Valid: true}
		}
		if photo.Height > 0 {
			dbPhoto.Height = sql.NullInt32{Int32: int32(photo.Height), Valid: true}
		}
		if photo.FileSize != nil {
			dbPhoto.FileSize = sql.NullInt64{Int64: *photo.FileSize, Valid: true}
		}

		dbPhotos = append(dbPhotos, dbPhoto)
	}

	if len(dbPhotos) == 0 {
		slog.Info("no valid photos to store for chat", "chat_id", chatID)
		return nil
	}

	if err := s.profilePhotoRepo.ReplaceChatPhotos(ctx, tx, chatID, dbPhotos); err != nil {
		return fmt.Errorf("failed to replace chat photos: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	slog.Info("stored chat profile photos", "chat_id", chatID, "count", len(dbPhotos))
	return nil
}

// GetAllUserIDs returns all user IDs from the database.
func (s *ProfilePhotoService) GetAllUserIDs(ctx context.Context) ([]int64, error) {
	return s.profilePhotoRepo.GetAllUserIDs(ctx)
}

// GetAllChatIDs returns all chat IDs from the database.
func (s *ProfilePhotoService) GetAllChatIDs(ctx context.Context) ([]int64, error) {
	return s.profilePhotoRepo.GetAllChatIDs(ctx)
}

// uploadOrReuseMedia handles file deduplication by checking hash before upload.
func (s *ProfilePhotoService) uploadOrReuseMedia(ctx context.Context, tx *sql.Tx, fileID string, data []byte, mimeType, mediaType string) (objectKey, fileHash string, err error) {
	fileHash = storage.ComputeFileHash(data)

	existingKey, err := s.mediaRepo.GetObjectKeyByHash(ctx, tx, fileHash)
	if err != nil {
		return "", "", fmt.Errorf("failed to check file hash: %w", err)
	}

	if existingKey != "" {
		slog.Debug("profile photo already exists in storage, reusing",
			"file_id", fileID,
			"media_type", mediaType,
			"object_key", existingKey,
		)
		return existingKey, fileHash, nil
	}

	objectKey, _, err = s.storageClient.UploadMedia(ctx, fileID, data, mimeType, mediaType)
	if err != nil {
		return "", "", fmt.Errorf("failed to upload to MinIO: %w", err)
	}

	return objectKey, fileHash, nil
}

// GetUserPhoto retrieves a user's profile photo by size.
// Size can be "small", "medium", or "large" (default).
// Returns a pre-signed URL for the photo with 1-hour expiry.
func (s *ProfilePhotoService) GetUserPhoto(ctx context.Context, userID int64, size string) (string, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:get-user-photo")
		defer segment.End()
	}

	photo, err := s.profilePhotoRepo.GetUserPhotoBySize(ctx, userID, size)
	if err != nil {
		return "", fmt.Errorf("failed to get user photo: %w", err)
	}

	if photo == nil {
		return "", ErrPhotoNotFound
	}

	url, err := s.storageClient.GetPresignedURL(ctx, photo.MinIOObjectKey, time.Hour)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	slog.Debug("retrieved user photo URL",
		"user_id", userID,
		"size", size,
		"object_key", photo.MinIOObjectKey,
	)

	return url, nil
}

// GetChatPhoto retrieves a chat's profile photo by size.
// Size can be "small", "medium", or "large" (default).
// Returns a pre-signed URL for the photo with 1-hour expiry.
func (s *ProfilePhotoService) GetChatPhoto(ctx context.Context, chatID int64, size string) (string, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:get-chat-photo")
		defer segment.End()
	}

	photo, err := s.profilePhotoRepo.GetChatPhotoBySize(ctx, chatID, size)
	if err != nil {
		return "", fmt.Errorf("failed to get chat photo: %w", err)
	}

	if photo == nil {
		return "", ErrPhotoNotFound
	}

	url, err := s.storageClient.GetPresignedURL(ctx, photo.MinIOObjectKey, time.Hour)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	slog.Debug("retrieved chat photo URL",
		"chat_id", chatID,
		"size", size,
		"object_key", photo.MinIOObjectKey,
	)

	return url, nil
}
