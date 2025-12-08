package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"

	"beef-briefing/apps/legacy-export-generator/internal/repository"
	"beef-briefing/apps/legacy-export-generator/internal/transformer"
)

var (
	// Version information (set at build time)
	version = "dev"

	// Database flags
	dbHost     string
	dbPort     int
	dbUser     string
	dbPassword string
	dbName     string

	// Filter flags
	startDate    string
	endDate      string
	sourceChatID int64

	// Output flags
	chatName string
	chatType string
	chatID   int64
	output   string

	// Logging flags
	verbose bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:     "legacy-export-generator",
	Short:   "Export messages from legacy PostgreSQL to Telegram JSON format",
	Long:    `A CLI tool to export messages from a legacy PostgreSQL database to Telegram-compatible result.json format.`,
	Version: version,
	RunE:    runExport,
}

func init() {
	// Database connection flags
	rootCmd.Flags().StringVar(&dbHost, "db-host", "localhost", "Database host")
	rootCmd.Flags().IntVar(&dbPort, "db-port", 5432, "Database port")
	rootCmd.Flags().StringVar(&dbUser, "db-user", "", "Database username")
	rootCmd.Flags().StringVar(&dbPassword, "db-password", "", "Database password")
	rootCmd.Flags().StringVar(&dbName, "db-name", "", "Database name")

	// Date range flags
	rootCmd.Flags().StringVar(&startDate, "start-date", "", "Start date for export (format: YYYY-MM-DD)")
	rootCmd.Flags().StringVar(&endDate, "end-date", "", "End date for export (format: YYYY-MM-DD)")

	// Source filter flags
	rootCmd.Flags().Int64Var(&sourceChatID, "source-chat-id", 0, "Filter by source chat ID (optional)")

	// Output metadata flags
	rootCmd.Flags().StringVar(&chatName, "chat-name", "", "Chat name for export metadata")
	rootCmd.Flags().StringVar(&chatType, "chat-type", "private_supergroup", "Chat type for export metadata")
	rootCmd.Flags().Int64Var(&chatID, "chat-id", 0, "Chat ID for export metadata")

	// Output file flag
	rootCmd.Flags().StringVarP(&output, "output", "o", "result.json", "Output file path")

	// Logging flag
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")

	// Mark required flags
	rootCmd.MarkFlagRequired("db-user")
	rootCmd.MarkFlagRequired("db-password")
	rootCmd.MarkFlagRequired("db-name")
	rootCmd.MarkFlagRequired("start-date")
	rootCmd.MarkFlagRequired("end-date")
	rootCmd.MarkFlagRequired("chat-name")
	rootCmd.MarkFlagRequired("chat-id")
}

func runExport(cmd *cobra.Command, args []string) error {
	setupLogging()

	ctx := context.Background()

	// Parse dates
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return fmt.Errorf("parsing start-date: %w", err)
	}

	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return fmt.Errorf("parsing end-date: %w", err)
	}
	// Set end date to end of day
	end = end.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	slog.Info("connecting to database",
		"host", dbHost,
		"port", dbPort,
		"database", dbName,
		"user", dbUser,
	)

	// Connect to database
	repo, err := repository.New(repository.Config{
		Host:     dbHost,
		Port:     dbPort,
		User:     dbUser,
		Password: dbPassword,
		DBName:   dbName,
	})
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer repo.Close()

	slog.Info("querying messages",
		"start_date", start.Format("2006-01-02"),
		"end_date", end.Format("2006-01-02"),
		"source_chat_id", sourceChatID,
	)

	// Query messages
	var sourceChatIDPtr *int64
	if sourceChatID != 0 {
		sourceChatIDPtr = &sourceChatID
	}

	messages, err := repo.Query(ctx, start, end, sourceChatIDPtr)
	if err != nil {
		return fmt.Errorf("querying messages: %w", err)
	}

	slog.Info("retrieved messages", "count", len(messages))

	if len(messages) == 0 {
		slog.Warn("no messages found in date range")
	}

	// Transform messages
	slog.Info("transforming messages to export format")
	trans := transformer.New(chatName, chatType, chatID)
	exportData, err := trans.Transform(messages)
	if err != nil {
		return fmt.Errorf("transforming messages: %w", err)
	}

	// Write output
	slog.Info("writing output", "path", output)
	if err := writeJSON(output, exportData); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	slog.Info("export completed successfully",
		"messages", len(exportData.Messages),
		"output", output,
	)

	return nil
}

func writeJSON(path string, data any) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", " ")
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}

	return nil
}

func setupLogging() {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(handler))
}
