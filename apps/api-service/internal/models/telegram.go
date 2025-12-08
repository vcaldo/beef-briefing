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
	MyChatMember         *ChatMemberUpdated          `json:"my_chat_member,omitempty"`
}

// Chat represents a Telegram chat
type Chat struct {
	ID                int64  `json:"id"`
	Type              string `json:"type"`
	Title             string `json:"title,omitempty"`
	Username          string `json:"username,omitempty"`
	FirstName         string `json:"first_name,omitempty"`
	LastName          string `json:"last_name,omitempty"`
	IsForum           bool   `json:"is_forum,omitempty"`
	MigrateFromChatID *int64 `json:"migrate_from_chat_id,omitempty"`
	MigrateToChatID   *int64 `json:"migrate_to_chat_id,omitempty"`
}

// ChatMemberUpdated represents changes in the status of a chat member
type ChatMemberUpdated struct {
	Chat          Chat  `json:"chat"`
	From          User  `json:"from"`
	Date          int64 `json:"date"`
	OldChatMember any   `json:"old_chat_member"`
	NewChatMember any   `json:"new_chat_member"`
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
	Sticker             *Sticker        `json:"sticker,omitempty"`
	Location            *Location       `json:"location,omitempty"`
	Venue               *Venue          `json:"venue,omitempty"`
	Contact             *Contact        `json:"contact,omitempty"`
	Poll                *Poll           `json:"poll,omitempty"`
	Dice                *Dice           `json:"dice,omitempty"`
	Game                *Game           `json:"game,omitempty"`
	HasProtectedContent bool            `json:"has_protected_content,omitempty"`
	IsAutomaticForward  bool            `json:"is_automatic_forward,omitempty"`
	IsTopicMessage      bool            `json:"is_topic_message,omitempty"`
	MigrateFromChatID   *int64          `json:"migrate_from_chat_id,omitempty"`
	MigrateToChatID     *int64          `json:"migrate_to_chat_id,omitempty"`
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

// Sticker represents a sticker
type Sticker struct {
	FileID        string     `json:"file_id"`
	FileUniqueID  string     `json:"file_unique_id"`
	Type          string     `json:"type"`
	Width         int        `json:"width"`
	Height        int        `json:"height"`
	IsAnimated    bool       `json:"is_animated"`
	IsVideo       bool       `json:"is_video"`
	Thumbnail     *PhotoSize `json:"thumbnail,omitempty"`
	Emoji         string     `json:"emoji,omitempty"`
	SetName       string     `json:"set_name,omitempty"`
	MaskPosition  any        `json:"mask_position,omitempty"`
	CustomEmojiID string     `json:"custom_emoji_id,omitempty"`
	FileSize      *int64     `json:"file_size,omitempty"`
}

// Venue represents a venue
type Venue struct {
	Location        Location `json:"location"`
	Title           string   `json:"title"`
	Address         string   `json:"address"`
	FoursquareID    string   `json:"foursquare_id,omitempty"`
	FoursquareType  string   `json:"foursquare_type,omitempty"`
	GooglePlaceID   string   `json:"google_place_id,omitempty"`
	GooglePlaceType string   `json:"google_place_type,omitempty"`
}

// Contact represents a phone contact
type Contact struct {
	PhoneNumber string `json:"phone_number"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name,omitempty"`
	UserID      *int64 `json:"user_id,omitempty"`
	VCard       string `json:"vcard,omitempty"`
}

// Poll represents a poll
type Poll struct {
	ID                    string       `json:"id"`
	Question              string       `json:"question"`
	Options               []PollOption `json:"options"`
	TotalVoterCount       int          `json:"total_voter_count"`
	IsClosed              bool         `json:"is_closed"`
	IsAnonymous           bool         `json:"is_anonymous"`
	Type                  string       `json:"type"`
	AllowsMultipleAnswers bool         `json:"allows_multiple_answers"`
	CorrectOptionID       *int         `json:"correct_option_id,omitempty"`
	Explanation           string       `json:"explanation,omitempty"`
	OpenPeriod            *int         `json:"open_period,omitempty"`
	CloseDate             *int64       `json:"close_date,omitempty"`
}

// PollOption represents one option in a poll
type PollOption struct {
	Text       string `json:"text"`
	VoterCount int    `json:"voter_count"`
}

// Dice represents an animated emoji that displays a random value
type Dice struct {
	Emoji string `json:"emoji"`
	Value int    `json:"value"`
}

// Game represents a game
type Game struct {
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	Photo        []PhotoSize     `json:"photo"`
	Text         string          `json:"text,omitempty"`
	TextEntities []MessageEntity `json:"text_entities,omitempty"`
	Animation    *Animation      `json:"animation,omitempty"`
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
	FileHash             string
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
	FileHash             string
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

// DBSticker represents a sticker row in the database
type DBSticker struct {
	ID            int64
	MessageID     int64
	MediaFileID   int64
	StickerType   string
	Emoji         sql.NullString
	SetName       sql.NullString
	IsAnimated    bool
	IsVideo       bool
	CustomEmojiID sql.NullString
	CreatedAt     time.Time
}

// DBGame represents a game row in the database
type DBGame struct {
	ID          int64
	MessageID   int64
	Title       string
	Description string
	Text        sql.NullString
	CreatedAt   time.Time
}

// DBGamePhoto represents a game photo row in the database
type DBGamePhoto struct {
	ID                   int64
	GameID               int64
	TelegramFileID       string
	TelegramFileUniqueID string
	MinIOObjectKey       string
	FileHash             string
	Width                int
	Height               int
	FileSize             sql.NullInt64
	CreatedAt            time.Time
}

// DBPoll represents a poll row in the database
type DBPoll struct {
	ID                    int64
	MessageID             int64
	TelegramPollID        string
	Question              string
	PollType              string
	TotalVoterCount       int
	IsClosed              bool
	IsAnonymous           bool
	AllowsMultipleAnswers bool
	CorrectOptionID       sql.NullInt32
	Explanation           sql.NullString
	OpenPeriod            sql.NullInt32
	CloseDate             sql.NullTime
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// DBPollOption represents a poll option row in the database
type DBPollOption struct {
	ID          int64
	PollID      int64
	OptionIndex int
	OptionText  string
	VoterCount  int
	CreatedAt   time.Time
}

// DBContact represents a contact row in the database
type DBContact struct {
	ID          int64
	MessageID   int64
	PhoneNumber string
	FirstName   string
	LastName    sql.NullString
	UserID      sql.NullInt64
	VCard       sql.NullString
	CreatedAt   time.Time
}

// DBVenue represents a venue row in the database
type DBVenue struct {
	ID              int64
	MessageID       int64
	LocationID      int64
	Title           string
	Address         string
	FoursquareID    sql.NullString
	FoursquareType  sql.NullString
	GooglePlaceID   sql.NullString
	GooglePlaceType sql.NullString
	CreatedAt       time.Time
}

// DBDice represents a dice row in the database
type DBDice struct {
	ID        int64
	MessageID int64
	Emoji     string
	DiceValue int
	CreatedAt time.Time
}
