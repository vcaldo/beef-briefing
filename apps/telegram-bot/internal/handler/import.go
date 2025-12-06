package handler

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"beef-briefing/apps/telegram-bot/internal/importer"

	tele "gopkg.in/telebot.v4"
)

// HandleImportCommand handles the /import command for importing Telegram export files
func (h *Handler) HandleImportCommand(c tele.Context) error {
	// Check if user is admin
	if !h.config.IsAdmin(c.Sender().ID) {
		slog.Warn("unauthorized import attempt", "user_id", c.Sender().ID, "username", c.Sender().Username)
		return c.Send("❌ Unauthorized. Only administrators can use this command.")
	}

	msg := c.Message()
	var doc *tele.Document

	// Check if document is attached directly or in caption
	if msg.Document != nil {
		doc = msg.Document
	} else if msg.ReplyTo != nil && msg.ReplyTo.Document != nil {
		// Handle case where user replies to a file with /import
		doc = msg.ReplyTo.Document
	} else {
		return c.Send("❌ Please attach or reply to a ZIP file containing the Telegram export.\n\nUsage: /import (attach ZIP file or reply to a file)")
	}

	chatID := c.Chat().ID

	// Validate file size
	maxSize := int64(h.config.MaxImportSizeMB) * 1024 * 1024
	if doc.FileSize > maxSize {
		return c.Send(fmt.Sprintf("❌ File too large. Maximum size: %d MB", h.config.MaxImportSizeMB))
	}

	// Validate file extension (check MIME type and filename)
	isValidZip := doc.MIME == "application/zip" || doc.MIME == "application/x-zip-compressed"
	if !isValidZip && doc.File.UniqueID != "" {
		// Also accept if filename ends with .zip
		if doc.FileName != "" {
			isValidZip = len(doc.FileName) > 4 && doc.FileName[len(doc.FileName)-4:] == ".zip"
		}
	}
	if !isValidZip {
		return c.Send("❌ Invalid file type. Please upload a ZIP file.")
	}

	slog.Info("import started", "chat_id", chatID, "user_id", c.Sender().ID, "file_size", doc.FileSize, "filename", doc.FileName)

	// Send initial status
	statusMsg, err := c.Bot().Send(c.Chat(), "⏳ Downloading export file...")
	if err != nil {
		slog.Error("failed to send status message", "error", err)
	}

	// Create temp file for ZIP
	tempFile, err := os.CreateTemp("", "telegram-import-*.zip")
	if err != nil {
		slog.Error("failed to create temp file", "error", err)
		return c.Send("❌ Failed to process file.")
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Download ZIP file using streaming (handles large files better)
	err = h.downloadTelegramFile(h.bot, doc.FileID, tempFile)
	if err != nil {
		slog.Error("failed to download file", "error", err)
		return c.Send("❌ Failed to download file.")
	}

	// Update status
	if statusMsg != nil {
		c.Bot().Edit(statusMsg, "📦 Extracting archive...")
	}

	// Extract ZIP file
	extractedDir, cleanup, err := importer.ExtractZIP(tempFile.Name())
	if err != nil {
		slog.Error("failed to extract ZIP", "error", err)
		return c.Send(fmt.Sprintf("❌ Failed to extract archive: %v", err))
	}
	defer cleanup()

	// Update status
	if statusMsg != nil {
		c.Bot().Edit(statusMsg, "🔄 Starting import...")
	}

	// Create importer
	imp := importer.NewImporter(h.store, h.minioClient, h.config.ImportChunkSize)

	// Create progress channel
	progressChan := make(chan importer.ImportProgress, 10)

	// Start progress updater goroutine with throttling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go h.updateImportProgress(ctx, c, statusMsg, progressChan)

	// Run import
	if err := imp.Import(context.Background(), chatID, extractedDir, progressChan); err != nil {
		slog.Error("import failed", "error", err)
		close(progressChan)
		return c.Send(fmt.Sprintf("❌ Import failed: %v", err))
	}

	close(progressChan)

	// Wait a bit for final progress update
	time.Sleep(500 * time.Millisecond)

	slog.Info("import completed", "chat_id", chatID)
	return nil
}

// updateImportProgress updates the import status message with throttling
func (h *Handler) updateImportProgress(ctx context.Context, c tele.Context, statusMsg *tele.Message, progressChan <-chan importer.ImportProgress) {
	if statusMsg == nil {
		return
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var lastProgress importer.ImportProgress
	var needsUpdate bool

	for {
		select {
		case <-ctx.Done():
			return

		case progress, ok := <-progressChan:
			if !ok {
				// Channel closed, send final update
				if needsUpdate {
					h.sendProgressUpdate(c, statusMsg, lastProgress, true)
				}
				return
			}
			lastProgress = progress
			needsUpdate = true

		case <-ticker.C:
			if needsUpdate {
				h.sendProgressUpdate(c, statusMsg, lastProgress, false)
				needsUpdate = false
			}
		}
	}
}

// sendProgressUpdate sends a progress update to the user
func (h *Handler) sendProgressUpdate(c tele.Context, statusMsg *tele.Message, progress importer.ImportProgress, isFinal bool) {
	var message string

	if isFinal {
		// Final summary
		message = fmt.Sprintf(
			"✅ <b>Import Complete!</b>\n\n"+
				"📊 <b>Summary:</b>\n"+
				"• Total: %d\n"+
				"• Inserted: %d\n"+
				"• Skipped: %d\n"+
				"• Media: %d\n"+
				"• Errors: %d",
			progress.Total,
			progress.Inserted,
			progress.Skipped,
			progress.MediaUploaded,
			progress.ErrorCount,
		)
	} else {
		// Progress update
		percentage := 0.0
		if progress.Total > 0 {
			percentage = float64(progress.Processed) / float64(progress.Total) * 100
		}

		message = fmt.Sprintf(
			"🔄 <b>Importing...</b>\n\n"+
				"📦 Chunk %d/%d\n"+
				"📈 Progress: %d/%d (%.1f%%)\n\n"+
				"✅ Inserted: %d\n"+
				"⏭️ Skipped: %d\n"+
				"🖼️ Media: %d\n"+
				"❌ Errors: %d",
			progress.CurrentChunk,
			progress.TotalChunks,
			progress.Processed,
			progress.Total,
			percentage,
			progress.Inserted,
			progress.Skipped,
			progress.MediaUploaded,
			progress.ErrorCount,
		)
	}

	if _, err := c.Bot().Edit(statusMsg, message, &tele.SendOptions{ParseMode: tele.ModeHTML}); err != nil {
		slog.Warn("failed to update progress message", "error", err)
	}
}
