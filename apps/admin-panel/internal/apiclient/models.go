package apiclient

import "time"

// StandardResponse wraps all API responses
type StandardResponse struct {
	Data     interface{} `json:"data"`
	Metadata Metadata    `json:"metadata"`
}

type Metadata struct {
	ChatID     int64     `json:"chat_id,omitempty"`
	StartDate  time.Time `json:"start_date,omitempty"`
	EndDate    time.Time `json:"end_date,omitempty"`
	Generated  time.Time `json:"generated_at"`
	TotalCount int       `json:"total_count,omitempty"`
}

// ChatSummary for list view
type ChatSummary struct {
	ID           int64     `json:"id"`
	Title        string    `json:"title"`
	Type         string    `json:"type"`
	Username     string    `json:"username,omitempty"`
	MessageCount int       `json:"message_count"`
	UserCount    int       `json:"user_count"`
	LastActivity time.Time `json:"last_activity"`
}

// ChatDetail for single chat view
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

// Overview response
type OverviewResponse struct {
	TotalMessages  int              `json:"total_messages"`
	TotalUsers     int              `json:"total_users"`
	TotalReactions int              `json:"total_reactions"`
	TotalMedia     int              `json:"total_media"`
	MessagesPerDay float64          `json:"messages_per_day"`
	MostActiveUser *UserSummary     `json:"most_active_user"`
	TopEmojis      []EmojiBreakdown `json:"top_emojis"`
}

type UserSummary struct {
	UserID       int64  `json:"user_id"`
	Username     string `json:"username,omitempty"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name,omitempty"`
	MessageCount int    `json:"message_count"`
}

type EmojiBreakdown struct {
	Emoji string `json:"emoji"`
	Count int    `json:"count"`
}

// Leaderboard response
type LeaderboardEntry struct {
	Rank      int    `json:"rank"`
	UserID    int64  `json:"user_id"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Score     int    `json:"score"`
}

// User detail response
type UserDetailResponse struct {
	UserID            int64            `json:"user_id"`
	Username          string           `json:"username,omitempty"`
	FirstName         string           `json:"first_name"`
	LastName          string           `json:"last_name,omitempty"`
	Stats             UserDetailStats  `json:"stats"`
	CurrentStreak     *StreakInfo      `json:"current_streak,omitempty"`
	LongestStreak     *StreakInfo      `json:"longest_streak,omitempty"`
	TopEmojisUsed     []EmojiBreakdown `json:"top_emojis_used"`
	TopEmojisReceived []EmojiBreakdown `json:"top_emojis_received"`
	ActivityByHour    map[string]int   `json:"activity_by_hour"`
}

type UserDetailStats struct {
	TotalMessages     int       `json:"total_messages"`
	ReactionsGiven    int       `json:"reactions_given"`
	ReactionsReceived int       `json:"reactions_received"`
	MediaSent         int       `json:"media_sent"`
	RepliesSent       int       `json:"replies_sent"`
	RepliesReceived   int       `json:"replies_received"`
	AvgMessageLength  float64   `json:"avg_message_length"`
	FirstActive       time.Time `json:"first_active,omitempty"`
	LastActive        time.Time `json:"last_active,omitempty"`
}

type StreakInfo struct {
	Days      int       `json:"days"`
	StartDate time.Time `json:"start_date,omitempty"`
	EndDate   time.Time `json:"end_date,omitempty"`
}

// Timeline response
type TimelinePoint struct {
	Period        string `json:"period"`
	MessageCount  int    `json:"message_count"`
	UserCount     int    `json:"user_count"`
	ReactionCount int    `json:"reaction_count"`
}

// Heatmap response
type HeatmapDay struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// Top content response
type TopMessage struct {
	MessageID    int64            `json:"message_id"`
	UserID       int64            `json:"user_id"`
	Username     string           `json:"username,omitempty"`
	FirstName    string           `json:"first_name"`
	LastName     string           `json:"last_name,omitempty"`
	Date         time.Time        `json:"date"`
	Text         string           `json:"text,omitempty"`
	Score        int              `json:"score"`
	TopReactions []EmojiBreakdown `json:"top_reactions,omitempty"`
}

// Compare response
type UserComparison struct {
	UserID            int64   `json:"user_id"`
	Username          string  `json:"username,omitempty"`
	FirstName         string  `json:"first_name"`
	LastName          string  `json:"last_name,omitempty"`
	MessageCount      int     `json:"message_count"`
	ReactionsGiven    int     `json:"reactions_given"`
	ReactionsReceived int     `json:"reactions_received"`
	MediaSent         int     `json:"media_sent"`
	AvgMessageLength  float64 `json:"avg_message_length"`
}
