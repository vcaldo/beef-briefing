package handler

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"

	"beef-briefing/apps/admin-panel/internal/auth"
	"beef-briefing/apps/admin-panel/internal/repository"
	"beef-briefing/apps/admin-panel/templates"
)

// Handler handles all HTTP requests
type Handler struct {
	auth     *auth.Auth
	chatRepo *repository.ChatRepository
}

// NewHandler creates a new Handler
func NewHandler(a *auth.Auth, db *sql.DB) *Handler {
	return &Handler{
		auth:     a,
		chatRepo: repository.NewChatRepository(db),
	}
}

// LoginPage renders the login form
func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	// If already authenticated, redirect to dashboard
	if h.auth.IsAuthenticated(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if err := templates.Login("").Render(r.Context(), w); err != nil {
		slog.Error("failed to render login page", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Login handles the login form submission
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		slog.Error("failed to parse login form", "error", err)
		renderLoginError(w, r, "Invalid form data")
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	// Validate input
	if username == "" || password == "" {
		renderLoginError(w, r, "Username and password are required")
		return
	}

	if len(username) > 255 {
		renderLoginError(w, r, "Invalid username")
		return
	}

	// Verify credentials
	if !h.auth.VerifyCredentials(username, password) {
		slog.Warn("failed login attempt", "username", username)
		renderLoginError(w, r, "Invalid username or password")
		return
	}

	// Create session
	if err := h.auth.CreateSession(w, r); err != nil {
		slog.Error("failed to create session", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	slog.Info("user logged in", "username", username)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Logout destroys the session and redirects to login
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.auth.DestroySession(w, r); err != nil {
		slog.Error("failed to destroy session", "error", err)
	}

	slog.Info("user logged out")
	http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
}

// Dashboard renders the main dashboard with chat list
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	theme := h.auth.GetTheme(r)

	chats, err := h.chatRepo.ListChats(ctx)
	if err != nil {
		slog.Error("failed to get chats", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Convert repository chats to template chats
	templateChats := make([]templates.Chat, len(chats))
	for i, c := range chats {
		templateChats[i] = templates.Chat{
			ID:           c.ID,
			Title:        c.Title,
			Type:         c.Type,
			Username:     c.Username,
			MessageCount: c.MessageCount,
			UserCount:    c.UserCount,
			LastActivity: c.LastActivity,
		}
	}

	if err := templates.Dashboard(templateChats, theme).Render(ctx, w); err != nil {
		slog.Error("failed to render dashboard", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// ChatDetail renders the chat detail page
func (h *Handler) ChatDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	theme := h.auth.GetTheme(r)

	vars := mux.Vars(r)
	chatID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid chat ID", http.StatusBadRequest)
		return
	}

	// Get chat details
	chat, err := h.chatRepo.GetChat(ctx, chatID)
	if err != nil {
		slog.Error("failed to get chat", "error", err, "chat_id", chatID)
		http.Error(w, "Chat not found", http.StatusNotFound)
		return
	}

	// Get available years for filters
	years, err := h.chatRepo.GetAvailableYears(ctx, chatID)
	if err != nil {
		slog.Error("failed to get available years", "error", err, "chat_id", chatID)
		years = []int{time.Now().Year()}
	}

	// Get user stats (all time by default)
	stats, err := h.chatRepo.GetUserStats(ctx, chatID, 0, 0)
	if err != nil {
		slog.Error("failed to get user stats", "error", err, "chat_id", chatID)
		stats = []repository.UserStats{}
	}

	// Convert to template types
	templateChat := templates.ChatDetail{
		ID:           chat.ID,
		Title:        chat.Title,
		Type:         chat.Type,
		Username:     chat.Username,
		FirstName:    chat.FirstName,
		LastName:     chat.LastName,
		MessageCount: chat.MessageCount,
		UserCount:    chat.UserCount,
		MediaCount:   chat.MediaCount,
		FirstMessage: chat.FirstMessage,
		LastMessage:  chat.LastMessage,
	}

	templateStats := make([]templates.UserStats, len(stats))
	for i, s := range stats {
		templateStats[i] = templates.UserStats{
			UserID:            s.UserID,
			Username:          s.Username,
			FirstName:         s.FirstName,
			LastName:          s.LastName,
			MessageCount:      s.MessageCount,
			ReactionsGiven:    s.ReactionsGiven,
			ReactionsReceived: s.ReactionsReceived,
			MediaShared:       s.MediaShared,
			FirstActive:       s.FirstActive,
			LastActive:        s.LastActive,
		}
	}

	filter := templates.FilterParams{
		Month:          0,
		Year:           0,
		AvailableYears: years,
	}

	if err := templates.ChatDetailPage(templateChat, templateStats, filter, theme).Render(ctx, w); err != nil {
		slog.Error("failed to render chat detail", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// StatsPartial returns the stats table partial for HTMX
func (h *Handler) StatsPartial(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	chatID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid chat ID", http.StatusBadRequest)
		return
	}

	// Parse filter params
	month, _ := strconv.Atoi(r.URL.Query().Get("month"))
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))

	stats, err := h.chatRepo.GetUserStats(ctx, chatID, month, year)
	if err != nil {
		slog.Error("failed to get user stats", "error", err, "chat_id", chatID)
		stats = []repository.UserStats{}
	}

	templateStats := make([]templates.UserStats, len(stats))
	for i, s := range stats {
		templateStats[i] = templates.UserStats{
			UserID:            s.UserID,
			Username:          s.Username,
			FirstName:         s.FirstName,
			LastName:          s.LastName,
			MessageCount:      s.MessageCount,
			ReactionsGiven:    s.ReactionsGiven,
			ReactionsReceived: s.ReactionsReceived,
			MediaShared:       s.MediaShared,
			FirstActive:       s.FirstActive,
			LastActive:        s.LastActive,
		}
	}

	if err := templates.StatsTable(templateStats).Render(ctx, w); err != nil {
		slog.Error("failed to render stats table", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// CalendarData returns the calendar partial for HTMX
func (h *Handler) CalendarData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	chatID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid chat ID", http.StatusBadRequest)
		return
	}

	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	if year == 0 {
		year = time.Now().Year()
	}

	data, err := h.chatRepo.GetCalendarData(ctx, chatID, year)
	if err != nil {
		slog.Error("failed to get calendar data", "error", err, "chat_id", chatID)
		data = []repository.CalendarDay{}
	}

	templateData := make([]templates.CalendarDay, len(data))
	for i, d := range data {
		templateData[i] = templates.CalendarDay{
			Date:  d.Date,
			Count: d.Count,
		}
	}

	if err := templates.CalendarPartial(templateData, year).Render(ctx, w); err != nil {
		slog.Error("failed to render calendar", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// SetTheme updates the user's theme preference
func (h *Handler) SetTheme(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Theme string `json:"theme"`
	}

	// Try JSON body first
	if r.Header.Get("Content-Type") == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
	} else {
		// Fall back to form data
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}
		data.Theme = r.FormValue("theme")
	}

	if !auth.ValidThemes[data.Theme] {
		http.Error(w, "Invalid theme", http.StatusBadRequest)
		return
	}

	if err := h.auth.SetTheme(w, r, data.Theme); err != nil {
		slog.Error("failed to set theme", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func renderLoginError(w http.ResponseWriter, r *http.Request, msg string) {
	w.WriteHeader(http.StatusUnauthorized)
	if err := templates.Login(msg).Render(r.Context(), w); err != nil {
		slog.Error("failed to render login error", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
