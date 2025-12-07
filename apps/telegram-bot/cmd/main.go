package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"beef-briefing/apps/telegram-bot/internal/client"
	"beef-briefing/apps/telegram-bot/internal/handlers"
	"beef-briefing/pkg/config"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
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

	slog.Info("starting telegram-bot",
		"environment", cfg.Environment,
		"api_service_url", cfg.APIServiceURL,
	)

	// Create context for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Initialize API client
	apiClient := client.NewAPIClient(cfg.APIServiceURL)

	// Initialize update handler once (reused for all updates)
	updateHandler := handlers.NewUpdateHandler(apiClient)

	// Create bot instance with allowed updates including reactions
	opts := []bot.Option{
		bot.WithDefaultHandler(func(ctx context.Context, b *bot.Bot, update *models.Update) {
			updateHandler.Handle(ctx, b, update)
		}),
		bot.WithAllowedUpdates(bot.AllowedUpdates{
			"message",
			"edited_message",
			"channel_post",
			"edited_channel_post",
			"message_reaction",
			"message_reaction_count",
			"inline_query",
			"chosen_inline_result",
			"callback_query",
			"shipping_query",
			"pre_checkout_query",
			"poll",
			"poll_answer",
			"my_chat_member",
			"chat_member",
			"chat_join_request",
			"chat_boost",
			"removed_chat_boost",
		}),
	}

	b, err := bot.New(cfg.TelegramBotToken, opts...)
	if err != nil {
		slog.Error("failed to create bot", "error", err)
		os.Exit(1)
	}

	slog.Info("bot initialized successfully, starting long polling...")

	// Start bot with graceful shutdown
	b.Start(ctx)

	slog.Info("bot stopped gracefully")
}

func setupLogger(cfg *config.Config) {
	var handler slog.Handler

	level := parseLogLevel(cfg.LogLevel)

	if cfg.IsProduction() {
		// Production: JSON handler
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	} else {
		// Development: Text handler
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	}

	slog.SetDefault(slog.New(handler))
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
