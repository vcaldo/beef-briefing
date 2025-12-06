package importer

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"beef-briefing/apps/telegram-bot/internal/storage"
)

// MediaProcessor handles media file processing for imports
type MediaProcessor struct {
	minioClient  *storage.MinIOClient
	extractedDir string
}

// NewMediaProcessor creates a new media processor
func NewMediaProcessor(minioClient *storage.MinIOClient, extractedDir string) *MediaProcessor {
	return &MediaProcessor{
		minioClient:  minioClient,
		extractedDir: extractedDir,
	}
}

// ProcessMedia reads a media file from the extracted directory and uploads it to MinIO
func (mp *MediaProcessor) ProcessMedia(ctx context.Context, relativePath, mimeType string) (hash string, err error) {
	if relativePath == "" {
		return "", fmt.Errorf("empty media path")
	}

	// Build full path
	fullPath := filepath.Join(mp.extractedDir, relativePath)

	// Read file
	data, err := os.ReadFile(fullPath)
	if err != nil {
		slog.Error("failed to read media file", "path", relativePath, "error", err)
		return "", fmt.Errorf("failed to read media file: %w", err)
	}

	// Upload to MinIO with deduplication
	reader := bytes.NewReader(data)
	hash, err = mp.minioClient.UploadFile(ctx, reader, mimeType)
	if err != nil {
		slog.Error("failed to upload media to MinIO", "path", relativePath, "error", err)
		return "", fmt.Errorf("failed to upload media: %w", err)
	}

	slog.Debug("media uploaded", "path", relativePath, "hash", hash, "size", len(data))
	return hash, nil
}

// DetermineMediaType extracts the message type from export message fields
func DetermineMediaType(exportMsg *ExportMessage) string {
	if exportMsg.Photo != nil {
		return "photo"
	}
	if exportMsg.MediaType != nil {
		switch *exportMsg.MediaType {
		case "video_file":
			return "video"
		case "voice_message":
			return "voice"
		case "animation":
			return "animation"
		case "sticker":
			return "sticker"
		case "video_message":
			return "video_note"
		}
	}
	if exportMsg.File != nil {
		return "document"
	}
	return "text"
}

// GetMediaPath returns the relative path to the media file
func GetMediaPath(exportMsg *ExportMessage) string {
	if exportMsg.Photo != nil {
		return *exportMsg.Photo
	}
	if exportMsg.File != nil {
		return *exportMsg.File
	}
	return ""
}
