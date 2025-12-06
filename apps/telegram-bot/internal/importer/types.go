package importer

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ExportData represents the top-level structure of Telegram export JSON
type ExportData struct {
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	ID       int64           `json:"id"`
	Messages []ExportMessage `json:"messages"`
}

// ExportMessage represents a single message from the export
type ExportMessage struct {
	ID              int             `json:"id"`
	Type            string          `json:"type"` // "message" or "service"
	Date            string          `json:"date"`
	DateUnixtime    string          `json:"date_unixtime"`
	From            *string         `json:"from"`
	FromID          string          `json:"from_id"`
	Text            json.RawMessage `json:"text"` // Can be string or []TextEntity
	TextEntities    []TextEntity    `json:"text_entities"`
	ReplyToMsgID    *int            `json:"reply_to_message_id"`
	ForwardedFrom   *string         `json:"forwarded_from"`
	ForwardedFromID *string         `json:"forwarded_from_id"`

	// Media fields
	Photo         *string `json:"photo"`
	PhotoFileSize *int    `json:"photo_file_size"`
	File          *string `json:"file"`
	FileName      *string `json:"file_name"`
	FileSize      *int    `json:"file_size"`
	MediaType     *string `json:"media_type"`
	MimeType      *string `json:"mime_type"`
	DurationSec   *int    `json:"duration_seconds"`
	Width         *int    `json:"width"`
	Height        *int    `json:"height"`
	Thumbnail     *string `json:"thumbnail"`

	// Service message fields
	Actor   *string  `json:"actor"`
	ActorID *string  `json:"actor_id"`
	Action  *string  `json:"action"`
	Members []string `json:"members"`
}

// TextEntity represents a text entity in a message
type TextEntity struct {
	Type   string `json:"type"`
	Text   string `json:"text"`
	UserID *int64 `json:"user_id,omitempty"`
}

// ImportProgress tracks the progress of an import operation
type ImportProgress struct {
	Total         int
	Processed     int
	Inserted      int
	Skipped       int
	ErrorCount    int
	MediaUploaded int
	CurrentChunk  int
	TotalChunks   int
}

// ParseUserID extracts the numeric user ID from Telegram export format
// Example: "user42511703" -> 42511703
func ParseUserID(fromID string) (int64, error) {
	if fromID == "" {
		return 0, fmt.Errorf("empty user ID")
	}

	// Remove "user" prefix
	idStr := strings.TrimPrefix(fromID, "user")
	if idStr == fromID {
		// No "user" prefix found, try parsing as-is
		id, err := strconv.ParseInt(fromID, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse user ID %q: %w", fromID, err)
		}
		return id, nil
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse user ID %q: %w", fromID, err)
	}
	return id, nil
}

// MapServiceAction maps Telegram export service actions to database actions
func MapServiceAction(exportAction string) string {
	actionMap := map[string]string{
		"invite_members":        "user_joined",
		"remove_members":        "user_left",
		"join_group_by_link":    "user_joined",
		"migrate_to_supergroup": "chat_migrated",
		"pin_message":           "message_pinned",
		"edit_group_title":      "title_changed",
		"edit_group_photo":      "photo_changed",
	}

	if mapped, ok := actionMap[exportAction]; ok {
		return mapped
	}
	return exportAction
}

// GetTextContent extracts text content from the Text field (handles string or array)
func (em *ExportMessage) GetTextContent() string {
	if len(em.Text) == 0 {
		return ""
	}

	// Try to unmarshal as string first
	var str string
	if err := json.Unmarshal(em.Text, &str); err == nil {
		return str
	}

	// Try to unmarshal as array of TextEntity
	var entities []TextEntity
	if err := json.Unmarshal(em.Text, &entities); err == nil {
		var sb strings.Builder
		for _, entity := range entities {
			sb.WriteString(entity.Text)
		}
		return sb.String()
	}

	return ""
}
