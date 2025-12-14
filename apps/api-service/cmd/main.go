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
	ingestService := services.NewIngestService(db, minioClient, nrApp)
	ingestHandler := handlers.NewIngestHandler(ingestService, cfg)

	profilePhotoService := services.NewProfilePhotoService(db, minioClient, nrApp)
	profilePhotoHandler := handlers.NewProfilePhotoHandler(profilePhotoService, cfg)

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

	slog.Info("all API endpoints require authentication", "path_prefix", "/api/v1")

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
