package handler

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"beef-briefing/apps/telegram-bot/internal/importer"

	tele "gopkg.in/telebot.v4"
)

// HandleImportCommand handles the /import command for importing Telegram export files
// This command scans the local import directory for ZIP files and processes them
func (h *Handler) HandleImportCommand(c tele.Context) error {
	// Check if user is admin
	if !h.config.IsAdmin(c.Sender().ID) {
		slog.Warn("unauthorized import attempt", "user_id", c.Sender().ID, "username", c.Sender().Username)
		return c.Send("❌ Unauthorized. Only administrators can use this command.")
	}

	chatID := c.Chat().ID

	slog.Info("import triggered", "chat_id", chatID, "user_id", c.Sender().ID, "import_path", h.config.LocalImportPath)

	// Send initial status
	statusMsg, err := c.Bot().Send(c.Chat(), "🔍 Scanning for ZIP files in import directory...")
	if err != nil {
		slog.Error("failed to send status message", "error", err)
	}

	// Scan import directory for ZIP files
	zipFiles, err := h.scanForZipFiles()
	if err != nil {
		slog.Error("failed to scan import directory", "error", err)
		return c.Send(fmt.Sprintf("❌ Failed to scan import directory: %v", err))
	}

	if len(zipFiles) == 0 {
		return c.Send("ℹ️ No ZIP files found in import directory.")
	}

	// Send list of files to be processed
	fileList := "📋 <b>Found ZIP files:</b>\n\n"
	for i, file := range zipFiles {
		fileInfo, _ := os.Stat(file)
		sizeMB := float64(fileInfo.Size()) / 1024 / 1024
		fileList += fmt.Sprintf("%d. %s (%.2f MB)\n", i+1, filepath.Base(file), sizeMB)
	}
	fileList += fmt.Sprintf("\n🔄 Processing %d file(s)...", len(zipFiles))

	if statusMsg != nil {
		c.Bot().Edit(statusMsg, fileList, &tele.SendOptions{ParseMode: tele.ModeHTML})
	}

	// Process each ZIP file
	successCount := 0
	failedCount := 0

	for idx, zipPath := range zipFiles {
		slog.Info("processing zip file", "file", filepath.Base(zipPath), "index", idx+1, "total", len(zipFiles))

		// Update status
		if statusMsg != nil {
			msg := fmt.Sprintf(
				"📦 <b>Processing file %d/%d</b>\n\n"+
					"File: %s\n"+
					"Status: Extracting...",
				idx+1,
				len(zipFiles),
				filepath.Base(zipPath),
			)
			c.Bot().Edit(statusMsg, msg, &tele.SendOptions{ParseMode: tele.ModeHTML})
		}

		// Process the ZIP file
		if err := h.processZipFile(c, statusMsg, chatID, zipPath); err != nil {
			slog.Error("failed to process zip file", "file", filepath.Base(zipPath), "error", err)
			failedCount++
			// Continue with next file even if this one failed
			continue
		}

		// Delete ZIP file after successful processing
		if err := os.Remove(zipPath); err != nil {
			slog.Warn("failed to delete zip file after processing", "file", filepath.Base(zipPath), "error", err)
		} else {
			slog.Info("deleted processed zip file", "file", filepath.Base(zipPath))
		}

		successCount++
	}

	// Send final summary
	summaryMsg := fmt.Sprintf(
		"✅ <b>Batch Import Complete!</b>\n\n"+
			"📊 <b>Summary:</b>\n"+
			"• Total files: %d\n"+
			"• Successful: %d\n"+
			"• Failed: %d",
		len(zipFiles),
		successCount,
		failedCount,
	)

	if statusMsg != nil {
		c.Bot().Edit(statusMsg, summaryMsg, &tele.SendOptions{ParseMode: tele.ModeHTML})
	}

	slog.Info("batch import completed", "chat_id", chatID, "total", len(zipFiles), "success", successCount, "failed", failedCount)
	return nil
}

// scanForZipFiles scans the local import directory for ZIP files
func (h *Handler) scanForZipFiles() ([]string, error) {
	// Check if directory exists
	if _, err := os.Stat(h.config.LocalImportPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("import directory does not exist: %s", h.config.LocalImportPath)
	}

	var zipFiles []string

	// Walk through directory
	err := filepath.Walk(h.config.LocalImportPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if file is a ZIP
		if strings.HasSuffix(strings.ToLower(info.Name()), ".zip") {
			zipFiles = append(zipFiles, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return zipFiles, nil
}

// processZipFile processes a single ZIP file
func (h *Handler) processZipFile(c tele.Context, statusMsg *tele.Message, chatID int64, zipPath string) error {
	// Extract ZIP file
	extractedDir, cleanup, err := importer.ExtractZIP(zipPath)
	if err != nil {
		return fmt.Errorf("failed to extract ZIP: %w", err)
	}
	defer cleanup()

	// Update status
	if statusMsg != nil {
		msg := fmt.Sprintf(
			"📦 <b>Processing:</b> %s\n\n"+
				"Status: Starting import...",
			filepath.Base(zipPath),
		)
		c.Bot().Edit(statusMsg, msg, &tele.SendOptions{ParseMode: tele.ModeHTML})
	}

	// Create importer
	imp := importer.NewImporter(h.store, h.minioClient, h.config.ImportChunkSize)

	// Create progress channel
	progressChan := make(chan importer.ImportProgress, 10)

	// Start progress updater goroutine with throttling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go h.updateImportProgress(ctx, c, statusMsg, progressChan, filepath.Base(zipPath))

	// Run import
	if err := imp.Import(context.Background(), chatID, extractedDir, progressChan); err != nil {
		close(progressChan)
		return fmt.Errorf("import failed: %w", err)
	}

	close(progressChan)

	// Wait a bit for final progress update
	time.Sleep(500 * time.Millisecond)

	return nil
}

// updateImportProgress updates the import status message with throttling
func (h *Handler) updateImportProgress(ctx context.Context, c tele.Context, statusMsg *tele.Message, progressChan <-chan importer.ImportProgress, fileName string) {
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
					h.sendProgressUpdate(c, statusMsg, lastProgress, fileName, true)
				}
				return
			}
			lastProgress = progress
			needsUpdate = true

		case <-ticker.C:
			if needsUpdate {
				h.sendProgressUpdate(c, statusMsg, lastProgress, fileName, false)
				needsUpdate = false
			}
		}
	}
}

// sendProgressUpdate sends a progress update to the user
func (h *Handler) sendProgressUpdate(c tele.Context, statusMsg *tele.Message, progress importer.ImportProgress, fileName string, isFinal bool) {
	var message string

	if isFinal {
		// Final summary
		message = fmt.Sprintf(
			"✅ <b>File Complete:</b> %s\n\n"+
				"📊 <b>Summary:</b>\n"+
				"• Total: %d\n"+
				"• Inserted: %d\n"+
				"• Skipped: %d\n"+
				"• Media: %d\n"+
				"• Errors: %d",
			fileName,
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
			"🔄 <b>Importing:</b> %s\n\n"+
				"📦 Chunk %d/%d\n"+
				"📈 Progress: %d/%d (%.1f%%)\n\n"+
				"✅ Inserted: %d\n"+
				"⏭️ Skipped: %d\n"+
				"🖼️ Media: %d\n"+
				"❌ Errors: %d",
			fileName,
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
