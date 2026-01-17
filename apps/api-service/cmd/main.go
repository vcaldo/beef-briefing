package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // Embed timezone database for Alpine containers

	"beef-briefing/apps/api-service/internal/handlers"
	"beef-briefing/apps/api-service/internal/middleware"
	"beef-briefing/apps/api-service/internal/migrations"
	"beef-briefing/apps/api-service/internal/services"
	"beef-briefing/apps/api-service/internal/storage"
	"beef-briefing/pkg/config"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/newrelic/go-agent/v3/integrations/nrgorilla"
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

	slog.Info("starting api-service",
		"environment", cfg.Environment,
		"api_port", cfg.APIPort,
	)

	// Initialize New Relic APM (optional - continues without if not configured)
	var nrApp *newrelic.Application
	if cfg.NewRelicEnabled() {
		// Build app name: {base-name}-{service}-{environment}
		// e.g., "beef-briefing-api-service-production"
		const serviceName = "api-service"
		appName := fmt.Sprintf("%s-%s-%s", cfg.NewRelicAppName, serviceName, cfg.Environment)

		nrApp, err = newrelic.NewApplication(
			newrelic.ConfigAppName(appName),
			newrelic.ConfigLicense(cfg.NewRelicLicenseKey),
			newrelic.ConfigDistributedTracerEnabled(true),
			newrelic.ConfigAppLogForwardingEnabled(true),
		)
		if err != nil {
			slog.Warn("failed to initialize New Relic, continuing without instrumentation", "error", err)
			nrApp = nil
		} else {
			slog.Info("New Relic APM initialized", "app_name", appName)
			// Allow time for New Relic to initialize
			time.Sleep(250 * time.Millisecond)
		}
	} else {
		slog.Debug("New Relic APM not configured, skipping initialization")
	}

	// Initialize database connection
	db, err := initDatabase(cfg)
	if err != nil {
		slog.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Run database migrations
	migrationCtx, migrationCancel := context.WithTimeout(context.Background(), 60*time.Second)
	if err := migrations.Run(migrationCtx, db); err != nil {
		slog.Error("failed to run database migrations", "error", err)
		os.Exit(1)
	}
	migrationCancel()

	// Initialize MinIO client with New Relic instrumentation
	minioClient, err := storage.NewMinIOClient(
		cfg.MinIOEndpoint,
		cfg.MinIOAccessKey,
		cfg.MinIOSecretKey,
		cfg.MinIOBucket,
		cfg.MinIORegion,
		cfg.MinIOUseSSL,
		nrApp,
	)
	if err != nil {
		slog.Error("failed to initialize MinIO client", "error", err)
		os.Exit(1)
	}

	// Setup HTTP router
	router := setupRouter(db, minioClient, cfg, nrApp)

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.APIPort),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		slog.Info("server starting", "port", cfg.APIPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")

	// Give outstanding requests 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	// Graceful shutdown - flush New Relic data
	if nrApp != nil {
		slog.Info("flushing New Relic data...")
		nrApp.Shutdown(5 * time.Second)
	}

	slog.Info("server stopped gracefully")
}

func setupLogger(cfg *config.Config) {
	var handler slog.Handler

	if cfg.IsProduction() {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	}

	slog.SetDefault(slog.New(handler))
}

func initDatabase(cfg *config.Config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	slog.Info("database connection established")
	return db, nil
}

func setupRouter(db *sql.DB, minioClient *storage.MinIOClient, cfg *config.Config, nrApp *newrelic.Application) *mux.Router {
	router := mux.NewRouter()

	// Health check endpoint - MUST be registered BEFORE auth middleware (unauthenticated)
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// Add New Relic middleware for automatic HTTP transaction instrumentation
	if nrApp != nil {
		router.Use(nrgorilla.Middleware(nrApp))
	}

	// Load app keys for authentication
	appKeys, err := config.LoadAppKeysFromDir(cfg.AppKeysDir)
	if err != nil {
		slog.Error("failed to load app keys", "error", err, "dir", cfg.AppKeysDir)
		os.Exit(1)
	}

	if len(appKeys) == 0 {
		slog.Error("no app keys configured - all API endpoints will be inaccessible",
			"app_keys_dir", cfg.AppKeysDir)
		os.Exit(1)
	}

	slog.Info("app keys loaded", "count", len(appKeys), "apps", getAppNames(appKeys))

	// Create multi-key auth middleware
	multiKeyAuth := middleware.NewMultiAPIKeyAuth(appKeys)

	// Create services and handlers with New Relic instrumentation
	ingestService := services.NewIngestService(db, minioClient, nrApp, nil)
	ingestHandler := handlers.NewIngestHandler(ingestService, cfg)

	profilePhotoService := services.NewProfilePhotoService(db, minioClient, nrApp)
	profilePhotoHandler := handlers.NewProfilePhotoHandler(profilePhotoService, cfg)

	mlService := services.NewMLService(db, nrApp, nil)
	mlHandler := handlers.NewMLHandler(mlService, cfg)

	cardService := services.NewCardService(db, minioClient, nrApp, nil)
	cardHandler := handlers.NewCardHandler(cardService, cfg)

	// Chat handler
	chatHandler := handlers.NewChatHandler(db)

	// Mini App service and handler (optional - only if configured)
	var miniAppHandler *handlers.MiniAppHandler
	var arenaHandler *handlers.ArenaHandler
	var jwtAuth *middleware.JWTAuth
	if cfg.MiniAppEnabled() {
		miniAppService := services.NewMiniAppService(db, cfg.JWTSecretKey, cfg.TelegramBotToken, nrApp, minioClient, cfg, nil)
		miniAppHandler = handlers.NewMiniAppHandler(miniAppService, cardService, cfg)
		jwtAuth = middleware.NewJWTAuth(cfg.JWTSecretKey)

		// Arena game service and handler
		arenaService := services.NewArenaService(db, minioClient, cardService, nrApp, nil)
		arenaHandler = handlers.NewArenaHandler(arenaService, cfg)

		slog.Info("Mini App endpoints enabled")
	} else {
		slog.Info("Mini App endpoints disabled (JWT_SECRET_KEY not configured)")
	}

	// CORS middleware for Mini App (if configured)
	var corsMiddleware *middleware.CORS
	if origins := cfg.GetCORSOrigins(); len(origins) > 0 {
		corsMiddleware = middleware.NewCORS(origins)
		slog.Info("CORS enabled", "origins", origins)
	}

	// API v1 routes - ALL AUTHENTICATED
	api := router.PathPrefix("/api/v1").Subrouter()
	api.Use(multiKeyAuth.Authenticate)

	// Ingest routes
	api.HandleFunc("/ingest", ingestHandler.HandleIngest).Methods("POST")

	// Profile photo routes
	api.HandleFunc("/profile-photos/user", profilePhotoHandler.HandleUserPhotos).Methods("POST")
	api.HandleFunc("/profile-photos/chat", profilePhotoHandler.HandleChatPhotos).Methods("POST")
	api.HandleFunc("/users", profilePhotoHandler.HandleGetUsers).Methods("GET")
	api.HandleFunc("/chats", profilePhotoHandler.HandleGetChats).Methods("GET")
	api.HandleFunc("/users/{id}/photo", profilePhotoHandler.HandleGetUserPhoto).Methods("GET")
	api.HandleFunc("/chats/{id}/photo", profilePhotoHandler.HandleGetChatPhoto).Methods("GET")

	// ML analytics routes
	api.HandleFunc("/ml/messages", mlHandler.HandleGetMessages).Methods("GET")
	api.HandleFunc("/ml/results", mlHandler.HandlePostResults).Methods("POST")
	api.HandleFunc("/ml/status", mlHandler.HandleGetStatus).Methods("GET")

	// User card routes (order matters: /cards/weeks before /cards/{user_id})
	api.HandleFunc("/cards", cardHandler.HandleGetChatCards).Methods("GET")
	api.HandleFunc("/cards/weeks", cardHandler.HandleGetAvailableWeeks).Methods("GET")
	api.HandleFunc("/cards/{user_id}", cardHandler.HandleGetUserCard).Methods("GET")
	api.HandleFunc("/cards/{user_id}/history", cardHandler.HandleGetUserHistory).Methods("GET")
	api.HandleFunc("/cards/{user_id}/image", cardHandler.HandleGetCardImage).Methods("GET")

	// Chat configuration routes
	api.HandleFunc("/chat/{chat_id}", chatHandler.GetChatInfo).Methods("GET")

	// Arena bot endpoints (API key authenticated, used by telegram-bot)
	if arenaHandler != nil {
		api.HandleFunc("/arena/match", arenaHandler.HandleBotCreateMatch).Methods("POST")
		api.HandleFunc("/arena/match/{id}", arenaHandler.HandleBotGetMatch).Methods("GET")
		api.HandleFunc("/arena/match/{id}/join", arenaHandler.HandleBotJoinMatch).Methods("POST")
		api.HandleFunc("/arena/match/{id}/leave", arenaHandler.HandleBotLeaveMatch).Methods("POST")
		api.HandleFunc("/arena/match/{id}/start", arenaHandler.HandleBotStartMatch).Methods("POST")
		api.HandleFunc("/arena/matches/pending", arenaHandler.HandleBotGetPendingMatches).Methods("GET")
		api.HandleFunc("/arena/match/{id}/auto-start", arenaHandler.HandleBotAutoStartMatch).Methods("POST")
		api.HandleFunc("/arena/match/{id}/force-submit", arenaHandler.HandleBotForceSubmitTeams).Methods("POST")
		api.HandleFunc("/arena/match/{id}/share-data", arenaHandler.HandleBotGetShareData).Methods("GET")

		// Ranked tournament bot endpoints
		api.HandleFunc("/arena/tournament/today", arenaHandler.HandleBotGetTodayTournament).Methods("GET")
		api.HandleFunc("/arena/tournament/{id}", arenaHandler.HandleBotGetTournament).Methods("GET")
		api.HandleFunc("/arena/tournaments/pending-announcements", arenaHandler.HandleBotGetPendingAnnouncements).Methods("GET")
		api.HandleFunc("/arena/tournament/announce", arenaHandler.HandleBotAnnounceTournament).Methods("POST")
		api.HandleFunc("/arena/tournament/join", arenaHandler.HandleBotJoinTournament).Methods("POST")
		api.HandleFunc("/arena/tournament/leave", arenaHandler.HandleBotLeaveTournament).Methods("POST")
		api.HandleFunc("/arena/tournaments/pending-close", arenaHandler.HandleBotGetPendingClose).Methods("GET")
		api.HandleFunc("/arena/tournament/{id}/close", arenaHandler.HandleBotCloseTournament).Methods("POST")
		api.HandleFunc("/arena/tournaments/pending-rounds", arenaHandler.HandleBotGetPendingRounds).Methods("GET")
		slog.Info("Arena bot endpoints registered", "path_prefix", "/api/v1/arena")
	}

	slog.Info("all API endpoints require authentication", "path_prefix", "/api/v1")

	// Mini App routes (JWT authenticated, CORS enabled)
	if miniAppHandler != nil {
		miniApp := router.PathPrefix("/api/v1/mini-app").Subrouter()

		// Apply CORS middleware if configured
		if corsMiddleware != nil {
			miniApp.Use(corsMiddleware.Handler)
		}

		// Auth endpoint - unauthenticated (validates Telegram init data)
		miniApp.HandleFunc("/auth", miniAppHandler.HandleAuth).Methods("POST", "OPTIONS")

		// Protected endpoints - require JWT
		protected := miniApp.PathPrefix("").Subrouter()
		protected.Use(jwtAuth.Authenticate)
		protected.HandleFunc("/stats", miniAppHandler.HandleStats).Methods("GET", "OPTIONS")
		protected.HandleFunc("/activity", miniAppHandler.HandleActivity).Methods("GET", "OPTIONS")
		protected.HandleFunc("/leaderboard", miniAppHandler.HandleLeaderboard).Methods("GET", "OPTIONS")
		protected.HandleFunc("/reactions-overview", miniAppHandler.HandleReactionsOverview).Methods("GET", "OPTIONS")
		protected.HandleFunc("/replies-overview", miniAppHandler.HandleRepliesOverview).Methods("GET", "OPTIONS")
		protected.HandleFunc("/profile", miniAppHandler.HandleProfile).Methods("GET", "OPTIONS")
		protected.HandleFunc("/heatmap", miniAppHandler.HandleHeatmap).Methods("GET", "OPTIONS")
		protected.HandleFunc("/users", miniAppHandler.HandleUsers).Methods("GET", "OPTIONS")
		protected.HandleFunc("/media-overview", miniAppHandler.HandleMediaOverview).Methods("GET", "OPTIONS")

		// Settings endpoints - admin configuration
		protected.HandleFunc("/settings/timezone", miniAppHandler.HandleGetTimezone).Methods("GET", "OPTIONS")
		protected.HandleFunc("/settings/timezone", miniAppHandler.HandleSetTimezone).Methods("PUT", "OPTIONS")

		// Gallery endpoints - card images for deck-mini-app
		protected.HandleFunc("/gallery/weeks", miniAppHandler.HandleGalleryWeeks).Methods("GET", "OPTIONS")
		protected.HandleFunc("/gallery/images", miniAppHandler.HandleGalleryImages).Methods("GET", "OPTIONS")
		protected.HandleFunc("/gallery/image/{id:[0-9]+}", miniAppHandler.HandleGalleryImageURL).Methods("GET", "OPTIONS")

		// Arena game endpoints
		if arenaHandler != nil {
			protected.HandleFunc("/arena/constants", arenaHandler.HandleGetConstants).Methods("GET", "OPTIONS")
			protected.HandleFunc("/arena/matches", arenaHandler.HandleListMatches).Methods("GET", "OPTIONS")
			protected.HandleFunc("/arena/match", arenaHandler.HandleCreateMatch).Methods("POST", "OPTIONS")
			protected.HandleFunc("/arena/match/{id}", arenaHandler.HandleGetMatch).Methods("GET", "OPTIONS")
			protected.HandleFunc("/arena/match/{id}/join", arenaHandler.HandleJoinMatch).Methods("POST", "OPTIONS")
			protected.HandleFunc("/arena/match/{id}/leave", arenaHandler.HandleLeaveMatch).Methods("POST", "OPTIONS")
			protected.HandleFunc("/arena/match/{id}/start", arenaHandler.HandleStartMatch).Methods("POST", "OPTIONS")
			protected.HandleFunc("/arena/match/{id}/shop", arenaHandler.HandleGetShop).Methods("GET", "OPTIONS")
			protected.HandleFunc("/arena/match/{id}/buy", arenaHandler.HandleBuyCard).Methods("POST", "OPTIONS")
			protected.HandleFunc("/arena/match/{id}/reroll", arenaHandler.HandleReroll).Methods("POST", "OPTIONS")
			protected.HandleFunc("/arena/match/{id}/upgrade", arenaHandler.HandleUpgrade).Methods("POST", "OPTIONS")
			protected.HandleFunc("/arena/match/{id}/order", arenaHandler.HandleSetOrder).Methods("POST", "OPTIONS")
			protected.HandleFunc("/arena/match/{id}/team", arenaHandler.HandleSubmitTeam).Methods("POST", "OPTIONS")
			protected.HandleFunc("/arena/match/{id}/battle", arenaHandler.HandleGetBattle).Methods("GET", "OPTIONS")
			protected.HandleFunc("/arena/match/{id}/share", arenaHandler.HandleShareResult).Methods("POST", "OPTIONS")
			protected.HandleFunc("/arena/leaderboard", arenaHandler.HandleGetLeaderboard).Methods("GET", "OPTIONS")
			protected.HandleFunc("/arena/history", arenaHandler.HandleGetHistory).Methods("GET", "OPTIONS")
			protected.HandleFunc("/arena/h2h", arenaHandler.HandleGetH2H).Methods("GET", "OPTIONS")
			protected.HandleFunc("/arena/profile", arenaHandler.HandleGetProfile).Methods("GET", "OPTIONS")
			slog.Info("Arena game endpoints registered")
		}

		slog.Info("Mini App endpoints registered", "path_prefix", "/api/v1/mini-app")
	}

	return router
}

// getAppNames extracts app names from the appKeys map for logging
func getAppNames(appKeys map[string]string) []string {
	names := make([]string, 0, len(appKeys))
	for name := range appKeys {
		names = append(names, name)
	}
	return names
}
