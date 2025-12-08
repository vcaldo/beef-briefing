package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"beef-briefing/apps/import-cli/internal/client"
	"beef-briefing/apps/import-cli/internal/mapper"
	"beef-briefing/apps/import-cli/internal/models"
	"beef-briefing/apps/import-cli/internal/parser"
	"beef-briefing/apps/import-cli/internal/reporter"
	"beef-briefing/apps/import-cli/internal/state"
)

var (
	// Version information (set at build time)
	version = "dev"

	// Global flags
	verbose bool

	// Import flags
	exportPath   string
	chatID       int64
	createChat   bool
	includeMedia bool
	batchSize    int
	delayMS      int
	apiURL       string
	resetState   bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:     "import-cli",
	Short:   "Telegram export importer for beef-briefing",
	Long:    `A CLI tool to import Telegram Desktop export data into the beef-briefing system.`,
	Version: version,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setupLogging()
	},
}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import messages from a Telegram export",
	Long: `Import messages from a Telegram Desktop export folder into the beef-briefing system.

The export folder should contain a result.json file with the exported messages.

Examples:
  # Import with existing chat ID
  import-cli import --chat-id 2572302334 --export-path ./local_import

  # Auto-create chat from export metadata
  import-cli import --create-chat --export-path ./local_import

  # Import with media files
  import-cli import --chat-id 2572302334 --export-path ./local_import --include-media

  # Custom batch size and delay
  import-cli import --chat-id 2572302334 --export-path ./local_import --batch-size 50 --delay-ms 5`,
	RunE: runImport,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show import status for an export folder",
	Long:  `Display the current import status including progress, errors, and user mappings.`,
	RunE:  runStatus,
}

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset import state for an export folder",
	Long:  `Remove the import state file to start a fresh import.`,
	RunE:  runReset,
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")

	// Import command flags
	importCmd.Flags().StringVarP(&exportPath, "export-path", "e", "", "Path to the Telegram export folder (required)")
	importCmd.Flags().Int64VarP(&chatID, "chat-id", "c", 0, "Target chat ID (required unless --create-chat is used)")
	importCmd.Flags().BoolVar(&createChat, "create-chat", false, "Auto-create chat from export metadata if not exists")
	importCmd.Flags().BoolVarP(&includeMedia, "include-media", "m", false, "Import media files (photos, videos, etc.)")
	importCmd.Flags().IntVarP(&batchSize, "batch-size", "b", 100, "Number of messages per batch")
	importCmd.Flags().IntVarP(&delayMS, "delay-ms", "d", 1, "Delay between batches in milliseconds")
	importCmd.Flags().StringVarP(&apiURL, "api-url", "u", "http://localhost:8080", "API service URL")
	importCmd.Flags().BoolVar(&resetState, "reset", false, "Reset import state before starting")

	importCmd.MarkFlagRequired("export-path")

	// Status command flags
	statusCmd.Flags().StringVarP(&exportPath, "export-path", "e", "", "Path to the Telegram export folder (required)")
	statusCmd.MarkFlagRequired("export-path")

	// Reset command flags
	resetCmd.Flags().StringVarP(&exportPath, "export-path", "e", "", "Path to the Telegram export folder (required)")
	resetCmd.MarkFlagRequired("export-path")

	// Add commands to root
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(resetCmd)
}

func setupLogging() {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(handler))
}

func runImport(cmd *cobra.Command, args []string) error {
	startTime := time.Now()

	// Validate flags
	if chatID == 0 && !createChat {
		return fmt.Errorf("either --chat-id or --create-chat must be specified")
	}

	// Validate export path
	resultPath := filepath.Join(exportPath, "result.json")
	if _, err := os.Stat(resultPath); os.IsNotExist(err) {
		return fmt.Errorf("result.json not found in export path: %s", exportPath)
	}

	slog.Info("starting import",
		"export_path", exportPath,
		"chat_id", chatID,
		"create_chat", createChat,
		"include_media", includeMedia,
		"batch_size", batchSize,
		"delay_ms", delayMS,
	)

	// Initialize parser
	p := parser.New(resultPath)

	// Parse metadata
	metadata, err := p.ParseMetadata()
	if err != nil {
		return fmt.Errorf("parsing export metadata: %w", err)
	}

	slog.Info("export metadata",
		"name", metadata.Name,
		"type", metadata.Type,
		"id", metadata.ID,
	)

	// Determine chat ID
	targetChatID := chatID
	if createChat {
		targetChatID = metadata.ID
		slog.Info("using chat ID from export", "chat_id", targetChatID)
	}

	// Initialize state manager
	stateMgr := state.NewManager(exportPath)

	// Handle reset
	if resetState {
		if err := stateMgr.Delete(); err != nil {
			slog.Warn("could not delete state file", "error", err)
		}
		slog.Info("import state reset")
	}

	// Load existing state
	if err := stateMgr.Load(); err != nil {
		return fmt.Errorf("loading import state: %w", err)
	}

	// Check for resume
	existingState := stateMgr.GetState()
	if existingState.LastProcessedID > 0 {
		slog.Info("resuming import",
			"last_processed_id", existingState.LastProcessedID,
			"imported_so_far", existingState.ImportedCount,
		)
	}

	// Set state metadata
	stateMgr.SetChatID(targetChatID)
	if existingState.StartedAt == "" {
		stateMgr.SetStartedAt(time.Now().Format(time.RFC3339))
	}

	// Initialize mapper
	m := mapper.New(targetChatID, metadata.Type, metadata.Name)

	// Initialize API client
	apiClient := client.New(apiURL, batchSize, delayMS)

	// Count messages for progress (if not already counted)
	if existingState.TotalMessages == 0 {
		slog.Info("counting messages...")
		count, err := p.CountMessages()
		if err != nil {
			return fmt.Errorf("counting messages: %w", err)
		}
		stateMgr.SetTotalMessages(count)
		slog.Info("total messages", "count", count)
	}

	// Process messages
	var batch []*models.Update
	var batchMediaPaths []string
	processedCount := 0
	lastSaveTime := time.Now()

	err = p.StreamMessages(func(msg *models.ExportMessage) error {
		processedCount++

		// Show progress every 1000 messages
		if processedCount%1000 == 0 {
			slog.Info("progress",
				"processed", processedCount,
				"imported", stateMgr.GetState().ImportedCount,
				"skipped", stateMgr.GetState().SkippedCount,
				"failed", stateMgr.GetState().FailedCount,
			)
		}

		// Skip if already processed (resume support)
		if stateMgr.ShouldSkip(msg.ID) {
			return nil
		}

		// Skip service messages
		if msg.IsServiceMessage() {
			stateMgr.IncrementSkipped()
			stateMgr.SetLastProcessedID(msg.ID)
			return nil
		}

		// Record user
		userID, userName := parseUserInfo(msg.From, msg.FromID)
		if userID != 0 {
			stateMgr.AddUser(userID, userName)
		}

		// Convert to API format
		update, err := m.ToUpdate(msg)
		if err != nil {
			stateMgr.IncrementFailed()
			stateMgr.AddError(msg.ID, err.Error(), time.Now().Format(time.RFC3339))
			stateMgr.SetLastProcessedID(msg.ID)
			return nil // Continue processing
		}

		if update == nil {
			stateMgr.IncrementSkipped()
			stateMgr.SetLastProcessedID(msg.ID)
			return nil
		}

		// Add to batch
		batch = append(batch, update)
		if includeMedia && msg.HasMedia() {
			batchMediaPaths = append(batchMediaPaths, msg.GetMediaPath())
		} else {
			batchMediaPaths = append(batchMediaPaths, "")
		}

		// Send batch when full
		if len(batch) >= batchSize {
			if err := sendBatch(apiClient, batch, batchMediaPaths, exportPath, includeMedia, stateMgr); err != nil {
				slog.Error("batch send error", "error", err)
			}
			batch = nil
			batchMediaPaths = nil

			// Save state periodically (every 30 seconds)
			if time.Since(lastSaveTime) > 30*time.Second {
				stateMgr.SetLastUpdatedAt(time.Now().Format(time.RFC3339))
				if err := stateMgr.Save(); err != nil {
					slog.Warn("could not save state", "error", err)
				}
				lastSaveTime = time.Now()
			}
		}

		stateMgr.SetLastProcessedID(msg.ID)
		return nil
	})

	if err != nil {
		return fmt.Errorf("processing messages: %w", err)
	}

	// Send remaining batch
	if len(batch) > 0 {
		if err := sendBatch(apiClient, batch, batchMediaPaths, exportPath, includeMedia, stateMgr); err != nil {
			slog.Error("final batch send error", "error", err)
		}
	}

	// Final state update
	stateMgr.SetLastUpdatedAt(time.Now().Format(time.RFC3339))
	if err := stateMgr.Save(); err != nil {
		slog.Warn("could not save final state", "error", err)
	}

	// Generate report
	duration := time.Since(startTime)
	rep := reporter.New(exportPath)
	if err := rep.Generate(stateMgr.GetState(), metadata.Name, duration); err != nil {
		slog.Error("could not generate report", "error", err)
	} else {
		slog.Info("report generated", "path", rep.GetReportPath())
	}

	// Print summary
	state := stateMgr.GetState()
	fmt.Println()
	fmt.Println("=== Import Complete ===")
	fmt.Printf("Duration: %s\n", duration.Round(time.Second))
	fmt.Printf("Total Messages: %d\n", state.TotalMessages)
	fmt.Printf("Imported: %d\n", state.ImportedCount)
	fmt.Printf("Skipped: %d\n", state.SkippedCount)
	fmt.Printf("Failed: %d\n", state.FailedCount)
	fmt.Printf("Report: %s\n", rep.GetReportPath())

	return nil
}

func sendBatch(apiClient *client.Client, batch []*models.Update, mediaPaths []string, exportPath string, includeMedia bool, stateMgr *state.Manager) error {
	for i, update := range batch {
		var err error

		if includeMedia && mediaPaths[i] != "" {
			err = apiClient.SendUpdateWithMedia(update, mediaPaths[i], exportPath)
			updateMediaStats(stateMgr, update, err)
		} else {
			err = apiClient.SendUpdate(update)
		}

		if err != nil {
			stateMgr.IncrementFailed()
			stateMgr.AddError(update.Message.MessageID, err.Error(), time.Now().Format(time.RFC3339))
			slog.Debug("message send failed", "message_id", update.Message.MessageID, "error", err)
		} else {
			stateMgr.IncrementImported()
		}

		// Add delay between messages
		if i < len(batch)-1 && delayMS > 0 {
			time.Sleep(time.Duration(delayMS) * time.Millisecond)
		}
	}

	return nil
}

func updateMediaStats(stateMgr *state.Manager, update *models.Update, err error) {
	if update.Message == nil {
		return
	}

	stats := stateMgr.GetMediaStats()
	msg := update.Message

	if len(msg.Photo) > 0 {
		if err != nil {
			stats.PhotosFailed++
		} else {
			stats.PhotosImported++
		}
	} else if msg.Video != nil {
		if err != nil {
			stats.VideosFailed++
		} else {
			stats.VideosImported++
		}
	} else if msg.Animation != nil {
		if err != nil {
			stats.AnimationsFailed++
		} else {
			stats.AnimationsImported++
		}
	} else if msg.Voice != nil {
		if err != nil {
			stats.VoicesFailed++
		} else {
			stats.VoicesImported++
		}
	} else if msg.Document != nil {
		if err != nil {
			stats.DocumentsFailed++
		} else {
			stats.DocumentsImported++
		}
	}
}

func parseUserInfo(name, fromID string) (int64, string) {
	displayName := name
	if displayName == "" {
		displayName = fromID
	}

	var userID int64
	if len(fromID) > 4 && fromID[:4] == "user" {
		fmt.Sscanf(fromID[4:], "%d", &userID)
	} else if len(fromID) > 7 && fromID[:7] == "channel" {
		fmt.Sscanf(fromID[7:], "%d", &userID)
	}

	return userID, displayName
}

func runStatus(cmd *cobra.Command, args []string) error {
	stateMgr := state.NewManager(exportPath)

	if err := stateMgr.Load(); err != nil {
		return fmt.Errorf("loading import state: %w", err)
	}

	state := stateMgr.GetState()

	if state.LastProcessedID == 0 {
		fmt.Println("No import in progress for this export folder.")
		return nil
	}

	fmt.Println("=== Import Status ===")
	fmt.Printf("Chat ID: %d\n", state.ChatID)
	fmt.Printf("Started At: %s\n", state.StartedAt)
	fmt.Printf("Last Updated: %s\n", state.LastUpdatedAt)
	fmt.Printf("Last Processed ID: %d\n", state.LastProcessedID)
	fmt.Println()
	fmt.Printf("Total Messages: %d\n", state.TotalMessages)
	fmt.Printf("Imported: %d\n", state.ImportedCount)
	fmt.Printf("Skipped: %d\n", state.SkippedCount)
	fmt.Printf("Failed: %d\n", state.FailedCount)

	if state.TotalMessages > 0 {
		progress := float64(state.ImportedCount+state.SkippedCount+state.FailedCount) / float64(state.TotalMessages) * 100
		fmt.Printf("Progress: %.2f%%\n", progress)
	}

	fmt.Println()
	fmt.Printf("Users Discovered: %d\n", len(state.Users))
	fmt.Printf("Errors Recorded: %d\n", len(state.Errors))

	return nil
}

func runReset(cmd *cobra.Command, args []string) error {
	stateMgr := state.NewManager(exportPath)

	if err := stateMgr.Delete(); err != nil {
		return fmt.Errorf("deleting state file: %w", err)
	}

	fmt.Println("Import state has been reset.")
	return nil
}
