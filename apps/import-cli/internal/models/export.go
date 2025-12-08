package models

// ExportRoot represents the root structure of a Telegram export JSON
type ExportRoot struct {
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	ID       int64           `json:"id"`
	Messages []ExportMessage `json:"messages"`
}

// ExportMessage represents a single message from the Telegram export
type ExportMessage struct {
	ID              int64            `json:"id"`
	Type            string           `json:"type"`
	Date            string           `json:"date"`
	DateUnixtime    string           `json:"date_unixtime"`
	Edited          string           `json:"edited,omitempty"`
	EditedUnixtime  string           `json:"edited_unixtime,omitempty"`
	From            string           `json:"from,omitempty"`
	FromID          string           `json:"from_id,omitempty"`
	Actor           string           `json:"actor,omitempty"`
	ActorID         string           `json:"actor_id,omitempty"`
	Action          string           `json:"action,omitempty"`
	ReplyToMsgID    *int64           `json:"reply_to_message_id,omitempty"`
	ForwardedFrom   string           `json:"forwarded_from,omitempty"`
	ForwardedFromID string           `json:"forwarded_from_id,omitempty"`
	Text            any              `json:"text,omitempty"` // Can be string or []TextEntity
	TextEntities    []ExportEntity   `json:"text_entities,omitempty"`
	Photo           string           `json:"photo,omitempty"`
	PhotoFileSize   int64            `json:"photo_file_size,omitempty"`
	Width           int              `json:"width,omitempty"`
	Height          int              `json:"height,omitempty"`
	File            string           `json:"file,omitempty"`
	FileName        string           `json:"file_name,omitempty"`
	FileSize        int64            `json:"file_size,omitempty"`
	MediaType       string           `json:"media_type,omitempty"`
	MimeType        string           `json:"mime_type,omitempty"`
	DurationSeconds int              `json:"duration_seconds,omitempty"`
	Thumbnail       string           `json:"thumbnail,omitempty"`
	Reactions       []ExportReaction `json:"reactions,omitempty"`
	Members         []string         `json:"members,omitempty"`
	StickerEmoji    string           `json:"sticker_emoji,omitempty"`
}

// ExportEntity represents a text entity in the export
type ExportEntity struct {
	Type   string `json:"type"`
	Text   string `json:"text"`
	UserID *int64 `json:"user_id,omitempty"`
}

// ExportReaction represents a reaction in the export
type ExportReaction struct {
	Type   string               `json:"type"`
	Count  int                  `json:"count"`
	Emoji  string               `json:"emoji"`
	Recent []ExportReactionUser `json:"recent,omitempty"`
}

// ExportReactionUser represents a user who reacted
type ExportReactionUser struct {
	From   string `json:"from"`
	FromID string `json:"from_id"`
	Date   string `json:"date"`
}

// GetText returns the message text as a plain string
func (m *ExportMessage) GetText() string {
	switch v := m.Text.(type) {
	case string:
		return v
	case []any:
		var result string
		for _, item := range v {
			switch t := item.(type) {
			case string:
				result += t
			case map[string]any:
				if text, ok := t["text"].(string); ok {
					result += text
				}
			}
		}
		return result
	default:
		return ""
	}
}

// IsServiceMessage returns true if this is a service message (join, leave, etc.)
func (m *ExportMessage) IsServiceMessage() bool {
	return m.Type == "service"
}

// HasMedia returns true if the message contains media
func (m *ExportMessage) HasMedia() bool {
	return m.Photo != "" || m.File != ""
}

// GetMediaPath returns the media file path (photo or file)
func (m *ExportMessage) GetMediaPath() string {
	if m.Photo != "" {
		return m.Photo
	}
	return m.File
}
