package models

import (
	"database/sql"
	"time"
)

// ReactionType represents the type of reaction
type ReactionType string

const (
	ReactionTypeEmoji       ReactionType = "emoji"
	ReactionTypeCustomEmoji ReactionType = "custom_emoji"
	ReactionTypePaid        ReactionType = "paid"
)

// Update represents a Telegram update
type Update struct {
	UpdateID             int64                       `json:"update_id"`
	Message              *Message                    `json:"message,omitempty"`
	EditedMessage        *Message                    `json:"edited_message,omitempty"`
	MessageReaction      *MessageReactionUpdated     `json:"message_reaction,omitempty"`
	MessageReactionCount *MessageReactionCountUpdate `json:"message_reaction_count,omitempty"`
}

// Chat represents a Telegram chat
type Chat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title,omitempty"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	IsForum   bool   `json:"is_forum,omitempty"`
}

// User represents a Telegram user
type User struct {
	ID           int64  `json:"id"`
	IsBot        bool   `json:"is_bot"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name,omitempty"`
	Username     string `json:"username,omitempty"`
	LanguageCode string `json:"language_code,omitempty"`
	IsPremium    bool   `json:"is_premium,omitempty"`
}

// Message represents a Telegram message
type Message struct {
	MessageID           int64           `json:"message_id"`
	MessageThreadID     *int64          `json:"message_thread_id,omitempty"`
	From                *User           `json:"from,omitempty"`
	Chat                Chat            `json:"chat"`
	Date                int64           `json:"date"`
	EditDate            *int64          `json:"edit_date,omitempty"`
	Text                string          `json:"text,omitempty"`
	Caption             string          `json:"caption,omitempty"`
	Entities            []MessageEntity `json:"entities,omitempty"`
	CaptionEntities     []MessageEntity `json:"caption_entities,omitempty"`
	ReplyToMessage      *Message        `json:"reply_to_message,omitempty"`
	MediaGroupID        string          `json:"media_group_id,omitempty"`
	Photo               []PhotoSize     `json:"photo,omitempty"`
	Video               *Video          `json:"video,omitempty"`
	Audio               *Audio          `json:"audio,omitempty"`
	Voice               *Voice          `json:"voice,omitempty"`
	Document            *Document       `json:"document,omitempty"`
	Animation           *Animation      `json:"animation,omitempty"`
	VideoNote           *VideoNote      `json:"video_note,omitempty"`
	Location            *Location       `json:"location,omitempty"`
	HasProtectedContent bool            `json:"has_protected_content,omitempty"`
	IsAutomaticForward  bool            `json:"is_automatic_forward,omitempty"`
	IsTopicMessage      bool            `json:"is_topic_message,omitempty"`
}

// MessageEntity represents a special entity in a text message
type MessageEntity struct {
	Type          string `json:"type"`
	Offset        int    `json:"offset"`
	Length        int    `json:"length"`
	URL           string `json:"url,omitempty"`
	User          *User  `json:"user,omitempty"`
	Language      string `json:"language,omitempty"`
	CustomEmojiID string `json:"custom_emoji_id,omitempty"`
}

// PhotoSize represents one size of a photo
type PhotoSize struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	FileSize     *int64 `json:"file_size,omitempty"`
}

// Video represents a video file
type Video struct {
	FileID       string     `json:"file_id"`
	FileUniqueID string     `json:"file_unique_id"`
	Width        int        `json:"width"`
	Height       int        `json:"height"`
	Duration     int        `json:"duration"`
	Thumbnail    *PhotoSize `json:"thumbnail,omitempty"`
	FileName     string     `json:"file_name,omitempty"`
	MimeType     string     `json:"mime_type,omitempty"`
	FileSize     *int64     `json:"file_size,omitempty"`
}

// Audio represents an audio file
type Audio struct {
	FileID       string     `json:"file_id"`
	FileUniqueID string     `json:"file_unique_id"`
	Duration     int        `json:"duration"`
	Performer    string     `json:"performer,omitempty"`
	Title        string     `json:"title,omitempty"`
	FileName     string     `json:"file_name,omitempty"`
	MimeType     string     `json:"mime_type,omitempty"`
	FileSize     *int64     `json:"file_size,omitempty"`
	Thumbnail    *PhotoSize `json:"thumbnail,omitempty"`
}

// Voice represents a voice note
type Voice struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Duration     int    `json:"duration"`
	MimeType     string `json:"mime_type,omitempty"`
	FileSize     *int64 `json:"file_size,omitempty"`
}

// Document represents a general file
type Document struct {
	FileID       string     `json:"file_id"`
	FileUniqueID string     `json:"file_unique_id"`
	Thumbnail    *PhotoSize `json:"thumbnail,omitempty"`
	FileName     string     `json:"file_name,omitempty"`
	MimeType     string     `json:"mime_type,omitempty"`
	FileSize     *int64     `json:"file_size,omitempty"`
}

// Animation represents an animation file (GIF or H.264/MPEG-4 AVC video without sound)
type Animation struct {
	FileID       string     `json:"file_id"`
	FileUniqueID string     `json:"file_unique_id"`
	Width        int        `json:"width"`
	Height       int        `json:"height"`
	Duration     int        `json:"duration"`
	Thumbnail    *PhotoSize `json:"thumbnail,omitempty"`
	FileName     string     `json:"file_name,omitempty"`
	MimeType     string     `json:"mime_type,omitempty"`
	FileSize     *int64     `json:"file_size,omitempty"`
}

// VideoNote represents a video message
type VideoNote struct {
	FileID       string     `json:"file_id"`
	FileUniqueID string     `json:"file_unique_id"`
	Length       int        `json:"length"`
	Duration     int        `json:"duration"`
	Thumbnail    *PhotoSize `json:"thumbnail,omitempty"`
	FileSize     *int64     `json:"file_size,omitempty"`
}

// Location represents a point on the map
type Location struct {
	Latitude             float64  `json:"latitude"`
	Longitude            float64  `json:"longitude"`
	HorizontalAccuracy   *float64 `json:"horizontal_accuracy,omitempty"`
	LivePeriod           *int     `json:"live_period,omitempty"`
	Heading              *int     `json:"heading,omitempty"`
	ProximityAlertRadius *int     `json:"proximity_alert_radius,omitempty"`
}

// MessageReactionUpdated represents a change of reaction on a message performed by a user
type MessageReactionUpdated struct {
	Chat        Chat           `json:"chat"`
	MessageID   int64          `json:"message_id"`
	User        *User          `json:"user,omitempty"`
	ActorChat   *Chat          `json:"actor_chat,omitempty"`
	Date        int64          `json:"date"`
	OldReaction []ReactionInfo `json:"old_reaction"`
	NewReaction []ReactionInfo `json:"new_reaction"`
}

// MessageReactionCountUpdate represents reaction changes on a message with anonymous reactions
type MessageReactionCountUpdate struct {
	Chat      Chat            `json:"chat"`
	MessageID int64           `json:"message_id"`
	Date      int64           `json:"date"`
	Reactions []ReactionCount `json:"reactions"`
}

// ReactionInfo represents the type of a reaction
type ReactionInfo struct {
	Type          string `json:"type"`
	Emoji         string `json:"emoji,omitempty"`
	CustomEmojiID string `json:"custom_emoji_id,omitempty"`
}

// ReactionCount represents a reaction added to a message along with the number of times it was added
type ReactionCount struct {
	Type       ReactionInfo `json:"type"`
	TotalCount int          `json:"total_count"`
}

// Database models

// DBMessage represents a message row in the database
type DBMessage struct {
	ID                  int64
	MessageID           int64
	ChatID              int64
	UserID              sql.NullInt64
	MessageThreadID     sql.NullInt64
	ReplyToMessageID    sql.NullInt64
	Date                time.Time
	EditDate            sql.NullTime
	Text                sql.NullString
	Caption             sql.NullString
	MediaGroupID        sql.NullString
	HasProtectedContent bool
	IsAutomaticForward  bool
	IsTopicMessage      bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// DBMessageReaction represents a message reaction row in the database
type DBMessageReaction struct {
	ID            int64
	ChatID        int64
	MessageID     int64
	UserID        sql.NullInt64
	ActorChatID   sql.NullInt64
	ReactionType  string
	EmojiValue    sql.NullString
	CustomEmojiID sql.NullString
	Date          time.Time
	IsRemoved     bool
	CreatedAt     time.Time
}

// DBReactionCount represents an aggregate reaction count row in the database
type DBReactionCount struct {
	ID            int64
	ChatID        int64
	MessageID     int64
	ReactionType  string
	EmojiValue    sql.NullString
	CustomEmojiID sql.NullString
	TotalCount    int
	Date          time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// DBMediaFile represents a media file row in the database
type DBMediaFile struct {
	ID                   int64
	MessageID            int64
	MediaType            string
	TelegramFileID       string
	TelegramFileUniqueID string
	MinIOObjectKey       string
	FileSize             sql.NullInt64
	MimeType             sql.NullString
	FileName             sql.NullString
	Duration             sql.NullInt32
	Width                sql.NullInt32
	Height               sql.NullInt32
	Performer            sql.NullString
	Title                sql.NullString
	CreatedAt            time.Time
}

// DBPhoto represents a photo row in the database
type DBPhoto struct {
	ID                   int64
	MessageID            int64
	TelegramFileID       string
	TelegramFileUniqueID string
	MinIOObjectKey       string
	Width                int
	Height               int
	FileSize             sql.NullInt64
	CreatedAt            time.Time
}

// DBLocation represents a location row in the database
type DBLocation struct {
	ID                   int64
	MessageID            int64
	Latitude             float64
	Longitude            float64
	HorizontalAccuracy   sql.NullFloat64
	LivePeriod           sql.NullInt32
	Heading              sql.NullInt32
	ProximityAlertRadius sql.NullInt32
	CreatedAt            time.Time
}
