package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"beef-briefing/apps/telegram-bot/internal/config"
	"beef-briefing/apps/telegram-bot/internal/handler"
	"beef-briefing/apps/telegram-bot/internal/storage"
	"beef-briefing/apps/telegram-bot/internal/store"

	tele "gopkg.in/telebot.v4"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Setup logger
	setupLogger(cfg)

	slog.Info("starting telegram bot",
		"environment", cfg.Environment,
		"log_level", cfg.LogLevel)

	// Initialize database store
	dbStore, err := store.NewPostgresStore(cfg.DSN())
	if err != nil {
		slog.Error("failed to create database store", "error", err)
		os.Exit(1)
	}
	defer dbStore.Close()
	slog.Info("database connection established")

	// Initialize MinIO storage
	minioClient, err := storage.NewMinIOClient(
		cfg.MinIOEndpoint,
		cfg.MinIOAccessKey,
		cfg.MinIOSecretKey,
		cfg.MinIOBucket,
		cfg.MinIOUseSSL,
	)
	if err != nil {
		slog.Error("failed to create MinIO client", "error", err)
		os.Exit(1)
	}
	slog.Info("MinIO client initialized", "bucket", cfg.MinIOBucket)

	// Create bot (needed for file downloads)
	pref := tele.Settings{
		Token:  cfg.TelegramBotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := tele.NewBot(pref)
	if err != nil {
		slog.Error("failed to create bot", "error", err)
		os.Exit(1)
	}

	slog.Info("bot created successfully")

	// Initialize handler with MinIO client, bot, and config
	h := handler.NewHandler(dbStore, minioClient, bot, cfg)

	// Register handlers
	bot.Handle(tele.OnText, h.HandleMessage)
	bot.Handle(tele.OnPhoto, h.HandleMessage)
	bot.Handle(tele.OnVideo, h.HandleMessage)
	bot.Handle(tele.OnVoice, h.HandleMessage)
	bot.Handle(tele.OnDocument, h.HandleMessage)
	bot.Handle(tele.OnSticker, h.HandleMessage)
	bot.Handle(tele.OnAnimation, h.HandleMessage)
	bot.Handle(tele.OnVideoNote, h.HandleMessage)
	bot.Handle(tele.OnLocation, h.HandleMessage)
	bot.Handle(tele.OnVenue, h.HandleMessage)
	bot.Handle(tele.OnUserJoined, h.HandleUserJoined)
	bot.Handle(tele.OnUserLeft, h.HandleUserLeft)

	// Register import command
	bot.Handle("/import", h.HandleImportCommand)

	slog.Info("handlers registered")

	// Start bot in goroutine
	go func() {
		slog.Info("bot starting to poll for updates")
		bot.Start()
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down bot...")

	// Stop bot
	bot.Stop()

	slog.Info("bot stopped gracefully")
}

func setupLogger(cfg *config.Config) {
	var level slog.Level
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	var handler slog.Handler
	if cfg.IsProduction() {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}

	slog.SetDefault(slog.New(handler))
}
