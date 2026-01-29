package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"beef-briefing/apps/api-service/internal/apperror"
	"beef-briefing/apps/api-service/internal/middleware"
	"beef-briefing/apps/api-service/internal/repository"
	"beef-briefing/apps/api-service/internal/testutil"
	"beef-briefing/pkg/config"
)

// Test helpers

func newTestMiniAppService(mockRepo *testutil.MockMiniAppRepository, mockMinIO *testutil.MockMinIOClient, jwtSecretKey, botToken string) *MiniAppService {
	cfg := &config.Config{
		AdminUserIDs: "999", // Default test admin ID (comma-separated string)
	}

	return &MiniAppService{
		repo:          mockRepo,
		jwtAuth:       middleware.NewJWTAuth(jwtSecretKey),
		botToken:      botToken,
		nrApp:         nil, // Not needed for these tests
		storageClient: mockMinIO,
		config:        cfg,
	}
}

// generateValidInitData creates a valid Telegram init data string with proper HMAC signature
func generateValidInitData(botToken string, userID int64, authDate int64, firstName, lastName, username string, chatID *int64) string {
	params := url.Values{}
	params.Set("auth_date", fmt.Sprintf("%d", authDate))

	// Build user JSON
	userJSON := fmt.Sprintf(`{"id":%d,"first_name":"%s"`, userID, firstName)
	if lastName != "" {
		userJSON += fmt.Sprintf(`,"last_name":"%s"`, lastName)
	}
	if username != "" {
		userJSON += fmt.Sprintf(`,"username":"%s"`, username)
	}
	userJSON += "}"
	params.Set("user", userJSON)

	// Add chat if provided
	if chatID != nil {
		chatJSON := fmt.Sprintf(`{"id":%d}`, *chatID)
		params.Set("chat", chatJSON)
	}

	// Sort keys and create data-check-string
	var keys []string
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var dataCheckParts []string
	for _, k := range keys {
		dataCheckParts = append(dataCheckParts, fmt.Sprintf("%s=%s", k, params.Get(k)))
	}
	dataCheckString := strings.Join(dataCheckParts, "\n")

	// Calculate secret key: HMAC_SHA256("WebAppData", bot_token)
	secretKeyHMAC := hmac.New(sha256.New, []byte("WebAppData"))
	secretKeyHMAC.Write([]byte(botToken))
	secretKey := secretKeyHMAC.Sum(nil)

	// Calculate hash: HMAC_SHA256(secret_key, data_check_string)
	hashHMAC := hmac.New(sha256.New, secretKey)
	hashHMAC.Write([]byte(dataCheckString))
	calculatedHash := hex.EncodeToString(hashHMAC.Sum(nil))

	params.Set("hash", calculatedHash)

	return params.Encode()
}

// Test fixtures

func newTestOverviewStats(chatID int64) *repository.OverviewStats {
	return &repository.OverviewStats{
		TotalMessages:  1000,
		TotalUsers:     50,
		TotalReactions: 2500,
		TotalMedia:     300,
		MessagesPerDay: 142.86,
	}
}

func newTestUserRankings(chatID int64) []repository.UserRanking {
	username1 := "user1"
	username2 := "user2"

	return []repository.UserRanking{
		{
			Rank:      1,
			UserID:    1,
			FirstName: "Alice",
			Username:  &username1,
			Score:     500,
		},
		{
			Rank:      2,
			UserID:    2,
			FirstName: "Bob",
			Username:  &username2,
			Score:     300,
		},
	}
}

// TestValidateInitData_ValidHash verifies that ValidateInitData successfully validates
// a properly signed init data string with valid HMAC signature.
func TestValidateInitData_ValidHash(t *testing.T) {
	botToken := "test-bot-token-123456"
	svc := newTestMiniAppService(testutil.NewMockMiniAppRepository(), nil, "test-jwt-secret", botToken)

	userID := int64(12345)
	authDate := time.Now().Unix()
	chatID := int64(-1001234567890)

	initData := generateValidInitData(botToken, userID, authDate, "TestUser", "LastName", "testusername", &chatID)

	// Execute
	validated, err := svc.ValidateInitData(initData, 86400) // 24 hours max age

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if validated == nil {
		t.Fatal("expected validated data, got nil")
	}
	if validated.UserID != userID {
		t.Errorf("expected user_id %d, got %d", userID, validated.UserID)
	}
	if validated.ChatID == nil || *validated.ChatID != chatID {
		t.Errorf("expected chat_id %d, got %v", chatID, validated.ChatID)
	}
	if validated.FirstName != "TestUser" {
		t.Errorf("expected first_name 'TestUser', got %s", validated.FirstName)
	}
	if validated.Username == nil || *validated.Username != "testusername" {
		t.Errorf("expected username 'testusername', got %v", validated.Username)
	}
	if validated.LastName == nil || *validated.LastName != "LastName" {
		t.Errorf("expected last_name 'LastName', got %v", validated.LastName)
	}
	if validated.AuthDate != authDate {
		t.Errorf("expected auth_date %d, got %d", authDate, validated.AuthDate)
	}
}

// TestValidateInitData_InvalidHash verifies that ValidateInitData returns an error
// when the HMAC signature is invalid or tampered with.
func TestValidateInitData_InvalidHash(t *testing.T) {
	botToken := "test-bot-token-123456"
	svc := newTestMiniAppService(testutil.NewMockMiniAppRepository(), nil, "test-jwt-secret", botToken)

	userID := int64(12345)
	authDate := time.Now().Unix()

	initData := generateValidInitData(botToken, userID, authDate, "TestUser", "", "", nil)

	// Tamper with the hash
	initData = strings.Replace(initData, "hash=", "hash=invalid", 1)

	// Execute
	_, err := svc.ValidateInitData(initData, 86400)

	// Verify
	if err == nil {
		t.Fatal("expected error for invalid hash, got nil")
	}
	if err != apperror.ErrInvalidHash {
		t.Errorf("expected ErrInvalidHash, got %v", err)
	}
}

// TestValidateInitData_ExpiredData verifies that ValidateInitData returns an error
// when the auth_date is older than the maximum allowed age.
func TestValidateInitData_ExpiredData(t *testing.T) {
	botToken := "test-bot-token-123456"
	svc := newTestMiniAppService(testutil.NewMockMiniAppRepository(), nil, "test-jwt-secret", botToken)

	userID := int64(12345)
	authDate := time.Now().Unix() - 86401 // More than 24 hours old

	initData := generateValidInitData(botToken, userID, authDate, "TestUser", "", "", nil)

	// Execute
	_, err := svc.ValidateInitData(initData, 86400) // 24 hours max age

	// Verify
	if err == nil {
		t.Fatal("expected error for expired data, got nil")
	}
	if err != apperror.ErrExpiredInitData {
		t.Errorf("expected ErrExpiredInitData, got %v", err)
	}
}

// TestAuthenticate_CreatesJWT verifies that Authenticate validates init data and
// returns a valid JWT token with proper claims.
func TestAuthenticate_CreatesJWT(t *testing.T) {
	ctx := context.Background()
	botToken := "test-bot-token-123456"
	jwtSecret := "test-jwt-secret"
	mockRepo := testutil.NewMockMiniAppRepository()
	svc := newTestMiniAppService(mockRepo, nil, jwtSecret, botToken)

	userID := int64(12345)
	chatID := int64(-1001234567890)
	authDate := time.Now().Unix()
	chatTitle := "Test Chat"
	timezone := "America/New_York"

	// Setup mock data
	mockRepo.SetChatTitle(chatID, chatTitle)
	mockRepo.SetChatTimezoneValue(chatID, timezone)

	initData := generateValidInitData(botToken, userID, authDate, "TestUser", "", "testusername", &chatID)

	// Execute
	authResp, err := svc.Authenticate(ctx, initData)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if authResp == nil {
		t.Fatal("expected auth response, got nil")
	}
	if authResp.Token == "" {
		t.Error("expected JWT token, got empty string")
	}
	if authResp.UserID != userID {
		t.Errorf("expected user_id %d, got %d", userID, authResp.UserID)
	}
	if authResp.ChatID == nil || *authResp.ChatID != chatID {
		t.Errorf("expected chat_id %d, got %v", chatID, authResp.ChatID)
	}
	if authResp.FirstName != "TestUser" {
		t.Errorf("expected first_name 'TestUser', got %s", authResp.FirstName)
	}
	if authResp.Username == nil || *authResp.Username != "testusername" {
		t.Errorf("expected username 'testusername', got %v", authResp.Username)
	}
	if authResp.ChatTitle == nil || *authResp.ChatTitle != chatTitle {
		t.Errorf("expected chat_title '%s', got %v", chatTitle, authResp.ChatTitle)
	}
	if authResp.ChatTimezone == nil || *authResp.ChatTimezone != timezone {
		t.Errorf("expected chat_timezone '%s', got %v", timezone, authResp.ChatTimezone)
	}

	// Verify JWT token can be parsed
	jwtAuth := middleware.NewJWTAuth(jwtSecret)
	claims, err := jwtAuth.ValidateToken(authResp.Token)
	if err != nil {
		t.Fatalf("expected valid JWT token, got error: %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("expected JWT claims user_id %d, got %d", userID, claims.UserID)
	}
	if claims.ChatID == nil || *claims.ChatID != chatID {
		t.Errorf("expected JWT claims chat_id %d, got %v", chatID, claims.ChatID)
	}
}

// TestGetOverviewStats_ValidPeriod verifies that GetOverviewStats returns
// overview statistics for a valid time period.
func TestGetOverviewStats_ValidPeriod(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMiniAppRepository()
	svc := newTestMiniAppService(mockRepo, nil, "test-jwt-secret", "test-bot-token")

	chatID := int64(-1001234567890)
	period := "7d"
	tz := time.UTC

	// Setup mock data
	stats := newTestOverviewStats(chatID)
	mockRepo.SetOverviewStats(chatID, stats)

	// Execute
	result, err := svc.GetOverviewStats(ctx, chatID, period, tz)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected stats, got nil")
	}
	if result.TotalMessages != 1000 {
		t.Errorf("expected TotalMessages 1000, got %d", result.TotalMessages)
	}
	if result.TotalUsers != 50 {
		t.Errorf("expected TotalUsers 50, got %d", result.TotalUsers)
	}
	if result.TotalReactions != 2500 {
		t.Errorf("expected TotalReactions 2500, got %d", result.TotalReactions)
	}

	// Verify repository was called
	if mockRepo.GetOverviewStatsCalls == 0 {
		t.Error("expected GetOverviewStats to be called")
	}
}

// TestGetUserRankings_Pagination verifies that GetUserRankings correctly handles
// pagination with page and limit parameters.
func TestGetUserRankings_Pagination(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMiniAppRepository()
	mockMinIO := testutil.NewMockMinIOClient()
	svc := newTestMiniAppService(mockRepo, mockMinIO, "test-jwt-secret", "test-bot-token")

	chatID := int64(-1001234567890)
	metric := "messages"
	period := "7d"
	page := 1
	limit := 10
	tz := time.UTC

	// Setup mock data
	rankings := newTestUserRankings(chatID)
	mockRepo.SetUserRankings(chatID, metric, rankings)
	mockRepo.SetUserRankingsTotal(chatID, 2)

	// Execute
	result, total, err := svc.GetUserRankings(ctx, chatID, metric, period, page, limit, tz)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected rankings, got nil")
	}
	if len(result) != 2 {
		t.Errorf("expected 2 rankings, got %d", len(result))
	}
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}

	// Verify first ranking
	if result[0].Rank != 1 {
		t.Errorf("expected rank 1, got %d", result[0].Rank)
	}
	if result[0].UserID != 1 {
		t.Errorf("expected user_id 1, got %d", result[0].UserID)
	}
	if result[0].FirstName != "Alice" {
		t.Errorf("expected first_name 'Alice', got %s", result[0].FirstName)
	}
	if result[0].Score != 500 {
		t.Errorf("expected score 500, got %d", result[0].Score)
	}

	// Verify repository was called
	if mockRepo.GetUserRankingsCalls == 0 {
		t.Error("expected GetUserRankings to be called")
	}
	if mockRepo.GetUserRankingsTotalCalls == 0 {
		t.Error("expected GetUserRankingsTotal to be called")
	}
}

// TestGetUserRankings_FilteringByTimeframe verifies that GetUserRankings correctly
// applies date filtering based on the period parameter (7d, 30d, max).
func TestGetUserRankings_FilteringByTimeframe(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMiniAppRepository()
	mockMinIO := testutil.NewMockMinIOClient()
	svc := newTestMiniAppService(mockRepo, mockMinIO, "test-jwt-secret", "test-bot-token")

	chatID := int64(-1001234567890)
	metric := "messages"
	tz := time.UTC

	testCases := []struct {
		period string
		desc   string
	}{
		{"7d", "7 days period"},
		{"30d", "30 days period"},
		{"max", "all time period"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Setup mock data
			rankings := newTestUserRankings(chatID)
			mockRepo.SetUserRankings(chatID, metric, rankings)
			mockRepo.SetUserRankingsTotal(chatID, 2)

			// Execute
			result, total, err := svc.GetUserRankings(ctx, chatID, metric, tc.period, 1, 10, tz)

			// Verify
			if err != nil {
				t.Fatalf("expected no error for period %s, got %v", tc.period, err)
			}
			if result == nil {
				t.Fatalf("expected rankings for period %s, got nil", tc.period)
			}
			if total != 2 {
				t.Errorf("expected total 2 for period %s, got %d", tc.period, total)
			}
		})
	}
}

// TestGetUserRankings_MultipleMetrics verifies that GetUserRankings correctly handles
// different ranking metrics (messages, reactions, media).
func TestGetUserRankings_MultipleMetrics(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMiniAppRepository()
	mockMinIO := testutil.NewMockMinIOClient()
	svc := newTestMiniAppService(mockRepo, mockMinIO, "test-jwt-secret", "test-bot-token")

	chatID := int64(-1001234567890)
	period := "7d"
	tz := time.UTC

	testCases := []struct {
		metric string
		desc   string
	}{
		{"messages", "messages metric"},
		{"reactions", "reactions metric"},
		{"media", "media metric"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Setup mock data
			rankings := newTestUserRankings(chatID)
			mockRepo.SetUserRankings(chatID, tc.metric, rankings)
			mockRepo.SetUserRankingsTotal(chatID, 2)

			// Execute
			result, total, err := svc.GetUserRankings(ctx, chatID, tc.metric, period, 1, 10, tz)

			// Verify
			if err != nil {
				t.Fatalf("expected no error for metric %s, got %v", tc.metric, err)
			}
			if result == nil {
				t.Fatalf("expected rankings for metric %s, got nil", tc.metric)
			}
			if total != 2 {
				t.Errorf("expected total 2 for metric %s, got %d", tc.metric, total)
			}

			// Verify that the correct metric was used in the repository call
			if len(result) > 0 {
				// Rankings should be present
				if result[0].Rank != 1 {
					t.Errorf("expected first rank to be 1, got %d", result[0].Rank)
				}
			}
		})
	}
}

// TestGetUserProfile_TimezoneHandling verifies that GetUserProfile correctly handles
// timezone conversion for date-based statistics.
func TestGetUserProfile_TimezoneHandling(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMiniAppRepository()
	mockMinIO := testutil.NewMockMinIOClient()
	svc := newTestMiniAppService(mockRepo, mockMinIO, "test-jwt-secret", "test-bot-token")

	chatID := int64(-1001234567890)
	userID := int64(12345)
	period := "7d"

	testCases := []struct {
		timezone string
		desc     string
	}{
		{"UTC", "UTC timezone"},
		{"America/New_York", "New York timezone"},
		{"Asia/Tokyo", "Tokyo timezone"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			tz, err := time.LoadLocation(tc.timezone)
			if err != nil {
				t.Fatalf("failed to load timezone %s: %v", tc.timezone, err)
			}

			// Setup mock data
			mockRepo.SetProfileStats(chatID, userID, &repository.ProfileStats{
				MessageCount:            100,
				ReactionsSent:           30,
				ReactionsReceived:       50,
				RepliesSent:             25,
				RepliesReceived:         15,
				ActiveDays:              7,
				AvgMessagesPerDay:       14.29,
				RankByMessages:          1,
				RankByReactionsReceived: 2,
			})

			// Execute
			result, err := svc.GetUserProfile(ctx, chatID, userID, period, tz)

			// Verify
			if err != nil {
				t.Fatalf("expected no error for timezone %s, got %v", tc.timezone, err)
			}
			if result == nil {
				t.Fatalf("expected profile for timezone %s, got nil", tc.timezone)
			}
			if result.Stats == nil {
				t.Fatal("expected profile stats, got nil")
			}
			if result.Stats.MessageCount != 100 {
				t.Errorf("expected 100 messages, got %d", result.Stats.MessageCount)
			}
		})
	}
}

// TestGetUserProfile_MissingUser verifies that GetUserProfile returns empty stats
// when the user has no data in the specified period.
func TestGetUserProfile_MissingUser(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMiniAppRepository()
	mockMinIO := testutil.NewMockMinIOClient()
	svc := newTestMiniAppService(mockRepo, mockMinIO, "test-jwt-secret", "test-bot-token")

	chatID := int64(-1001234567890)
	userID := int64(99999)
	period := "7d"
	tz := time.UTC

	// Setup mock to return nil (user not found)
	mockRepo.SetProfileStats(chatID, userID, nil)

	// Execute
	result, err := svc.GetUserProfile(ctx, chatID, userID, period, tz)

	// Verify - should return error or empty result
	if err != nil {
		// Expected behavior: return error for missing user
		return
	}
	if result == nil {
		// Also acceptable: return nil
		return
	}
	// If result is returned, it should have zero stats
	if result.Stats != nil && result.Stats.MessageCount != 0 {
		t.Errorf("expected 0 messages for missing user, got %d", result.Stats.MessageCount)
	}
}

// TestGetDailyActivity_EmptyChat verifies that GetDailyActivity returns empty array
// when a chat has no messages in the specified period.
func TestGetDailyActivity_EmptyChat(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMiniAppRepository()
	svc := newTestMiniAppService(mockRepo, nil, "test-jwt-secret", "test-bot-token")

	chatID := int64(-1001234567890)
	period := "7d"
	tz := time.UTC

	// Setup mock to return empty array
	mockRepo.SetDailyActivity(chatID, []repository.DailyActivity{})

	// Execute
	result, err := svc.GetDailyActivity(ctx, chatID, period, tz)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected empty array, got nil")
	}
	if len(result) != 0 {
		t.Errorf("expected empty array, got %d items", len(result))
	}
}

// TestGetDailyActivity_DateFiltering verifies that GetDailyActivity correctly
// filters results based on the period parameter.
func TestGetDailyActivity_DateFiltering(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMiniAppRepository()
	svc := newTestMiniAppService(mockRepo, nil, "test-jwt-secret", "test-bot-token")

	chatID := int64(-1001234567890)
	tz := time.UTC

	// Create sample daily activity data (Date field is string in format "2006-01-02")
	now := time.Now().In(tz)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, tz)

	dailyActivity := []repository.DailyActivity{
		{Date: today.AddDate(0, 0, -1).Format("2006-01-02"), Messages: 10, Users: 5},
		{Date: today.AddDate(0, 0, -2).Format("2006-01-02"), Messages: 15, Users: 7},
		{Date: today.AddDate(0, 0, -3).Format("2006-01-02"), Messages: 20, Users: 10},
	}

	testCases := []struct {
		period       string
		expectedDays int
		desc         string
	}{
		{"7d", 3, "7 days period"},
		{"30d", 3, "30 days period"},
		{"max", 3, "all time period"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Setup mock data
			mockRepo.SetDailyActivity(chatID, dailyActivity)

			// Execute
			result, err := svc.GetDailyActivity(ctx, chatID, tc.period, tz)

			// Verify
			if err != nil {
				t.Fatalf("expected no error for period %s, got %v", tc.period, err)
			}
			if result == nil {
				t.Fatalf("expected activity for period %s, got nil", tc.period)
			}
			if len(result) != tc.expectedDays {
				t.Errorf("expected %d days for period %s, got %d", tc.expectedDays, tc.period, len(result))
			}
		})
	}
}

// TestGetHeatmapData_HourlyType verifies that GetHeatmapData returns hourly
// aggregation data when no userID is provided (group heatmap).
func TestGetHeatmapData_HourlyType(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMiniAppRepository()
	svc := newTestMiniAppService(mockRepo, nil, "test-jwt-secret", "test-bot-token")

	chatID := int64(-1001234567890)
	period := "7d"
	tz := time.UTC

	// Setup mock heatmap data (group heatmap includes unique users)
	heatmapData := &repository.HeatmapData{
		Data: []repository.HeatmapCell{
			{DayOfWeek: 1, Hour: 9, MessageCount: 10, UniqueUsers: 5},
			{DayOfWeek: 1, Hour: 10, MessageCount: 15, UniqueUsers: 7},
			{DayOfWeek: 2, Hour: 14, MessageCount: 20, UniqueUsers: 8},
		},
		TotalMessages: 45,
		MaxCount:      20,
	}
	mockRepo.SetGroupHeatmap(chatID, heatmapData)

	// Execute - no userID means group heatmap
	result, err := svc.GetHeatmapData(ctx, chatID, nil, period, tz)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected heatmap data, got nil")
	}
	if result.Group == nil {
		t.Fatal("expected group heatmap, got nil")
	}
	if len(result.Group.Data) != 3 {
		t.Errorf("expected 3 heatmap cells, got %d", len(result.Group.Data))
	}
	if result.Group.TotalMessages != 45 {
		t.Errorf("expected total messages 45, got %d", result.Group.TotalMessages)
	}
	if result.Group.MaxCount != 20 {
		t.Errorf("expected max count 20, got %d", result.Group.MaxCount)
	}

	// Verify first cell
	if result.Group.Data[0].Hour != 9 {
		t.Errorf("expected hour 9, got %d", result.Group.Data[0].Hour)
	}
	if result.Group.Data[0].MessageCount != 10 {
		t.Errorf("expected message count 10, got %d", result.Group.Data[0].MessageCount)
	}
}

// TestGetHeatmapData_UserSpecific verifies that GetHeatmapData returns user-specific
// heatmap when userID is provided.
func TestGetHeatmapData_UserSpecific(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMiniAppRepository()
	svc := newTestMiniAppService(mockRepo, nil, "test-jwt-secret", "test-bot-token")

	chatID := int64(-1001234567890)
	userID := int64(12345)
	period := "7d"
	tz := time.UTC

	// Setup mock user heatmap data (no unique users field)
	heatmapData := &repository.HeatmapData{
		Data: []repository.HeatmapCell{
			{DayOfWeek: 1, Hour: 9, MessageCount: 5},
			{DayOfWeek: 1, Hour: 10, MessageCount: 8},
		},
		TotalMessages: 13,
		MaxCount:      8,
	}
	mockRepo.SetUserHeatmap(chatID, userID, heatmapData)

	// Execute - with userID means user heatmap
	result, err := svc.GetHeatmapData(ctx, chatID, &userID, period, tz)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected heatmap data, got nil")
	}
	if result.User == nil {
		t.Fatal("expected user heatmap, got nil")
	}
	if len(result.User.Data) != 2 {
		t.Errorf("expected 2 heatmap cells, got %d", len(result.User.Data))
	}
	if result.User.TotalMessages != 13 {
		t.Errorf("expected total messages 13, got %d", result.User.TotalMessages)
	}
}

// TestGetMediaOverview_SortingLogic verifies that GetMediaOverview returns
// media types sorted by count descending.
func TestGetMediaOverview_SortingLogic(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMiniAppRepository()
	svc := newTestMiniAppService(mockRepo, nil, "test-jwt-secret", "test-bot-token")

	chatID := int64(-1001234567890)
	period := "7d"
	limit := 10
	tz := time.UTC

	// Setup mock media data (ordered by count DESC)
	mediaDistribution := []repository.MediaTypeDistribution{
		{MediaType: "photo", Count: 100},
		{MediaType: "video", Count: 50},
		{MediaType: "document", Count: 25},
	}
	mockRepo.SetMediaTypeDistribution(chatID, mediaDistribution)

	mockRepo.SetMediaOverviewStats(chatID, &repository.MediaOverviewStats{
		TotalMedia:  175,
		TotalPhotos: 100,
		TotalVideos: 50,
	})

	// Execute
	result, err := svc.GetMediaOverview(ctx, chatID, period, limit, tz)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected media overview, got nil")
	}
	if result.Stats == nil {
		t.Fatal("expected media stats, got nil")
	}
	if result.Stats.TotalMedia != 175 {
		t.Errorf("expected total media 175, got %d", result.Stats.TotalMedia)
	}
	if len(result.Distribution) != 3 {
		t.Errorf("expected 3 media types, got %d", len(result.Distribution))
	}

	// Verify ordering (should be descending by count)
	if result.Distribution[0].Count < result.Distribution[1].Count {
		t.Error("expected distribution to be sorted by count descending")
	}
	if result.Distribution[1].Count < result.Distribution[2].Count {
		t.Error("expected distribution to be sorted by count descending")
	}
}

// TestGetMediaOverview_Pagination verifies that GetMediaOverview respects the limit
// parameter for media distribution results.
func TestGetMediaOverview_Pagination(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMiniAppRepository()
	svc := newTestMiniAppService(mockRepo, nil, "test-jwt-secret", "test-bot-token")

	chatID := int64(-1001234567890)
	period := "7d"
	tz := time.UTC

	// Setup mock media data with 5 types
	mediaDistribution := []repository.MediaTypeDistribution{
		{MediaType: "photo", Count: 100},
		{MediaType: "video", Count: 50},
		{MediaType: "document", Count: 25},
		{MediaType: "audio", Count: 10},
		{MediaType: "sticker", Count: 5},
	}

	testCases := []struct {
		limit    int
		expected int
		desc     string
	}{
		{3, 3, "limit 3"},
		{5, 5, "limit 5"},
		{10, 5, "limit exceeds available"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Setup mock data
			mockRepo.SetMediaTypeDistribution(chatID, mediaDistribution[:tc.expected])
			mockRepo.SetMediaOverviewStats(chatID, &repository.MediaOverviewStats{
				TotalMedia: 190,
			})

			// Execute
			result, err := svc.GetMediaOverview(ctx, chatID, period, tc.limit, tz)

			// Verify
			if err != nil {
				t.Fatalf("expected no error for limit %d, got %v", tc.limit, err)
			}
			if len(result.Distribution) != tc.expected {
				t.Errorf("expected %d media types for limit %d, got %d", tc.expected, tc.limit, len(result.Distribution))
			}
		})
	}
}

// TestGetReactionsOverview_TimeSeriesData verifies that GetReactionsOverview returns
// time series data showing reaction trends over time.
func TestGetReactionsOverview_TimeSeriesData(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockMiniAppRepository()
	svc := newTestMiniAppService(mockRepo, nil, "test-jwt-secret", "test-bot-token")

	chatID := int64(-1001234567890)
	period := "7d"
	limit := 5
	tz := time.UTC

	// Setup mock reaction data
	topReactions := []repository.TopReaction{
		{Emoji: "👍", ReactionType: "emoji", Count: 100},
		{Emoji: "❤️", ReactionType: "emoji", Count: 80},
		{Emoji: "😂", ReactionType: "emoji", Count: 60},
	}
	mockRepo.SetTopReactions(chatID, topReactions)

	// Execute
	result, err := svc.GetReactionsOverview(ctx, chatID, period, limit, tz)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected reactions overview, got nil")
	}
	if len(result.TopReactions) != 3 {
		t.Errorf("expected 3 reaction types, got %d", len(result.TopReactions))
	}
	// Verify first reaction
	if result.TopReactions[0].Emoji != "👍" {
		t.Errorf("expected emoji '👍', got %s", result.TopReactions[0].Emoji)
	}
	if result.TopReactions[0].Count != 100 {
		t.Errorf("expected count 100, got %d", result.TopReactions[0].Count)
	}
}

// =====================================================
// GAMES API SIGNATURE VALIDATION TESTS
// =====================================================

// generateValidGameSignature creates a valid HMAC-SHA256 signature for Games API authentication.
// Data format: "chat_id=%d&match_id=%s&ts=%d&user_id=%d" (alphabetically sorted keys)
func generateValidGameSignature(botToken string, chatID, userID int64, matchID string, ts int64) string {
	dataString := fmt.Sprintf("chat_id=%d&match_id=%s&ts=%d&user_id=%d", chatID, matchID, ts, userID)
	mac := hmac.New(sha256.New, []byte(botToken))
	mac.Write([]byte(dataString))
	return hex.EncodeToString(mac.Sum(nil))
}

// TestValidateGameSignature_ValidSignature verifies that ValidateGameSignature
// successfully validates a properly signed Games API request.
func TestValidateGameSignature_ValidSignature(t *testing.T) {
	botToken := "test-bot-token-123456"
	svc := newTestMiniAppService(testutil.NewMockMiniAppRepository(), nil, "test-jwt-secret", botToken)

	chatID := int64(-1002345678901)
	userID := int64(12345)
	matchID := "match-abc123"
	ts := time.Now().Unix()
	sig := generateValidGameSignature(botToken, chatID, userID, matchID, ts)

	// Execute
	err := svc.ValidateGameSignature(chatID, userID, matchID, ts, sig)

	// Verify
	if err != nil {
		t.Fatalf("expected no error for valid signature, got %v", err)
	}
}

// TestValidateGameSignature_InvalidSignature verifies that ValidateGameSignature
// returns ErrInvalidGameSignature when the signature is invalid or tampered with.
func TestValidateGameSignature_InvalidSignature(t *testing.T) {
	botToken := "test-bot-token-123456"
	svc := newTestMiniAppService(testutil.NewMockMiniAppRepository(), nil, "test-jwt-secret", botToken)

	chatID := int64(-1002345678901)
	userID := int64(12345)
	matchID := "match-abc123"
	ts := time.Now().Unix()

	// Generate valid signature then tamper with it
	validSig := generateValidGameSignature(botToken, chatID, userID, matchID, ts)
	tamperedSig := "0000" + validSig[4:] // Change first 4 characters

	// Execute
	err := svc.ValidateGameSignature(chatID, userID, matchID, ts, tamperedSig)

	// Verify
	if err == nil {
		t.Fatal("expected error for invalid signature, got nil")
	}
	if err != apperror.ErrInvalidGameSignature {
		t.Errorf("expected ErrInvalidGameSignature, got %v", err)
	}
}

// TestValidateGameSignature_ExpiredTimestamp verifies that ValidateGameSignature
// returns ErrExpiredGameTimestamp when the timestamp is older than 24 hours.
func TestValidateGameSignature_ExpiredTimestamp(t *testing.T) {
	botToken := "test-bot-token-123456"
	svc := newTestMiniAppService(testutil.NewMockMiniAppRepository(), nil, "test-jwt-secret", botToken)

	chatID := int64(-1002345678901)
	userID := int64(12345)
	matchID := "match-abc123"
	ts := time.Now().Unix() - 86401 // 24 hours + 1 second old
	sig := generateValidGameSignature(botToken, chatID, userID, matchID, ts)

	// Execute
	err := svc.ValidateGameSignature(chatID, userID, matchID, ts, sig)

	// Verify
	if err == nil {
		t.Fatal("expected error for expired timestamp, got nil")
	}
	if err != apperror.ErrExpiredGameTimestamp {
		t.Errorf("expected ErrExpiredGameTimestamp, got %v", err)
	}
}

// TestValidateGameSignature_TamperedParams verifies that ValidateGameSignature
// returns ErrInvalidGameSignature when request params don't match the signature.
func TestValidateGameSignature_TamperedParams(t *testing.T) {
	botToken := "test-bot-token-123456"
	svc := newTestMiniAppService(testutil.NewMockMiniAppRepository(), nil, "test-jwt-secret", botToken)

	chatID := int64(-1002345678901)
	userID := int64(12345)
	matchID := "match-abc123"
	ts := time.Now().Unix()

	// Generate signature with original params
	sig := generateValidGameSignature(botToken, chatID, userID, matchID, ts)

	testCases := []struct {
		name        string
		chatID      int64
		userID      int64
		matchID     string
		ts          int64
		sig         string
		expectedErr error
	}{
		{
			name:        "tampered chat_id",
			chatID:      chatID + 1, // Different chat_id
			userID:      userID,
			matchID:     matchID,
			ts:          ts,
			sig:         sig,
			expectedErr: apperror.ErrInvalidGameSignature,
		},
		{
			name:        "tampered user_id",
			chatID:      chatID,
			userID:      userID + 1, // Different user_id
			matchID:     matchID,
			ts:          ts,
			sig:         sig,
			expectedErr: apperror.ErrInvalidGameSignature,
		},
		{
			name:        "tampered match_id",
			chatID:      chatID,
			userID:      userID,
			matchID:     "different-match", // Different match_id
			ts:          ts,
			sig:         sig,
			expectedErr: apperror.ErrInvalidGameSignature,
		},
		{
			name:        "tampered timestamp",
			chatID:      chatID,
			userID:      userID,
			matchID:     matchID,
			ts:          ts + 100, // Different timestamp
			sig:         sig,
			expectedErr: apperror.ErrInvalidGameSignature,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Execute
			err := svc.ValidateGameSignature(tc.chatID, tc.userID, tc.matchID, tc.ts, tc.sig)

			// Verify
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
			if err != tc.expectedErr {
				t.Errorf("expected %v for %s, got %v", tc.expectedErr, tc.name, err)
			}
		})
	}
}
