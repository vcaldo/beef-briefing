package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"

	"beef-briefing/apps/admin-panel/internal/apiclient"
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

	// Initialize API client
	apiClient := apiclient.NewClient(apiclient.Config{
		BaseURL: cfg.APIServiceURL,
		APIKey:  cfg.AnalyticsAPIKey,
		Timeout: 30 * time.Second,
	})

	slog.Info("configured API client",
		"api_url", cfg.APIServiceURL,
	)

	// Initialize components
	authManager, err := auth.NewAuth(cfg)
	if err != nil {
		slog.Error("failed to create auth manager", "error", err)
		os.Exit(1)
	}
	rateLimiter := middleware.NewRateLimiter()
	h := handler.NewHandler(authManager, apiClient)

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

	// Dashboard and chat views
	protected.HandleFunc("/", h.Dashboard).Methods("GET")
	protected.HandleFunc("/chats/{id:[0-9-]+}", h.ChatDetail).Methods("GET")

	// Analytics partials (HTMX endpoints)
	protected.HandleFunc("/chats/{id:[0-9-]+}/leaderboard", h.LeaderboardPartial).Methods("GET")
	protected.HandleFunc("/chats/{id:[0-9-]+}/heatmap", h.HeatmapPartial).Methods("GET")
	protected.HandleFunc("/chats/{id:[0-9-]+}/timeline", h.TimelinePartial).Methods("GET")
	protected.HandleFunc("/chats/{id:[0-9-]+}/top-content", h.TopContentPartial).Methods("GET")
	protected.HandleFunc("/chats/{id:[0-9-]+}/users/{user_id:[0-9]+}", h.UserDetailPartial).Methods("GET")
	protected.HandleFunc("/chats/{id:[0-9-]+}/compare", h.CompareUsersPartial).Methods("GET")

	// Auth and settings
	protected.HandleFunc("/auth/logout", h.Logout).Methods("POST")
	protected.HandleFunc("/theme", h.SetTheme).Methods("POST")

	return r
}
