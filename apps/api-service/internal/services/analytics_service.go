package services

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"beef-briefing/apps/api-service/internal/models"
	"beef-briefing/apps/api-service/internal/repository"

	"github.com/newrelic/go-agent/v3/newrelic"
)

type AnalyticsService struct {
	db            *sql.DB
	nrApp         *newrelic.Application
	analyticsRepo *repository.AnalyticsRepository
}

func NewAnalyticsService(db *sql.DB, nrApp *newrelic.Application) *AnalyticsService {
	return &AnalyticsService{
		db:            db,
		nrApp:         nrApp,
		analyticsRepo: repository.NewAnalyticsRepository(db, nrApp),
	}
}

// GetOverview returns chat overview statistics
func (s *AnalyticsService) GetOverview(ctx context.Context, chatID int64, startDate, endDate time.Time) (*models.OverviewResponse, error) {
	overview, err := s.analyticsRepo.GetOverviewStats(ctx, chatID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get overview stats: %w", err)
	}

	// Get most active user
	mostActive, err := s.analyticsRepo.GetMostActiveUser(ctx, chatID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get most active user: %w", err)
	}
	overview.MostActiveUser = mostActive

	// Get top emojis
	topEmojis, err := s.analyticsRepo.GetTopEmojis(ctx, chatID, startDate, endDate, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to get top emojis: %w", err)
	}
	overview.TopEmojis = topEmojis

	// Get media breakdown
	mediaBreakdown, err := s.analyticsRepo.GetMediaTypeBreakdown(ctx, chatID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get media breakdown: %w", err)
	}
	overview.MediaTypeBreakdown = mediaBreakdown

	return overview, nil
}

// GetLeaderboard returns user rankings
func (s *AnalyticsService) GetLeaderboard(ctx context.Context, chatID int64, startDate, endDate time.Time, metric string, limit int) ([]models.LeaderboardEntry, error) {
	// Validate metric
	validMetrics := map[string]bool{
		"messages":           true,
		"reactions_given":    true,
		"reactions_received": true,
		"media_sent":         true,
	}
	if !validMetrics[metric] {
		return nil, fmt.Errorf("invalid metric: %s", metric)
	}

	// Enforce limit bounds
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	return s.analyticsRepo.GetLeaderboard(ctx, chatID, startDate, endDate, metric, limit)
}

// GetUserDetail returns detailed user statistics with streaks
func (s *AnalyticsService) GetUserDetail(ctx context.Context, chatID, userID int64, startDate, endDate time.Time) (*models.UserDetailResponse, error) {
	// Get user info
	userInfo, err := s.analyticsRepo.GetUserInfo(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	// Get basic stats
	stats, err := s.analyticsRepo.GetUserDetailStats(ctx, chatID, userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get user stats: %w", err)
	}

	// Get message days for streak calculation
	messageDays, err := s.analyticsRepo.GetUserMessageDays(ctx, chatID, userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get message days: %w", err)
	}

	streaks := calculateStreaks(messageDays, endDate)

	// Get top emojis used
	topEmojisUsed, err := s.analyticsRepo.GetUserTopEmojis(ctx, chatID, userID, startDate, endDate, 10, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get top emojis used: %w", err)
	}

	// Get top emojis received
	topEmojisReceived, err := s.analyticsRepo.GetUserTopEmojis(ctx, chatID, userID, startDate, endDate, 10, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get top emojis received: %w", err)
	}

	// Get activity by hour
	activityByHour, err := s.analyticsRepo.GetUserActivityByHour(ctx, chatID, userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get activity by hour: %w", err)
	}

	return &models.UserDetailResponse{
		UserSummary:       *userInfo,
		Stats:             *stats,
		Streaks:           streaks,
		TopEmojisUsed:     topEmojisUsed,
		TopEmojisReceived: topEmojisReceived,
		ActivityByHour:    activityByHour,
	}, nil
}

// calculateStreaks calculates current and longest streaks from message days
func calculateStreaks(days []time.Time, endDate time.Time) models.StreakInfo {
	if len(days) == 0 {
		return models.StreakInfo{
			Current: models.StreakDetail{Days: 0},
			Longest: models.StreakDetail{Days: 0},
		}
	}

	var current, longest models.StreakDetail
	var tempStreak models.StreakDetail

	for i := 0; i < len(days); i++ {
		if i == 0 {
			tempStreak = models.StreakDetail{
				Days:      1,
				StartDate: days[i],
				EndDate:   days[i],
			}
		} else {
			// Check if consecutive day (difference <= 1 day)
			daysDiff := days[i].Sub(days[i-1]).Hours() / 24
			if daysDiff <= 1.5 { // Allow some tolerance for timezone differences
				tempStreak.Days++
				tempStreak.EndDate = days[i]
			} else {
				// Streak broken, check if it's the longest
				if tempStreak.Days > longest.Days {
					longest = tempStreak
				}
				// Start new streak
				tempStreak = models.StreakDetail{
					Days:      1,
					StartDate: days[i],
					EndDate:   days[i],
				}
			}
		}
	}

	// Check final streak
	if tempStreak.Days > longest.Days {
		longest = tempStreak
	}

	// Current streak is only valid if it extends to today/endDate
	if len(days) > 0 {
		lastDay := days[len(days)-1]
		daysSinceLastMessage := endDate.Sub(lastDay).Hours() / 24
		if daysSinceLastMessage <= 1.5 { // Allow tolerance
			current = tempStreak
		} else {
			current = models.StreakDetail{Days: 0}
		}
	}

	return models.StreakInfo{
		Current: current,
		Longest: longest,
	}
}

// GetTimeline returns activity timeline
func (s *AnalyticsService) GetTimeline(ctx context.Context, chatID int64, startDate, endDate time.Time, granularity string) ([]models.TimelinePoint, error) {
	validGranularities := map[string]bool{
		"hour":  true,
		"day":   true,
		"week":  true,
		"month": true,
	}
	if !validGranularities[granularity] {
		return nil, fmt.Errorf("invalid granularity: %s", granularity)
	}

	return s.analyticsRepo.GetTimeline(ctx, chatID, startDate, endDate, granularity)
}

// GetHeatmap returns daily activity heatmap
func (s *AnalyticsService) GetHeatmap(ctx context.Context, chatID int64, startDate, endDate time.Time) ([]models.HeatmapDay, error) {
	return s.analyticsRepo.GetHeatmapData(ctx, chatID, startDate, endDate)
}

// GetTopContent returns most reacted/replied messages
func (s *AnalyticsService) GetTopContent(ctx context.Context, chatID int64, startDate, endDate time.Time, metric string, limit int) ([]models.TopMessage, error) {
	validMetrics := map[string]bool{
		"most_reacted": true,
		"most_replied": true,
	}
	if !validMetrics[metric] {
		return nil, fmt.Errorf("invalid metric: %s", metric)
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	messages, err := s.analyticsRepo.GetTopMessages(ctx, chatID, startDate, endDate, metric, limit)
	if err != nil {
		return nil, err
	}

	// Enrich with top reactions for each message
	for i := range messages {
		reactions, err := s.analyticsRepo.GetMessageTopReactions(ctx, chatID, messages[i].TelegramMessageID, 3)
		if err == nil {
			messages[i].TopReactions = reactions
		}
	}

	return messages, nil
}

// CompareUsers returns comparison of multiple users
func (s *AnalyticsService) CompareUsers(ctx context.Context, chatID int64, userIDs []int64, startDate, endDate time.Time) ([]models.UserComparison, error) {
	if len(userIDs) == 0 {
		return nil, fmt.Errorf("at least one user ID required")
	}
	if len(userIDs) > 5 {
		return nil, fmt.Errorf("maximum 5 users can be compared")
	}

	return s.analyticsRepo.GetUserComparisons(ctx, chatID, userIDs, startDate, endDate)
}

// ListChats returns all chats with summary statistics (no time range required)
func (s *AnalyticsService) ListChats(ctx context.Context) ([]models.ChatSummary, error) {
	return s.analyticsRepo.ListChats(ctx)
}

// GetChat returns detailed information about a single chat (no time range required)
func (s *AnalyticsService) GetChat(ctx context.Context, chatID int64) (*models.ChatDetail, error) {
	return s.analyticsRepo.GetChat(ctx, chatID)
}
