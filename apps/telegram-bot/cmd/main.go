package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"beef-briefing/apps/telegram-bot/internal/client"
	"beef-briefing/apps/telegram-bot/internal/handlers"
	"beef-briefing/pkg/config"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/newrelic/go-agent/v3/newrelic"
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

	// Initialize New Relic APM (optional - continues without if fails)
	var nrApp *newrelic.Application
	if cfg.NewRelicEnabled() {
		var appName string
		nrApp, appName, err = initNewRelic(cfg)
		if err != nil {
			slog.Error("failed to initialize New Relic", "error", err)
			slog.Info("continuing without New Relic instrumentation")
		} else {
			slog.Info("New Relic APM initialized", "app_name", appName, "region", cfg.NewRelicRegion)
		}
	} else {
		slog.Debug("New Relic not configured, skipping APM initialization")
	}

	// Create context for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Initialize API client
	apiClient := client.NewAPIClient(cfg.APIServiceURL, nrApp)

	// Initialize update handler once (reused for all updates)
	updateHandler := handlers.NewUpdateHandler(apiClient, nrApp)

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

	slog.Info("bot stopped, shutting down...")

	// Graceful shutdown - flush New Relic data
	if nrApp != nil {
		slog.Info("flushing New Relic data...")
		nrApp.Shutdown(5 * time.Second)
	}

	slog.Info("telegram-bot stopped gracefully")
}

// initNewRelic initializes the New Relic application with the given configuration
func initNewRelic(cfg *config.Config) (*newrelic.Application, string, error) {
	const serviceName = "telegram-bot"

	// Build app name following pattern: {base}-{service}-{environment}
	appName := fmt.Sprintf("%s-%s-%s", cfg.NewRelicAppName, serviceName, cfg.Environment)

	opts := []newrelic.ConfigOption{
		newrelic.ConfigAppName(appName),
		newrelic.ConfigLicense(cfg.NewRelicLicenseKey),
		newrelic.ConfigDistributedTracerEnabled(true),
		newrelic.ConfigAppLogForwardingEnabled(true),
	}

	// Configure EU region endpoint if specified
	// Note: The agent auto-detects region from license key prefix (eu01),
	// but we allow explicit override via config
	if cfg.NewRelicRegion == "EU" {
		opts = append(opts, func(c *newrelic.Config) {
			c.Host = "collector.eu.newrelic.com"
		})
	}

	nrApp, err := newrelic.NewApplication(opts...)
	if err != nil {
		return nil, "", err
	}

	// Wait for connection to New Relic
	if err := nrApp.WaitForConnection(5 * time.Second); err != nil {
		slog.Warn("New Relic connection timeout", "error", err)
		// Continue anyway - data will be sent when connection establishes
	}

	return nrApp, appName, nil
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
