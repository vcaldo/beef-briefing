package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"beef-briefing/apps/api-service/internal/middleware"
	"beef-briefing/apps/api-service/internal/repository"
	"beef-briefing/apps/api-service/internal/storage"

	"github.com/newrelic/go-agent/v3/newrelic"
)

// Telegram init data validation errors
var (
	ErrMissingHash     = errors.New("missing hash in init data")
	ErrInvalidHash     = errors.New("invalid hash")
	ErrExpiredInitData = errors.New("init data expired")
	ErrInvalidUserData = errors.New("invalid user data format")
	ErrMissingUserID   = errors.New("missing user ID in init data")
)

// InitDataUser represents user data from Telegram init data
type InitDataUser struct {
	ID        int64   `json:"id"`
	FirstName string  `json:"first_name"`
	LastName  *string `json:"last_name,omitempty"`
	Username  *string `json:"username,omitempty"`
}

// ValidatedInitData represents validated Telegram Mini App init data
type ValidatedInitData struct {
	UserID    int64
	ChatID    *int64
	FirstName string
	LastName  *string
	Username  *string
	AuthDate  int64
}

// AuthResponse represents the response from Mini App authentication
type AuthResponse struct {
	Token     string  `json:"token"`
	UserID    int64   `json:"user_id"`
	ChatID    *int64  `json:"chat_id,omitempty"`
	FirstName string  `json:"first_name"`
	Username  *string `json:"username,omitempty"`
}

// MiniAppService handles Mini App authentication and analytics
type MiniAppService struct {
	repo          *repository.MiniAppRepository
	jwtAuth       *middleware.JWTAuth
	botToken      string
	nrApp         *newrelic.Application
	storageClient *storage.MinIOClient
}

// NewMiniAppService creates a new MiniAppService
func NewMiniAppService(db *sql.DB, jwtSecretKey, botToken string, nrApp *newrelic.Application, storageClient *storage.MinIOClient) *MiniAppService {
	return &MiniAppService{
		repo:          repository.NewMiniAppRepository(db, nrApp),
		jwtAuth:       middleware.NewJWTAuth(jwtSecretKey),
		botToken:      botToken,
		nrApp:         nrApp,
		storageClient: storageClient,
	}
}

// generatePhotoURL creates a presigned URL for a photo object key
func (s *MiniAppService) generatePhotoURL(ctx context.Context, objectKey *string) *string {
	if objectKey == nil || *objectKey == "" || s.storageClient == nil {
		return nil
	}
	url, err := s.storageClient.GetPresignedURL(ctx, *objectKey, time.Hour)
	if err != nil {
		slog.Warn("failed to generate photo URL", "object_key", *objectKey, "error", err)
		return nil
	}
	return &url
}

// ValidateInitData validates Telegram Mini App init data
// Following Telegram documentation for WebApp init data validation
func (s *MiniAppService) ValidateInitData(initData string, maxAgeSeconds int64) (*ValidatedInitData, error) {
	// Parse query string
	params, err := url.ParseQuery(initData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse init data: %w", err)
	}

	// Extract and remove hash
	receivedHash := params.Get("hash")
	if receivedHash == "" {
		return nil, ErrMissingHash
	}
	params.Del("hash")

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
	secretKeyHMAC.Write([]byte(s.botToken))
	secretKey := secretKeyHMAC.Sum(nil)

	// Calculate expected hash: HMAC_SHA256(secret_key, data_check_string)
	hashHMAC := hmac.New(sha256.New, secretKey)
	hashHMAC.Write([]byte(dataCheckString))
	calculatedHash := hex.EncodeToString(hashHMAC.Sum(nil))

	// Constant-time comparison
	if !hmac.Equal([]byte(calculatedHash), []byte(receivedHash)) {
		return nil, ErrInvalidHash
	}

	// Validate auth_date
	authDateStr := params.Get("auth_date")
	authDate, err := strconv.ParseInt(authDateStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid auth_date: %w", err)
	}

	if time.Now().Unix()-authDate > maxAgeSeconds {
		return nil, ErrExpiredInitData
	}

	// Parse user object
	userStr := params.Get("user")
	if userStr == "" {
		userStr = "{}"
	}

	var userData InitDataUser
	if err := json.Unmarshal([]byte(userStr), &userData); err != nil {
		return nil, ErrInvalidUserData
	}

	if userData.ID == 0 {
		return nil, ErrMissingUserID
	}

	// Extract chat info if present
	var chatID *int64
	if chatStr := params.Get("chat"); chatStr != "" {
		var chatData struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal([]byte(chatStr), &chatData); err == nil && chatData.ID != 0 {
			chatID = &chatData.ID
		}
	}

	// Chat ID might also be in start_param (passed via URL)
	if chatID == nil {
		if startParam := params.Get("start_param"); startParam != "" {
			if id, err := strconv.ParseInt(startParam, 10, 64); err == nil {
				chatID = &id
			}
		}
	}

	return &ValidatedInitData{
		UserID:    userData.ID,
		ChatID:    chatID,
		FirstName: userData.FirstName,
		LastName:  userData.LastName,
		Username:  userData.Username,
		AuthDate:  authDate,
	}, nil
}

// Authenticate validates init data and returns a JWT token
func (s *MiniAppService) Authenticate(initData string) (*AuthResponse, error) {
	validated, err := s.ValidateInitData(initData, 86400) // 24 hours max age
	if err != nil {
		return nil, err
	}

	token, err := s.jwtAuth.CreateToken(
		validated.UserID,
		validated.ChatID,
		validated.Username,
		validated.FirstName,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWT token: %w", err)
	}

	slog.Info("Mini App auth successful",
		"user_id", validated.UserID,
		"chat_id", validated.ChatID,
	)

	return &AuthResponse{
		Token:     token,
		UserID:    validated.UserID,
		ChatID:    validated.ChatID,
		FirstName: validated.FirstName,
		Username:  validated.Username,
	}, nil
}

// GetOverviewStats returns overview statistics for a chat
func (s *MiniAppService) GetOverviewStats(ctx context.Context, chatID int64, period string) (*repository.OverviewStats, error) {
	startDate, endDate := getPeriodDates(period)
	return s.repo.GetOverviewStats(ctx, chatID, startDate, endDate)
}

// GetDailyActivity returns daily activity for a chat
func (s *MiniAppService) GetDailyActivity(ctx context.Context, chatID int64, period string) ([]repository.DailyActivity, error) {
	startDate, endDate := getPeriodDates(period)
	return s.repo.GetDailyActivity(ctx, chatID, startDate, endDate)
}

// UserRankingWithPhoto is a user ranking with photo URL
type UserRankingWithPhoto struct {
	Rank      int     `json:"rank"`
	UserID    int64   `json:"user_id"`
	FirstName string  `json:"first_name"`
	LastName  *string `json:"last_name,omitempty"`
	Username  *string `json:"username,omitempty"`
	Score     int64   `json:"score"`
	PhotoURL  *string `json:"photo_url,omitempty"`
}

// GetUserRankings returns user rankings for a chat
func (s *MiniAppService) GetUserRankings(ctx context.Context, chatID int64, metric, period string, page, limit int) ([]UserRankingWithPhoto, int, error) {
	startDate, endDate := getPeriodDates(period)
	offset := (page - 1) * limit

	rankings, err := s.repo.GetUserRankings(ctx, chatID, metric, limit, offset, startDate, endDate)
	if err != nil {
		return nil, 0, err
	}

	total, err := s.repo.GetUserRankingsTotal(ctx, chatID, startDate, endDate)
	if err != nil {
		return nil, 0, err
	}

	// Transform to response type with photo URLs
	result := make([]UserRankingWithPhoto, len(rankings))
	for i, r := range rankings {
		result[i] = UserRankingWithPhoto{
			Rank:      r.Rank,
			UserID:    r.UserID,
			FirstName: r.FirstName,
			LastName:  r.LastName,
			Username:  r.Username,
			Score:     r.Score,
			PhotoURL:  s.generatePhotoURL(ctx, r.PhotoObjectKey),
		}
	}

	return result, total, nil
}

// getPeriodDates returns start and end dates for a period
func getPeriodDates(period string) (*time.Time, *time.Time) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	endDate := today.Add(24 * time.Hour) // Include today

	switch period {
	case "max":
		return nil, nil
	case "ytd":
		startDate := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
		return &startDate, &endDate
	case "24h":
		startDate := today.Add(-1 * 24 * time.Hour)
		return &startDate, &endDate
	case "7d":
		startDate := today.Add(-7 * 24 * time.Hour)
		return &startDate, &endDate
	case "30d":
		startDate := today.Add(-30 * 24 * time.Hour)
		return &startDate, &endDate
	case "90d":
		startDate := today.Add(-90 * 24 * time.Hour)
		return &startDate, &endDate
	case "180d":
		startDate := today.Add(-180 * 24 * time.Hour)
		return &startDate, &endDate
	case "365d":
		startDate := today.Add(-365 * 24 * time.Hour)
		return &startDate, &endDate
	default:
		// Default to 30 days
		startDate := today.Add(-30 * 24 * time.Hour)
		return &startDate, &endDate
	}
}

// ReactionUserWithPhoto is a reaction user with photo URL
type ReactionUserWithPhoto struct {
	Rank      int     `json:"rank"`
	UserID    int64   `json:"user_id"`
	FirstName string  `json:"first_name"`
	LastName  *string `json:"last_name,omitempty"`
	Username  *string `json:"username,omitempty"`
	Score     int64   `json:"score"`
	PhotoURL  *string `json:"photo_url,omitempty"`
}

// TopInteractorWithPhoto is a top interactor with photo URL
type TopInteractorWithPhoto struct {
	Rank      int     `json:"rank"`
	UserID    int64   `json:"user_id"`
	FirstName string  `json:"first_name"`
	LastName  *string `json:"last_name,omitempty"`
	Username  *string `json:"username,omitempty"`
	Score     int64   `json:"score"`
	TopEmoji  *string `json:"top_emoji,omitempty"`
	PhotoURL  *string `json:"photo_url,omitempty"`
}

// ReactionsOverviewResponse represents reactions overview data
type ReactionsOverviewResponse struct {
	TopReactions []repository.TopReaction `json:"top_reactions"`
	TopGivers    []ReactionUserWithPhoto  `json:"top_givers"`
	TopReceivers []ReactionUserWithPhoto  `json:"top_receivers"`
}

// GetReactionsOverview returns top reactions, givers, and receivers for a chat
func (s *MiniAppService) GetReactionsOverview(ctx context.Context, chatID int64, period string, limit int) (*ReactionsOverviewResponse, error) {
	startDate, endDate := getPeriodDates(period)

	topReactions, err := s.repo.GetTopReactions(ctx, chatID, limit, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get top reactions: %w", err)
	}

	topGivers, err := s.repo.GetTopReactionGivers(ctx, chatID, limit, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get top givers: %w", err)
	}

	topReceivers, err := s.repo.GetTopReactionReceivers(ctx, chatID, limit, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get top receivers: %w", err)
	}

	// Ensure non-nil slices for JSON
	if topReactions == nil {
		topReactions = []repository.TopReaction{}
	}

	// Transform givers to response type with photo URLs
	giversWithPhoto := make([]ReactionUserWithPhoto, len(topGivers))
	for i, g := range topGivers {
		giversWithPhoto[i] = ReactionUserWithPhoto{
			Rank:      g.Rank,
			UserID:    g.UserID,
			FirstName: g.FirstName,
			LastName:  g.LastName,
			Username:  g.Username,
			Score:     g.Score,
			PhotoURL:  s.generatePhotoURL(ctx, g.PhotoObjectKey),
		}
	}

	// Transform receivers to response type with photo URLs
	receiversWithPhoto := make([]ReactionUserWithPhoto, len(topReceivers))
	for i, r := range topReceivers {
		receiversWithPhoto[i] = ReactionUserWithPhoto{
			Rank:      r.Rank,
			UserID:    r.UserID,
			FirstName: r.FirstName,
			LastName:  r.LastName,
			Username:  r.Username,
			Score:     r.Score,
			PhotoURL:  s.generatePhotoURL(ctx, r.PhotoObjectKey),
		}
	}

	return &ReactionsOverviewResponse{
		TopReactions: topReactions,
		TopGivers:    giversWithPhoto,
		TopReceivers: receiversWithPhoto,
	}, nil
}

// ProfileResponse represents user profile data
type ProfileResponse struct {
	PhotoURL     *string                  `json:"photo_url,omitempty"`
	Stats        *repository.ProfileStats `json:"stats"`
	TopReactors  []TopInteractorWithPhoto `json:"top_reactors"`
	TopRepliers  []TopInteractorWithPhoto `json:"top_repliers"`
	TopRepliedTo []TopInteractorWithPhoto `json:"top_replied_to"`
	Heatmap      *repository.HeatmapData  `json:"heatmap"`
}

// GetUserProfile returns personal stats and top interactors for a user
func (s *MiniAppService) GetUserProfile(ctx context.Context, chatID, userID int64, period string) (*ProfileResponse, error) {
	startDate, endDate := getPeriodDates(period)

	stats, err := s.repo.GetUserProfileStats(ctx, chatID, userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile stats: %w", err)
	}

	topReactors, err := s.repo.GetTopReactorsToUser(ctx, chatID, userID, 5, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get top reactors: %w", err)
	}

	topRepliers, err := s.repo.GetTopRepliersToUser(ctx, chatID, userID, 5, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get top repliers: %w", err)
	}

	topRepliedTo, err := s.repo.GetTopRepliedToByUser(ctx, chatID, userID, 5, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get top replied to: %w", err)
	}

	heatmap, err := s.repo.GetUserHeatmap(ctx, chatID, userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get user heatmap: %w", err)
	}

	// Get current user's photo
	userPhotoKey, err := s.repo.GetUserPhotoObjectKey(ctx, userID)
	if err != nil {
		slog.Warn("failed to get user photo key", "user_id", userID, "error", err)
	}
	userPhotoURL := s.generatePhotoURL(ctx, userPhotoKey)

	// Transform top reactors to response type with photo URLs
	reactorsWithPhoto := make([]TopInteractorWithPhoto, len(topReactors))
	for i, r := range topReactors {
		reactorsWithPhoto[i] = TopInteractorWithPhoto{
			Rank:      r.Rank,
			UserID:    r.UserID,
			FirstName: r.FirstName,
			LastName:  r.LastName,
			Username:  r.Username,
			Score:     r.Score,
			TopEmoji:  r.TopEmoji,
			PhotoURL:  s.generatePhotoURL(ctx, r.PhotoObjectKey),
		}
	}

	// Transform top repliers to response type with photo URLs
	repliersWithPhoto := make([]TopInteractorWithPhoto, len(topRepliers))
	for i, r := range topRepliers {
		repliersWithPhoto[i] = TopInteractorWithPhoto{
			Rank:      r.Rank,
			UserID:    r.UserID,
			FirstName: r.FirstName,
			LastName:  r.LastName,
			Username:  r.Username,
			Score:     r.Score,
			PhotoURL:  s.generatePhotoURL(ctx, r.PhotoObjectKey),
		}
	}

	// Transform top replied to to response type with photo URLs
	repliedToWithPhoto := make([]TopInteractorWithPhoto, len(topRepliedTo))
	for i, r := range topRepliedTo {
		repliedToWithPhoto[i] = TopInteractorWithPhoto{
			Rank:      r.Rank,
			UserID:    r.UserID,
			FirstName: r.FirstName,
			LastName:  r.LastName,
			Username:  r.Username,
			Score:     r.Score,
			PhotoURL:  s.generatePhotoURL(ctx, r.PhotoObjectKey),
		}
	}

	return &ProfileResponse{
		PhotoURL:     userPhotoURL,
		Stats:        stats,
		TopReactors:  reactorsWithPhoto,
		TopRepliers:  repliersWithPhoto,
		TopRepliedTo: repliedToWithPhoto,
		Heatmap:      heatmap,
	}, nil
}

// HeatmapResponse represents heatmap data
type HeatmapResponse struct {
	Group *repository.HeatmapData `json:"group"`
	User  *repository.HeatmapData `json:"user,omitempty"`
}

// GetHeatmapData returns group and optionally user heatmap data
func (s *MiniAppService) GetHeatmapData(ctx context.Context, chatID int64, userID *int64, period string) (*HeatmapResponse, error) {
	// Group heatmap always uses all-time data from MV
	groupHeatmap, err := s.repo.GetGroupHeatmap(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group heatmap: %w", err)
	}

	response := &HeatmapResponse{
		Group: groupHeatmap,
	}

	// User heatmap if requested
	if userID != nil {
		startDate, endDate := getPeriodDates(period)
		userHeatmap, err := s.repo.GetUserHeatmap(ctx, chatID, *userID, startDate, endDate)
		if err != nil {
			return nil, fmt.Errorf("failed to get user heatmap: %w", err)
		}
		response.User = userHeatmap
	}

	return response, nil
}
