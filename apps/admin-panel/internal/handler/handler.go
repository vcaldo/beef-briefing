package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/newrelic/go-agent/v3/newrelic"

	"beef-briefing/apps/admin-panel/internal/apiclient"
	"beef-briefing/apps/admin-panel/internal/auth"
	"beef-briefing/apps/admin-panel/templates"
)

// Handler handles all HTTP requests
type Handler struct {
	auth  *auth.Auth
	api   *apiclient.Client
	nrApp *newrelic.Application
}

// NewHandler creates a new Handler
func NewHandler(a *auth.Auth, api *apiclient.Client, nrApp *newrelic.Application) *Handler {
	return &Handler{
		auth:  a,
		api:   api,
		nrApp: nrApp,
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
	txn := newrelic.FromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		slog.Error("failed to parse login form", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		renderLoginError(w, r, "Invalid form data")
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	// Add username attribute for tracking (not password!)
	if txn != nil {
		txn.AddAttribute("username", username)
	}

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
		if txn != nil {
			txn.AddAttribute("login_success", false)
		}
		renderLoginError(w, r, "Invalid username or password")
		return
	}

	// Create session
	if err := h.auth.CreateSession(w, r); err != nil {
		slog.Error("failed to create session", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if txn != nil {
		txn.AddAttribute("login_success", true)
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
	txn := newrelic.FromContext(ctx)
	theme := h.auth.GetTheme(r)

	chats, err := h.api.ListChats(ctx)
	if err != nil {
		slog.Error("failed to get chats from API", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		http.Error(w, "Failed to load chats", http.StatusInternalServerError)
		return
	}

	// Add custom attributes
	if txn != nil {
		txn.AddAttribute("chat_count", len(chats))
		txn.AddAttribute("theme", theme)
	}

	// Convert API chats to template chats
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
		if txn != nil {
			txn.NoticeError(err)
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// ChatDetail renders the chat detail page
func (h *Handler) ChatDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)
	theme := h.auth.GetTheme(r)

	vars := mux.Vars(r)
	chatID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid chat ID", http.StatusBadRequest)
		return
	}

	// Add custom attributes for tracking
	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("theme", theme)
	}

	// Parse filter parameters
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	if year == 0 {
		year = time.Now().Year()
	}

	month, _ := strconv.Atoi(r.URL.Query().Get("month"))
	if month < 0 || month > 12 {
		month = 0 // 0 = all months
	}

	timezone := r.URL.Query().Get("tz")
	if timezone == "" {
		timezone = "UTC"
	}

	// Add filter attributes
	if txn != nil {
		txn.AddAttribute("filter_year", year)
		txn.AddAttribute("filter_month", month)
		txn.AddAttribute("filter_timezone", timezone)
	}

	// Calculate date range based on year and month
	var startDate, endDate time.Time
	if month == 0 {
		// Full year
		startDate = time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		endDate = time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC)
		if year == time.Now().Year() {
			// For current year, use now as end date
			endDate = time.Now().Add(24 * time.Hour)
		}
	} else {
		// Specific month
		startDate = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		endDate = time.Date(year, time.Month(month)+1, 1, 0, 0, 0, 0, time.UTC)
		// Handle December -> January transition
		if month == 12 {
			endDate = time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		// For current month, cap at now
		now := time.Now()
		if year == now.Year() && month == int(now.Month()) {
			endDate = now.Add(24 * time.Hour)
		}
	}

	// Get chat details
	chat, err := h.api.GetChat(ctx, chatID)
	if err != nil {
		slog.Error("failed to get chat from API", "error", err, "chat_id", chatID)
		if txn != nil {
			txn.NoticeError(err)
		}
		http.Error(w, "Chat not found", http.StatusNotFound)
		return
	}

	// Get overview for the selected year
	overview, err := h.api.GetOverview(ctx, chatID, startDate, endDate)
	if err != nil {
		slog.Error("failed to get overview from API", "error", err, "chat_id", chatID)
		overview = &apiclient.OverviewResponse{}
	}

	// Get leaderboard for the selected year
	leaderboard, err := h.api.GetLeaderboard(ctx, chatID, startDate, endDate, "messages", 50)
	if err != nil {
		slog.Error("failed to get leaderboard from API", "error", err, "chat_id", chatID)
		leaderboard = []apiclient.LeaderboardEntry{}
	}

	// Get heatmap for the selected year
	heatmap, err := h.api.GetHeatmap(ctx, chatID, startDate, endDate)
	if err != nil {
		slog.Error("failed to get heatmap from API", "error", err, "chat_id", chatID)
		heatmap = []apiclient.HeatmapDay{}
	}

	// Calculate available years from chat first/last message
	years := calculateAvailableYears(chat.FirstMessage, chat.LastMessage)

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

	templateOverview := templates.Overview{
		TotalMessages:  overview.TotalMessages,
		UniqueUsers:    overview.TotalUsers,
		TotalReactions: overview.TotalReactions,
		TotalMedia:     overview.TotalMedia,
		MessagesPerDay: overview.MessagesPerDay,
	}
	if overview.MostActiveUser != nil {
		templateOverview.MostActiveUser = &templates.UserSummary{
			UserID:       overview.MostActiveUser.UserID,
			Username:     overview.MostActiveUser.Username,
			FirstName:    overview.MostActiveUser.FirstName,
			LastName:     overview.MostActiveUser.LastName,
			MessageCount: overview.MostActiveUser.MessageCount,
		}
	}
	templateOverview.TopEmojis = make([]templates.EmojiBreakdown, len(overview.TopEmojis))
	for i, e := range overview.TopEmojis {
		templateOverview.TopEmojis[i] = templates.EmojiBreakdown{Emoji: e.Emoji, Count: e.Count}
	}

	templateLeaderboard := make([]templates.LeaderboardEntry, len(leaderboard))
	for i, e := range leaderboard {
		templateLeaderboard[i] = templates.LeaderboardEntry{
			Rank:      e.Rank,
			UserID:    e.UserID,
			Username:  e.Username,
			FirstName: e.FirstName,
			LastName:  e.LastName,
			Score:     e.Score,
		}
	}

	templateHeatmap := make([]templates.HeatmapDay, len(heatmap))
	for i, d := range heatmap {
		templateHeatmap[i] = templates.HeatmapDay{
			Date:  d.Date,
			Count: d.Count,
		}
	}

	filter := templates.FilterParams{
		Year:           year,
		Month:          month,
		AvailableYears: years,
		Timezone:       timezone,
		StartDate:      startDate,
		EndDate:        endDate,
	}

	if err := templates.ChatDetailPage(templateChat, &templateOverview, templateLeaderboard, templateHeatmap, filter, theme).Render(ctx, w); err != nil {
		slog.Error("failed to render chat detail", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// LeaderboardPartial returns the leaderboard table partial for HTMX
func (h *Handler) LeaderboardPartial(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	vars := mux.Vars(r)
	chatID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid chat ID", http.StatusBadRequest)
		return
	}

	filter := parseFilterParams(r)

	metric := r.URL.Query().Get("metric")
	if metric == "" {
		metric = "messages"
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 50
	}

	// Add custom attributes
	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("metric", metric)
		txn.AddAttribute("limit", limit)
	}

	leaderboard, err := h.api.GetLeaderboard(ctx, chatID, filter.StartDate, filter.EndDate, metric, limit)
	if err != nil {
		slog.Error("failed to get leaderboard from API", "error", err, "chat_id", chatID)
		leaderboard = []apiclient.LeaderboardEntry{}
	}

	templateLeaderboard := make([]templates.LeaderboardEntry, len(leaderboard))
	for i, e := range leaderboard {
		templateLeaderboard[i] = templates.LeaderboardEntry{
			Rank:      e.Rank,
			UserID:    e.UserID,
			Username:  e.Username,
			FirstName: e.FirstName,
			LastName:  e.LastName,
			Score:     e.Score,
		}
	}

	if err := templates.LeaderboardTable(templateLeaderboard, chatID, filter.Year, filter.Month, filter.Timezone).Render(ctx, w); err != nil {
		slog.Error("failed to render leaderboard table", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// HeatmapPartial returns the heatmap partial for HTMX
func (h *Handler) HeatmapPartial(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	vars := mux.Vars(r)
	chatID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid chat ID", http.StatusBadRequest)
		return
	}

	filter := parseFilterParams(r)

	// Add custom attributes
	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
	}

	heatmap, err := h.api.GetHeatmap(ctx, chatID, filter.StartDate, filter.EndDate)
	if err != nil {
		slog.Error("failed to get heatmap from API", "error", err, "chat_id", chatID)
		heatmap = []apiclient.HeatmapDay{}
	}

	templateHeatmap := make([]templates.HeatmapDay, len(heatmap))
	for i, d := range heatmap {
		templateHeatmap[i] = templates.HeatmapDay{
			Date:  d.Date,
			Count: d.Count,
		}
	}

	if err := templates.HeatmapChart(templateHeatmap, filter.Year, filter.Month, filter.Timezone).Render(ctx, w); err != nil {
		slog.Error("failed to render heatmap", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// TimelinePartial returns the timeline chart data for HTMX
func (h *Handler) TimelinePartial(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	vars := mux.Vars(r)
	chatID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid chat ID", http.StatusBadRequest)
		return
	}

	filter := parseFilterParams(r)

	granularity := r.URL.Query().Get("granularity")
	if granularity == "" {
		granularity = "day"
	}

	// Add custom attributes
	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("granularity", granularity)
	}

	timeline, err := h.api.GetTimeline(ctx, chatID, filter.StartDate, filter.EndDate, granularity)
	if err != nil {
		slog.Error("failed to get timeline from API", "error", err, "chat_id", chatID)
		timeline = []apiclient.TimelinePoint{}
	}

	templateTimeline := make([]templates.TimelinePoint, len(timeline))
	for i, t := range timeline {
		templateTimeline[i] = templates.TimelinePoint{
			Period:        t.Period,
			MessageCount:  t.MessageCount,
			UserCount:     t.UserCount,
			ReactionCount: t.ReactionCount,
		}
	}

	if err := templates.TimelineChart(templateTimeline, granularity, filter.Timezone).Render(ctx, w); err != nil {
		slog.Error("failed to render timeline", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// TopContentPartial returns the top content list for HTMX
func (h *Handler) TopContentPartial(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	vars := mux.Vars(r)
	chatID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid chat ID", http.StatusBadRequest)
		return
	}

	filter := parseFilterParams(r)

	metric := r.URL.Query().Get("metric")
	if metric == "" {
		metric = "most_reacted"
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 10
	}

	// Add custom attributes
	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("metric", metric)
		txn.AddAttribute("limit", limit)
	}

	topContent, err := h.api.GetTopContent(ctx, chatID, filter.StartDate, filter.EndDate, metric, limit)
	if err != nil {
		slog.Error("failed to get top content from API", "error", err, "chat_id", chatID)
		topContent = []apiclient.TopMessage{}
	}

	templateTopContent := make([]templates.TopMessage, len(topContent))
	for i, m := range topContent {
		templateTopContent[i] = templates.TopMessage{
			MessageID: m.MessageID,
			UserID:    m.UserID,
			Username:  m.Username,
			FirstName: m.FirstName,
			LastName:  m.LastName,
			Text:      m.Text,
			Score:     m.Score,
			SentAt:    m.Date,
		}
		templateTopContent[i].TopReactions = make([]templates.EmojiBreakdown, len(m.TopReactions))
		for j, rct := range m.TopReactions {
			templateTopContent[i].TopReactions[j] = templates.EmojiBreakdown{Emoji: rct.Emoji, Count: rct.Count}
		}
	}

	if err := templates.TopContentList(templateTopContent, metric).Render(ctx, w); err != nil {
		slog.Error("failed to render top content", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// UserDetailPartial returns the user detail modal for HTMX
func (h *Handler) UserDetailPartial(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	vars := mux.Vars(r)
	chatID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid chat ID", http.StatusBadRequest)
		return
	}

	userID, err := strconv.ParseInt(vars["user_id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Add custom attributes
	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("user_id", userID)
	}

	filter := parseFilterParams(r)

	userDetail, err := h.api.GetUserDetail(ctx, chatID, userID, filter.StartDate, filter.EndDate)
	if err != nil {
		slog.Error("failed to get user detail from API", "error", err, "chat_id", chatID, "user_id", userID)
		if err := templates.UserDetailContent(nil, filter.Timezone).Render(ctx, w); err != nil {
			slog.Error("failed to render user not found", "error", err)
		}
		return
	}

	// Convert activity by hour from map to array
	activityByHour := make([]int, 24)
	for hourStr, count := range userDetail.ActivityByHour {
		hour, err := strconv.Atoi(hourStr)
		if err == nil && hour >= 0 && hour < 24 {
			activityByHour[hour] = count
		}
	}

	templateUser := &templates.UserDetail{
		UserID:    userDetail.UserID,
		Username:  userDetail.Username,
		FirstName: userDetail.FirstName,
		LastName:  userDetail.LastName,
		Stats: templates.UserDetailStats{
			MessageCount:      userDetail.Stats.TotalMessages,
			ReactionsGiven:    userDetail.Stats.ReactionsGiven,
			ReactionsReceived: userDetail.Stats.ReactionsReceived,
			MediaSent:         userDetail.Stats.MediaSent,
			RepliesSent:       userDetail.Stats.RepliesSent,
			RepliesReceived:   userDetail.Stats.RepliesReceived,
			AvgMessageLength:  userDetail.Stats.AvgMessageLength,
			FirstActive:       userDetail.Stats.FirstActive,
			LastActive:        userDetail.Stats.LastActive,
		},
		ActivityByHour: activityByHour,
	}

	if userDetail.CurrentStreak != nil {
		templateUser.CurrentStreak = &templates.StreakInfo{
			Days:      userDetail.CurrentStreak.Days,
			StartDate: userDetail.CurrentStreak.StartDate,
			EndDate:   userDetail.CurrentStreak.EndDate,
		}
	}

	if userDetail.LongestStreak != nil {
		templateUser.LongestStreak = &templates.StreakInfo{
			Days:      userDetail.LongestStreak.Days,
			StartDate: userDetail.LongestStreak.StartDate,
			EndDate:   userDetail.LongestStreak.EndDate,
		}
	}

	templateUser.TopEmojisUsed = make([]templates.EmojiBreakdown, len(userDetail.TopEmojisUsed))
	for i, e := range userDetail.TopEmojisUsed {
		templateUser.TopEmojisUsed[i] = templates.EmojiBreakdown{Emoji: e.Emoji, Count: e.Count}
	}

	templateUser.TopEmojisReceived = make([]templates.EmojiBreakdown, len(userDetail.TopEmojisReceived))
	for i, e := range userDetail.TopEmojisReceived {
		templateUser.TopEmojisReceived[i] = templates.EmojiBreakdown{Emoji: e.Emoji, Count: e.Count}
	}

	if err := templates.UserDetailContent(templateUser, filter.Timezone).Render(ctx, w); err != nil {
		slog.Error("failed to render user detail modal", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// CompareUsersPartial returns the user comparison table for HTMX
func (h *Handler) CompareUsersPartial(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txn := newrelic.FromContext(ctx)

	vars := mux.Vars(r)
	chatID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid chat ID", http.StatusBadRequest)
		return
	}

	filter := parseFilterParams(r)

	// Parse user IDs from query param
	userIDsStr := r.URL.Query().Get("user_ids")
	if userIDsStr == "" {
		http.Error(w, "user_ids parameter required", http.StatusBadRequest)
		return
	}

	var userIDs []int64
	for _, idStr := range splitAndTrim(userIDsStr, ",") {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			continue
		}
		userIDs = append(userIDs, id)
	}

	if len(userIDs) == 0 {
		http.Error(w, "No valid user IDs provided", http.StatusBadRequest)
		return
	}

	// Add custom attributes
	if txn != nil {
		txn.AddAttribute("chat_id", chatID)
		txn.AddAttribute("user_count", len(userIDs))
	}

	comparisons, err := h.api.CompareUsers(ctx, chatID, userIDs, filter.StartDate, filter.EndDate)
	if err != nil {
		slog.Error("failed to compare users from API", "error", err, "chat_id", chatID)
		comparisons = []apiclient.UserComparison{}
	}

	templateComparisons := make([]templates.UserComparison, len(comparisons))
	for i, c := range comparisons {
		templateComparisons[i] = templates.UserComparison{
			UserID:            c.UserID,
			Username:          c.Username,
			FirstName:         c.FirstName,
			LastName:          c.LastName,
			MessageCount:      c.MessageCount,
			ReactionsGiven:    c.ReactionsGiven,
			ReactionsReceived: c.ReactionsReceived,
			MediaSent:         c.MediaSent,
			AvgMessageLength:  c.AvgMessageLength,
		}
	}

	if err := templates.CompareUsersTable(templateComparisons).Render(ctx, w); err != nil {
		slog.Error("failed to render user comparison", "error", err)
		if txn != nil {
			txn.NoticeError(err)
		}
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

// calculateAvailableYears returns a list of years between first and last message
func calculateAvailableYears(firstMessage, lastMessage time.Time) []int {
	startYear := firstMessage.Year()
	endYear := lastMessage.Year()
	currentYear := time.Now().Year()

	if endYear < currentYear {
		endYear = currentYear
	}

	var years []int
	for y := endYear; y >= startYear; y-- {
		years = append(years, y)
	}

	if len(years) == 0 {
		years = append(years, currentYear)
	}

	return years
}

// filterParams holds parsed filter parameters
type filterParams struct {
	Year      int
	Month     int
	Timezone  string
	StartDate time.Time
	EndDate   time.Time
}

// parseFilterParams extracts filter parameters from the request query string
func parseFilterParams(r *http.Request) filterParams {
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	if year == 0 {
		year = time.Now().Year()
	}

	month, _ := strconv.Atoi(r.URL.Query().Get("month"))
	if month < 0 || month > 12 {
		month = 0
	}

	timezone := r.URL.Query().Get("tz")
	if timezone == "" {
		timezone = "UTC"
	}

	// Calculate date range
	var startDate, endDate time.Time
	if month == 0 {
		startDate = time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		endDate = time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC)
		if year == time.Now().Year() {
			endDate = time.Now().Add(24 * time.Hour)
		}
	} else {
		startDate = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		endDate = time.Date(year, time.Month(month)+1, 1, 0, 0, 0, 0, time.UTC)
		if month == 12 {
			endDate = time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		now := time.Now()
		if year == now.Year() && month == int(now.Month()) {
			endDate = now.Add(24 * time.Hour)
		}
	}

	return filterParams{
		Year:      year,
		Month:     month,
		Timezone:  timezone,
		StartDate: startDate,
		EndDate:   endDate,
	}
}

// splitAndTrim splits a string by separator and trims whitespace from each part
func splitAndTrim(s, sep string) []string {
	parts := make([]string, 0)
	for _, part := range splitString(s, sep) {
		trimmed := trimString(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func splitString(s, sep string) []string {
	if s == "" {
		return nil
	}
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimString(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
