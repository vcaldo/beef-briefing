package models

import (
	"fmt"
	"time"
)

// ============================================
// REQUEST MODELS
// ============================================

// TimeRangeRequest is embedded in all analytics requests
type TimeRangeRequest struct {
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
}

// Validate ensures time range is valid and not too large
func (t *TimeRangeRequest) Validate() error {
	if t.StartDate.IsZero() || t.EndDate.IsZero() {
		return fmt.Errorf("start_date and end_date are required")
	}
	if t.EndDate.Before(t.StartDate) {
		return fmt.Errorf("end_date must be after start_date")
	}
	// Max 1 year range
	if t.EndDate.Sub(t.StartDate) > 365*24*time.Hour {
		return fmt.Errorf("time range cannot exceed 1 year")
	}
	return nil
}

// ============================================
// RESPONSE MODELS
// ============================================

// StandardResponse wraps all API responses
type StandardResponse struct {
	Data     interface{} `json:"data"`
	Metadata Metadata    `json:"metadata"`
}

type Metadata struct {
	ChatID    int64     `json:"chat_id"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	Generated time.Time `json:"generated_at"`
}

// Overview response
type OverviewResponse struct {
	TotalMessages      int                    `json:"total_messages"`
	TotalUsers         int                    `json:"total_users"`
	TotalReactions     int                    `json:"total_reactions"`
	TotalMedia         int                    `json:"total_media"`
	MessagesPerDay     float64                `json:"messages_per_day"`
	MostActiveUser     *UserSummary           `json:"most_active_user"`
	TopEmojis          []EmojiBreakdown       `json:"top_emojis"`
	MediaTypeBreakdown map[string]int         `json:"media_type_breakdown"`
}

type UserSummary struct {
	UserID       int64  `json:"user_id"`
	Username     string `json:"username,omitempty"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name,omitempty"`
	MessageCount int    `json:"message_count,omitempty"`
}

type EmojiBreakdown struct {
	Emoji string `json:"emoji"`
	Count int    `json:"count"`
}

// Leaderboard response
type LeaderboardEntry struct {
	UserSummary
	Score int `json:"score"`
	Rank  int `json:"rank"`
}

// User detail response
type UserDetailResponse struct {
	UserSummary
	Stats              UserDetailStats  `json:"stats"`
	Streaks            StreakInfo       `json:"streaks"`
	TopEmojisUsed      []EmojiBreakdown `json:"top_emojis_used"`
	TopEmojisReceived  []EmojiBreakdown `json:"top_emojis_received"`
	ActivityByHour     map[int]int      `json:"activity_by_hour"` // Hour (0-23) -> count
}

type UserDetailStats struct {
	TotalMessages     int     `json:"total_messages"`
	ReactionsGiven    int     `json:"reactions_given"`
	ReactionsReceived int     `json:"reactions_received"`
	MediaSent         int     `json:"media_sent"`
	RepliesReceived   int     `json:"replies_received"`
	AvgMessageLength  float64 `json:"avg_message_length"`
	MediaPercentage   float64 `json:"media_percentage"`
}

type StreakInfo struct {
	Current StreakDetail `json:"current"`
	Longest StreakDetail `json:"longest"`
}

type StreakDetail struct {
	Days      int       `json:"days"`
	StartDate time.Time `json:"start_date,omitempty"`
	EndDate   time.Time `json:"end_date,omitempty"`
}

// Timeline response
type TimelinePoint struct {
	Timestamp time.Time `json:"timestamp"`
	Messages  int       `json:"messages"`
	Users     int       `json:"active_users"`
	Reactions int       `json:"reactions"`
}

// Heatmap response
type HeatmapDay struct {
	Date         string `json:"date"` // YYYY-MM-DD
	MessageCount int    `json:"message_count"`
	Level        int    `json:"level"` // 0-4 for visualization
}

// Top content response
type TopMessage struct {
	MessageID         int64            `json:"message_id"`
	TelegramMessageID int64            `json:"telegram_message_id"`
	UserSummary
	Date         time.Time        `json:"date"`
	Text         string           `json:"text,omitempty"`
	Caption      string           `json:"caption,omitempty"`
	Score        int              `json:"score"` // Reaction count or reply count
	TopReactions []EmojiBreakdown `json:"top_reactions,omitempty"`
}

// Compare response
type UserComparison struct {
	UserSummary
	Messages          int     `json:"messages"`
	ReactionsGiven    int     `json:"reactions_given"`
	ReactionsReceived int     `json:"reactions_received"`
	MediaSent         int     `json:"media_sent"`
	AvgMessageLength  float64 `json:"avg_message_length"`
}

// Chat listing response (no time range required)
type ChatSummary struct {
	ID           int64     `json:"id"`
	Title        string    `json:"title"`
	Type         string    `json:"type"`
	Username     string    `json:"username,omitempty"`
	MessageCount int       `json:"message_count"`
	UserCount    int       `json:"user_count"`
	LastActivity time.Time `json:"last_activity"`
}

// Chat detail response (no time range required)
type ChatDetail struct {
	ID           int64     `json:"id"`
	Title        string    `json:"title"`
	Type         string    `json:"type"`
	Username     string    `json:"username,omitempty"`
	FirstName    string    `json:"first_name,omitempty"`
	LastName     string    `json:"last_name,omitempty"`
	MessageCount int       `json:"message_count"`
	UserCount    int       `json:"user_count"`
	MediaCount   int       `json:"media_count"`
	FirstMessage time.Time `json:"first_message"`
	LastMessage  time.Time `json:"last_message"`
}
