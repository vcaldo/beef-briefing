package models

// Update represents a Telegram update for the API
type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message,omitempty"`
}

// Chat represents a Telegram chat
type Chat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title,omitempty"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

// User represents a Telegram user
type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

// Message represents a Telegram message for the API
type Message struct {
	MessageID      int64           `json:"message_id"`
	From           *User           `json:"from,omitempty"`
	Chat           Chat            `json:"chat"`
	Date           int64           `json:"date"`
	EditDate       *int64          `json:"edit_date,omitempty"`
	Text           string          `json:"text,omitempty"`
	Caption        string          `json:"caption,omitempty"`
	Entities       []MessageEntity `json:"entities,omitempty"`
	ReplyToMessage *Message        `json:"reply_to_message,omitempty"`
	Photo          []PhotoSize     `json:"photo,omitempty"`
	Video          *Video          `json:"video,omitempty"`
	Audio          *Audio          `json:"audio,omitempty"`
	Voice          *Voice          `json:"voice,omitempty"`
	Document       *Document       `json:"document,omitempty"`
	Animation      *Animation      `json:"animation,omitempty"`
	VideoNote      *VideoNote      `json:"video_note,omitempty"`
}

// MessageEntity represents a special entity in a text message
type MessageEntity struct {
	Type   string `json:"type"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
	URL    string `json:"url,omitempty"`
	User   *User  `json:"user,omitempty"`
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
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Duration     int    `json:"duration"`
	FileName     string `json:"file_name,omitempty"`
	MimeType     string `json:"mime_type,omitempty"`
	FileSize     *int64 `json:"file_size,omitempty"`
}

// Audio represents an audio file
type Audio struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Duration     int    `json:"duration"`
	FileName     string `json:"file_name,omitempty"`
	MimeType     string `json:"mime_type,omitempty"`
	FileSize     *int64 `json:"file_size,omitempty"`
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
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FileName     string `json:"file_name,omitempty"`
	MimeType     string `json:"mime_type,omitempty"`
	FileSize     *int64 `json:"file_size,omitempty"`
}

// Animation represents an animation file (GIF or H.264/MPEG-4 AVC video without sound)
type Animation struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Duration     int    `json:"duration"`
	FileName     string `json:"file_name,omitempty"`
	MimeType     string `json:"mime_type,omitempty"`
	FileSize     *int64 `json:"file_size,omitempty"`
}

// VideoNote represents a video message (round video)
type VideoNote struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Length       int    `json:"length"`
	Duration     int    `json:"duration"`
	FileSize     *int64 `json:"file_size,omitempty"`
}
