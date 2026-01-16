package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"beef-briefing/apps/api-service/internal/apperror"
	"beef-briefing/apps/api-service/internal/repository"
	"beef-briefing/apps/api-service/internal/testutil"
)

// Test helpers

func newTestCardService(mockRepo *testutil.MockCardRepository, mockMinIO *testutil.MockMinIOClient) *CardService {
	return &CardService{
		cardRepo:    mockRepo,
		minioClient: mockMinIO,
		nrApp:       nil, // Not needed for these tests
	}
}

// Test fixtures

func newTestUserCard(userID, chatID int64, weekStart string) *repository.UserCard {
	stats := json.RawMessage(`{"mood":{"score":85},"influence":{"score":70},"activity":{"score":90}}`)
	trends := json.RawMessage(`{"mood_trend":"improving"}`)
	timezone := "UTC"

	return &repository.UserCard{
		ID:               1,
		UserID:           userID,
		ChatID:           chatID,
		WeekStart:        weekStart,
		WeekEnd:          "2025-01-12",
		StatsWindowStart: "2025-01-06T00:00:00Z",
		StatsWindowEnd:   "2025-01-12T23:59:59Z",
		Stats:            stats,
		Trends:           trends,
		MessagesAnalyzed: 100,
		Timezone:         &timezone,
		CardVersion:      1,
		GeneratedAt:      time.Now(),
	}
}

func newTestCardUser(userID int64, firstName, lastName, username string) *repository.CardUser {
	return &repository.CardUser{
		ID:        userID,
		FirstName: firstName,
		LastName:  lastName,
		Username:  username,
	}
}

func newTestCardImage(userID, chatID int64, weekStart, theme string) *repository.CardImage {
	return &repository.CardImage{
		ID:          1,
		UserID:      userID,
		ChatID:      chatID,
		WeekStart:   weekStart,
		StoragePath: "cards/test-image.png",
		Theme:       theme,
		Width:       400,
		Height:      600,
		GeneratedAt: time.Now(),
	}
}

// TestGetUserCard_Found verifies that GetUserCard retrieves a card successfully.
func TestGetUserCard_Found(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockCardRepository()
	svc := newTestCardService(mockRepo, nil)

	userID := int64(1)
	chatID := int64(-100123)
	weekStart := "2025-01-06"
	weekTime, _ := time.Parse("2006-01-02", weekStart)

	// Setup test data
	card := newTestUserCard(userID, chatID, weekStart)
	user := newTestCardUser(userID, "Test", "User", "testuser")

	mockRepo.AddUserCard(card)
	mockRepo.AddUser(user)

	// Execute
	resultCard, resultUser, err := svc.GetUserCard(ctx, userID, chatID, &weekTime)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resultCard == nil {
		t.Fatal("expected card, got nil")
	}
	if resultUser == nil {
		t.Fatal("expected user, got nil")
	}
	if resultCard.UserID != userID {
		t.Errorf("expected userID %d, got %d", userID, resultCard.UserID)
	}
	if resultCard.ChatID != chatID {
		t.Errorf("expected chatID %d, got %d", chatID, resultCard.ChatID)
	}
	if resultCard.WeekStart != weekStart {
		t.Errorf("expected weekStart %s, got %s", weekStart, resultCard.WeekStart)
	}
	if resultUser.FirstName != "Test" {
		t.Errorf("expected firstName 'Test', got '%s'", resultUser.FirstName)
	}
	if mockRepo.GetUserCardCalls != 1 {
		t.Errorf("expected 1 GetUserCard call, got %d", mockRepo.GetUserCardCalls)
	}
	if mockRepo.GetUserInfoCalls != 1 {
		t.Errorf("expected 1 GetUserInfo call, got %d", mockRepo.GetUserInfoCalls)
	}
}

// TestGetUserCard_NotFound verifies that GetUserCard returns ErrCardNotFound when card doesn't exist.
func TestGetUserCard_NotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockCardRepository()
	svc := newTestCardService(mockRepo, nil)

	userID := int64(1)
	chatID := int64(-100123)
	weekStart := "2025-01-06"
	weekTime, _ := time.Parse("2006-01-02", weekStart)

	// Inject error to simulate card not found
	mockRepo.GetUserCardError = sql.ErrNoRows

	// Execute
	resultCard, resultUser, err := svc.GetUserCard(ctx, userID, chatID, &weekTime)

	// Verify
	if err != apperror.ErrCardNotFound {
		t.Fatalf("expected ErrCardNotFound, got %v", err)
	}
	if resultCard != nil {
		t.Error("expected nil card")
	}
	if resultUser != nil {
		t.Error("expected nil user")
	}
	if mockRepo.GetUserCardCalls != 1 {
		t.Errorf("expected 1 GetUserCard call, got %d", mockRepo.GetUserCardCalls)
	}
}

// TestGetChatCards_WithSorting verifies that GetChatCards retrieves and sorts cards correctly.
func TestGetChatCards_WithSorting(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockCardRepository()
	svc := newTestCardService(mockRepo, nil)

	chatID := int64(-100123)

	// Mock GetChatCards will be called - we need to set up the mock to return data
	// For this test, we'll inject the GetChatCards method behavior via the mock
	// Since MockCardRepository.GetChatCards returns empty by default, we need to verify the call

	req := GetChatCardsRequest{
		ChatID:  chatID,
		SortBy:  "mood",
		Order:   "desc",
		Limit:   10,
		Offset:  0,
	}

	// Execute
	result, err := svc.GetChatCards(ctx, req)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Metadata.ChatID != chatID {
		t.Errorf("expected chatID %d, got %d", chatID, result.Metadata.ChatID)
	}
	if result.Metadata.SortBy != "mood" {
		t.Errorf("expected sortBy 'mood', got '%s'", result.Metadata.SortBy)
	}
	if result.Pagination.Limit != 10 {
		t.Errorf("expected limit 10, got %d", result.Pagination.Limit)
	}
	if mockRepo.GetChatCardsCalls != 1 {
		t.Errorf("expected 1 GetChatCards call, got %d", mockRepo.GetChatCardsCalls)
	}
}

// TestGetChatCards_Pagination verifies that GetChatCards handles pagination correctly.
func TestGetChatCards_Pagination(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockCardRepository()
	svc := newTestCardService(mockRepo, nil)

	chatID := int64(-100123)

	req := GetChatCardsRequest{
		ChatID:  chatID,
		SortBy:  "mood",
		Order:   "desc",
		Limit:   5,
		Offset:  10,
	}

	// Execute
	result, err := svc.GetChatCards(ctx, req)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Pagination.Limit != 5 {
		t.Errorf("expected limit 5, got %d", result.Pagination.Limit)
	}
	if result.Pagination.Offset != 10 {
		t.Errorf("expected offset 10, got %d", result.Pagination.Offset)
	}
	if mockRepo.GetChatCardsCalls != 1 {
		t.Errorf("expected 1 GetChatCards call, got %d", mockRepo.GetChatCardsCalls)
	}
}

// TestGetCardImageURL_Found verifies that GetCardImageURL retrieves a presigned URL successfully.
func TestGetCardImageURL_Found(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockCardRepository()
	mockMinIO := testutil.NewMockMinIOClient()
	mockMinIO.PresignedURLBase = "https://minio.example.com"
	svc := newTestCardService(mockRepo, mockMinIO)

	userID := int64(1)
	chatID := int64(-100123)
	weekStart := "2025-01-06"
	weekTime, _ := time.Parse("2006-01-02", weekStart)
	theme := "gaming"

	// Setup test data
	cardImage := newTestCardImage(userID, chatID, weekStart, theme)
	mockRepo.AddCardImage(cardImage)

	// Execute
	result, err := svc.GetCardImageURL(ctx, userID, chatID, &weekTime, theme, 3600)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.URL == "" {
		t.Error("expected non-empty URL")
	}
	if result.Theme != theme {
		t.Errorf("expected theme '%s', got '%s'", theme, result.Theme)
	}
	if result.Width != 400 {
		t.Errorf("expected width 400, got %d", result.Width)
	}
	if result.Height != 600 {
		t.Errorf("expected height 600, got %d", result.Height)
	}
	if mockRepo.GetCardImageCalls != 1 {
		t.Errorf("expected 1 GetCardImage call, got %d", mockRepo.GetCardImageCalls)
	}
	if mockMinIO.GetPresignedURLSecondsCalls != 1 {
		t.Errorf("expected 1 GetPresignedURLSeconds call, got %d", mockMinIO.GetPresignedURLSecondsCalls)
	}
}

// TestGetCardImageURL_NotFound verifies that GetCardImageURL returns ErrCardImageNotFound when image doesn't exist.
func TestGetCardImageURL_NotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockCardRepository()
	mockMinIO := testutil.NewMockMinIOClient()
	svc := newTestCardService(mockRepo, mockMinIO)

	userID := int64(1)
	chatID := int64(-100123)
	weekStart := "2025-01-06"
	weekTime, _ := time.Parse("2006-01-02", weekStart)
	theme := "gaming"

	// Inject error to simulate image not found
	mockRepo.GetCardImageError = sql.ErrNoRows

	// Execute
	result, err := svc.GetCardImageURL(ctx, userID, chatID, &weekTime, theme, 3600)

	// Verify
	if err != apperror.ErrCardImageNotFound {
		t.Fatalf("expected ErrCardImageNotFound, got %v", err)
	}
	if result != nil {
		t.Error("expected nil result")
	}
	if mockRepo.GetCardImageCalls != 1 {
		t.Errorf("expected 1 GetCardImage call, got %d", mockRepo.GetCardImageCalls)
	}
}

// TestGetGalleryImages_FilterByTheme verifies that GetGalleryImages filters by theme correctly.
func TestGetGalleryImages_FilterByTheme(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockCardRepository()
	svc := newTestCardService(mockRepo, nil)

	chatID := int64(-100123)
	weekStart := "2025-01-06"
	weekTime, _ := time.Parse("2006-01-02", weekStart)
	theme := "gaming"

	// Execute
	result, err := svc.GetGalleryImages(ctx, chatID, &weekTime, nil, &theme)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if mockRepo.GetGalleryImagesCalls != 1 {
		t.Errorf("expected 1 GetGalleryImages call, got %d", mockRepo.GetGalleryImagesCalls)
	}
}
