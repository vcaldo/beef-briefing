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
