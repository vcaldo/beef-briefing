package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"beef-briefing/apps/api-service/internal/apperror"
	"beef-briefing/apps/api-service/internal/jsonutil"
	"beef-briefing/apps/api-service/internal/nrutil"
	"beef-briefing/apps/api-service/internal/repository"
	"beef-briefing/apps/api-service/internal/services/embedded"

	"github.com/newrelic/go-agent/v3/newrelic"
)

// CardService handles user card business logic.
type CardService struct {
	cardRepo    repository.CardRepositoryInterface
	minioClient MinIOClientInterface
	nrApp       *newrelic.Application
}

// CardServiceDeps allows optional dependency injection for testing.
// If nil is passed to NewCardService, default implementations are used.
type CardServiceDeps struct {
	CardRepo    repository.CardRepositoryInterface
	MinIOClient MinIOClientInterface
}

// NewCardService creates a new CardService.
// Pass nil for deps to use default implementations (production use).
// Pass a CardServiceDeps struct to inject mock implementations (testing use).
func NewCardService(db *sql.DB, minioClient MinIOClientInterface, nrApp *newrelic.Application, deps *CardServiceDeps) *CardService {
	s := &CardService{
		minioClient: minioClient,
		nrApp:       nrApp,
	}

	// Use injected dependencies if provided, otherwise use defaults
	if deps != nil && deps.CardRepo != nil {
		s.cardRepo = deps.CardRepo
	} else {
		s.cardRepo = repository.NewCardRepository(db, nrApp)
	}

	if deps != nil && deps.MinIOClient != nil {
		s.minioClient = deps.MinIOClient
	}

	return s
}

// GetChatCardsRequest holds parameters for GetChatCards.
type GetChatCardsRequest struct {
	ChatID    int64
	WeekStart *time.Time
	SortBy    string
	Order     string
	Limit     int
	Offset    int
}

// GetChatCardsResponse is the response for GetChatCards.
type GetChatCardsResponse struct {
	Cards      []repository.CardWithUser `json:"cards"`
	Pagination PaginationInfo            `json:"pagination"`
	Metadata   ChatCardsMetadata         `json:"metadata"`
}

// PaginationInfo contains pagination details.
type PaginationInfo struct {
	Total   int  `json:"total"`
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	HasMore bool `json:"has_more"`
}

// ChatCardsMetadata contains metadata about the card query.
type ChatCardsMetadata struct {
	ChatID    int64  `json:"chat_id"`
	WeekStart string `json:"week_start"`
	WeekEnd   string `json:"week_end"`
	SortBy    string `json:"sort_by"`
	Order     string `json:"order"`
}

// UserHistoryResponse is the response for GetUserHistory.
type UserHistoryResponse struct {
	User    repository.CardUser `json:"user"`
	History []CardSummary       `json:"history"`
	Summary UserStatsSummary    `json:"summary"`
}

// CardSummary represents a card without full details.
type CardSummary struct {
	WeekStart        string          `json:"week_start"`
	WeekEnd          string          `json:"week_end"`
	Stats            json.RawMessage `json:"stats"`
	Trends           json.RawMessage `json:"trends,omitempty"`
	MessagesAnalyzed int             `json:"messages_analyzed"`
}

// UserStatsSummary provides aggregate stats across history.
type UserStatsSummary struct {
	TotalWeeks    int     `json:"total_weeks"`
	AvgMood       float64 `json:"avg_mood,omitempty"`
	MoodTrend     string  `json:"mood_trend,omitempty"`
	TotalMessages int     `json:"total_messages"`
}

// AvailableWeeksResponse is the response for GetAvailableWeeks.
type AvailableWeeksResponse struct {
	Weeks  []repository.WeekInfo `json:"weeks"`
	Latest string                `json:"latest"`
}

// GetUserCard retrieves a single user's card.
func (s *CardService) GetUserCard(
	ctx context.Context,
	userID int64,
	chatID int64,
	weekStart *time.Time,
) (*repository.UserCard, *repository.CardUser, error) {
	defer nrutil.StartSegment(ctx, "service:GetUserCard")()

	// Get card from repository
	card, err := s.cardRepo.GetUserCard(ctx, userID, chatID, weekStart)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, apperror.ErrCardNotFound
		}
		return nil, nil, err
	}

	// Get user info (optional - don't fail if not found)
	user, err := s.cardRepo.GetUserInfo(ctx, userID)
	if err != nil {
		// Return default user info if not found
		user = &repository.CardUser{ID: userID, FirstName: "Unknown"}
	}

	return card, user, nil
}

// GetChatCards retrieves all cards for a chat with sorting.
func (s *CardService) GetChatCards(
	ctx context.Context,
	req GetChatCardsRequest,
) (*GetChatCardsResponse, error) {
	defer nrutil.StartSegment(ctx, "service:GetChatCards")()

	// Validate sort field
	validSortFields := map[string]bool{
		"mood":      true,
		"influence": true,
		"activity":  true,
		"reactions": true,
	}

	if !validSortFields[req.SortBy] {
		req.SortBy = "mood"
	}

	sortDesc := req.Order != "asc"

	// Get cards from repository
	cards, total, err := s.cardRepo.GetChatCards(ctx, repository.ChatCardsQuery{
		ChatID:    req.ChatID,
		WeekStart: req.WeekStart,
		SortBy:    req.SortBy,
		SortDesc:  sortDesc,
		Limit:     req.Limit,
		Offset:    req.Offset,
	})
	if err != nil {
		return nil, err
	}

	// Get week bounds for metadata
	weekStart := ""
	weekEnd := ""
	if len(cards) > 0 {
		weekStart = cards[0].WeekStart
		weekEnd = cards[0].WeekEnd
	}

	return &GetChatCardsResponse{
		Cards: cards,
		Pagination: PaginationInfo{
			Total:   total,
			Limit:   req.Limit,
			Offset:  req.Offset,
			HasMore: req.Offset+len(cards) < total,
		},
		Metadata: ChatCardsMetadata{
			ChatID:    req.ChatID,
			WeekStart: weekStart,
			WeekEnd:   weekEnd,
			SortBy:    req.SortBy,
			Order:     req.Order,
		},
	}, nil
}

// GetUserHistory retrieves a user's card progression.
func (s *CardService) GetUserHistory(
	ctx context.Context,
	userID int64,
	chatID int64,
	limit int,
) (*UserHistoryResponse, error) {
	defer nrutil.StartSegment(ctx, "service:GetUserHistory")()

	// Get user info
	user, err := s.cardRepo.GetUserInfo(ctx, userID)
	if err != nil {
		user = &repository.CardUser{ID: userID, FirstName: "Unknown"}
	}

	// Get history from repository
	cards, err := s.cardRepo.GetUserHistory(ctx, userID, chatID, limit)
	if err != nil {
		return nil, err
	}

	// Convert to summaries
	history := make([]CardSummary, len(cards))
	for i, card := range cards {
		history[i] = CardSummary{
			WeekStart:        card.WeekStart,
			WeekEnd:          card.WeekEnd,
			Stats:            card.Stats,
			Trends:           card.Trends,
			MessagesAnalyzed: card.MessagesAnalyzed,
		}
	}

	// Calculate summary
	summary := s.calculateHistorySummary(cards)

	return &UserHistoryResponse{
		User:    *user,
		History: history,
		Summary: summary,
	}, nil
}

// GetAvailableWeeks returns weeks with generated cards.
func (s *CardService) GetAvailableWeeks(
	ctx context.Context,
	chatID int64,
) (*AvailableWeeksResponse, error) {
	weeks, err := s.cardRepo.GetAvailableWeeks(ctx, chatID)
	if err != nil {
		return nil, err
	}

	latest := ""
	if len(weeks) > 0 {
		latest = weeks[0].WeekStart
	}

	return &AvailableWeeksResponse{
		Weeks:  weeks,
		Latest: latest,
	}, nil
}

// calculateHistorySummary computes aggregate stats from card history.
func (s *CardService) calculateHistorySummary(cards []repository.UserCard) UserStatsSummary {
	if len(cards) == 0 {
		return UserStatsSummary{}
	}

	var totalMood float64
	var moodCount int
	var totalMessages int

	for _, card := range cards {
		totalMessages += card.MessagesAnalyzed

		// Parse stats to get mood score
		moodScore := extractMoodScore(card.Stats)
		if moodScore > 0 {
			totalMood += moodScore
			moodCount++
		}
	}

	avgMood := 0.0
	if moodCount > 0 {
		avgMood = totalMood / float64(moodCount)
	}

	// Determine mood trend (compare oldest to newest)
	moodTrend := "stable"
	if len(cards) >= 2 {
		newestMood := extractMoodScore(cards[0].Stats)
		oldestMood := extractMoodScore(cards[len(cards)-1].Stats)

		if newestMood-oldestMood > 5 {
			moodTrend = "improving"
		} else if oldestMood-newestMood > 5 {
			moodTrend = "declining"
		}
	}

	return UserStatsSummary{
		TotalWeeks:    len(cards),
		AvgMood:       avgMood,
		MoodTrend:     moodTrend,
		TotalMessages: totalMessages,
	}
}

// extractMoodScore attempts to get mood score from stats JSON.
func extractMoodScore(statsJSON json.RawMessage) float64 {
	var stats map[string]interface{}
	if err := jsonutil.Unmarshal(statsJSON, &stats); err != nil {
		return 0
	}

	mood, ok := stats["mood"].(map[string]interface{})
	if !ok {
		return 0
	}

	score, ok := mood["score"].(float64)
	if !ok {
		return 0
	}

	return score
}

// CardImageURLResponse is the response for GetCardImageURL.
type CardImageURLResponse struct {
	ImageID     int64  `json:"image_id"`
	URL         string `json:"url"`
	ExpiresIn   int    `json:"expires_in"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Theme       string `json:"theme"`
	WeekStart   string `json:"week_start"`
	GeneratedAt string `json:"generated_at"`
}

// GetCardImageURL retrieves a presigned URL for a user's card image.
func (s *CardService) GetCardImageURL(
	ctx context.Context,
	userID int64,
	chatID int64,
	weekStart *time.Time,
	theme string,
	expirySeconds int,
) (*CardImageURLResponse, error) {
	defer nrutil.StartSegment(ctx, "service:GetCardImageURL")()

	if expirySeconds <= 0 {
		expirySeconds = 3600 // Default 1 hour
	}

	// Get card image from repository
	cardImage, err := s.cardRepo.GetCardImage(ctx, userID, chatID, weekStart, theme)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.ErrCardImageNotFound
		}
		return nil, err
	}

	// Generate presigned URL
	if s.minioClient == nil {
		return nil, apperror.ErrStorageClientNotConfigured
	}

	url, err := s.minioClient.GetPresignedURLSeconds(ctx, cardImage.StoragePath, expirySeconds)
	if err != nil {
		return nil, err
	}

	return &CardImageURLResponse{
		ImageID:     cardImage.ID,
		URL:         url,
		ExpiresIn:   expirySeconds,
		Width:       cardImage.Width,
		Height:      cardImage.Height,
		Theme:       cardImage.Theme,
		WeekStart:   cardImage.WeekStart,
		GeneratedAt: cardImage.GeneratedAt.Format(time.RFC3339),
	}, nil
}

// GetCardImageURLString retrieves just the presigned URL for a user's card image (for shop dealer).
func (s *CardService) GetCardImageURLString(
	ctx context.Context,
	userID int64,
	chatID int64,
	weekStart *time.Time,
	theme string,
	expirySeconds int,
) (string, error) {
	resp, err := s.GetCardImageURL(ctx, userID, chatID, weekStart, theme, expirySeconds)
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", nil
	}
	return resp.URL, nil
}

// GalleryWeeksResponse is the response for GetGalleryWeeks.
type GalleryWeeksResponse struct {
	Weeks []string `json:"weeks"`
}

// GalleryImage represents a card image for the gallery API.
type GalleryImage struct {
	ID          int64   `json:"id"`
	UserID      int64   `json:"user_id"`
	ChatID      int64   `json:"chat_id"`
	WeekStart   string  `json:"week_start"`
	StoragePath string  `json:"storage_path"`
	Theme       string  `json:"theme"`
	GeneratedAt string  `json:"generated_at"`
	FirstName   *string `json:"first_name,omitempty"`
	LastName    *string `json:"last_name,omitempty"`
	Username    *string `json:"username,omitempty"`
}

// GalleryImagesResponse is the response for GetGalleryImages.
type GalleryImagesResponse struct {
	Images []GalleryImage `json:"images"`
}

// GalleryImageURLResponse is the response for GetGalleryImageURL.
type GalleryImageURLResponse struct {
	ImageID   int64  `json:"image_id"`
	URL       string `json:"url"`
	ExpiresIn int    `json:"expires_in"`
}

// GetGalleryWeeks returns weeks with generated card images for a chat.
func (s *CardService) GetGalleryWeeks(ctx context.Context, chatID int64) (*GalleryWeeksResponse, error) {
	defer nrutil.StartSegment(ctx, "service:GetGalleryWeeks")()

	weeks, err := s.cardRepo.GetGalleryWeeks(ctx, chatID)
	if err != nil {
		return nil, err
	}

	return &GalleryWeeksResponse{Weeks: weeks}, nil
}

// GetGalleryImages returns card images for a chat/week with user info.
func (s *CardService) GetGalleryImages(
	ctx context.Context,
	chatID int64,
	weekStart *time.Time,
	userID *int64,
	theme *string,
) (*GalleryImagesResponse, error) {
	defer nrutil.StartSegment(ctx, "service:GetGalleryImages")()

	images, err := s.cardRepo.GetGalleryImages(ctx, chatID, weekStart, userID, theme)
	if err != nil {
		return nil, err
	}

	// Convert repository types to service types
	result := make([]GalleryImage, len(images))
	for i, img := range images {
		result[i] = GalleryImage{
			ID:          img.ID,
			UserID:      img.UserID,
			ChatID:      img.ChatID,
			WeekStart:   img.WeekStart,
			StoragePath: img.StoragePath,
			Theme:       img.Theme,
			GeneratedAt: img.GeneratedAt.Format(time.RFC3339),
			FirstName:   img.FirstName,
			LastName:    img.LastName,
			Username:    img.Username,
		}
	}

	return &GalleryImagesResponse{Images: result}, nil
}

// GetGalleryImageURL retrieves a presigned URL for a gallery image by ID.
func (s *CardService) GetGalleryImageURL(
	ctx context.Context,
	imageID int64,
	expirySeconds int,
) (*GalleryImageURLResponse, error) {
	defer nrutil.StartSegment(ctx, "service:GetGalleryImageURL")()

	if expirySeconds <= 0 {
		expirySeconds = 3600 // Default 1 hour
	}
	if expirySeconds > 86400 {
		expirySeconds = 86400 // Max 24 hours
	}

	// Get image from repository
	image, err := s.cardRepo.GetGalleryImageByID(ctx, imageID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.ErrCardImageNotFound
		}
		return nil, err
	}

	// Generate presigned URL
	if s.minioClient == nil {
		return nil, apperror.ErrStorageClientNotConfigured
	}

	url, err := s.minioClient.GetPresignedURLSeconds(ctx, image.StoragePath, expirySeconds)
	if err != nil {
		return nil, err
	}

	return &GalleryImageURLResponse{
		ImageID:   imageID,
		URL:       url,
		ExpiresIn: expirySeconds,
	}, nil
}

// GetPlaceholderPositions returns the embedded placeholder positions JSON for a given theme.
// Currently only supports "neon_arcade" theme. Returns the JSON as json.RawMessage for direct use.
func (s *CardService) GetPlaceholderPositions(theme string) json.RawMessage {
	// Default to neon_arcade if theme is empty or not recognized
	if theme == "" || theme == "neon_arcade" {
		return json.RawMessage(embedded.NeonArcadeCompactPositions)
	}

	// For unsupported themes, return neon_arcade as default
	return json.RawMessage(embedded.NeonArcadeCompactPositions)
}
