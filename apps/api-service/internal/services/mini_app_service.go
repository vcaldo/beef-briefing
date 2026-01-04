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
	repo     *repository.MiniAppRepository
	jwtAuth  *middleware.JWTAuth
	botToken string
	nrApp    *newrelic.Application
}

// NewMiniAppService creates a new MiniAppService
func NewMiniAppService(db *sql.DB, jwtSecretKey, botToken string, nrApp *newrelic.Application) *MiniAppService {
	return &MiniAppService{
		repo:     repository.NewMiniAppRepository(db, nrApp),
		jwtAuth:  middleware.NewJWTAuth(jwtSecretKey),
		botToken: botToken,
		nrApp:    nrApp,
	}
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

// GetUserRankings returns user rankings for a chat
func (s *MiniAppService) GetUserRankings(ctx context.Context, chatID int64, metric, period string, page, limit int) ([]repository.UserRanking, int, error) {
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

	return rankings, total, nil
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
