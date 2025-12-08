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

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"

	"beef-briefing/apps/admin-panel/internal/auth"
	"beef-briefing/apps/admin-panel/internal/config"
	"beef-briefing/apps/admin-panel/internal/handler"
	"beef-briefing/apps/admin-panel/internal/middleware"
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

	slog.Info("starting admin panel",
		"port", cfg.AdminPanelPort,
		"environment", cfg.Environment,
	)

	// Connect to database
	db, err := setupDatabase(cfg)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	slog.Info("connected to database",
		"host", cfg.DBHost,
		"database", cfg.DBName,
	)

	// Initialize components
	authManager, err := auth.NewAuth(cfg)
	if err != nil {
		slog.Error("failed to create auth manager", "error", err)
		os.Exit(1)
	}
	rateLimiter := middleware.NewRateLimiter()
	h := handler.NewHandler(authManager, db)

	// Setup router
	router := setupRouter(h, authManager, rateLimiter)

	// Create server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.AdminPanelPort),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		slog.Info("server listening", "addr", server.Addr)
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

	slog.Info("server stopped gracefully")
}

func setupLogger(cfg *config.Config) {
	var handler slog.Handler

	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	opts := &slog.HandlerOptions{Level: level}

	if cfg.IsProduction() {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}

func setupDatabase(cfg *config.Config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return db, nil
}

func setupRouter(h *handler.Handler, authManager *auth.Auth, rateLimiter *middleware.RateLimiter) *mux.Router {
	r := mux.NewRouter()

	// Static files
	staticFS := http.FileServer(http.Dir("./static"))
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", staticFS))

	// Public routes
	r.HandleFunc("/auth/login", h.LoginPage).Methods("GET")
	r.Handle("/auth/login", rateLimiter.Limit(http.HandlerFunc(h.Login))).Methods("POST")

	// Protected routes
	protected := r.PathPrefix("/").Subrouter()
	protected.Use(authManager.RequireAuth)

	protected.HandleFunc("/", h.Dashboard).Methods("GET")
	protected.HandleFunc("/chats/{id:[0-9-]+}", h.ChatDetail).Methods("GET")
	protected.HandleFunc("/chats/{id:[0-9-]+}/stats-partial", h.StatsPartial).Methods("GET")
	protected.HandleFunc("/chats/{id:[0-9-]+}/calendar-data", h.CalendarData).Methods("GET")
	protected.HandleFunc("/users/{id:[0-9]+}", h.UserDetail).Methods("GET")
	protected.HandleFunc("/users/{id:[0-9]+}/messages", h.UserMessagesPartial).Methods("GET")
	protected.HandleFunc("/auth/logout", h.Logout).Methods("POST")
	protected.HandleFunc("/theme", h.SetTheme).Methods("POST")

	return r
}
