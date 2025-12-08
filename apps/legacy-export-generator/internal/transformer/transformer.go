package transformer

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"beef-briefing/apps/legacy-export-generator/internal/models"
)

// Transformer converts legacy messages to Telegram export format
type Transformer struct {
	chatName string
	chatType string
	chatID   int64
}

// New creates a new Transformer instance
func New(chatName, chatType string, chatID int64) *Transformer {
	return &Transformer{
		chatName: chatName,
		chatType: chatType,
		chatID:   chatID,
	}
}

// Transform converts legacy messages to export format using two-pass approach
func (t *Transformer) Transform(messages []models.LegacyMessage) (*models.ExportRoot, error) {
	// Pass 1: Build message ID mapping (original message_id -> sequential ID)
	idMap := make(map[int64]int64)
	for i, msg := range messages {
		sequentialID := int64(i + 1)
		idMap[msg.MessageID] = sequentialID
	}

	// Pass 2: Transform messages with resolved reply IDs
	exportMessages := make([]models.ExportMessage, 0, len(messages))
	for i, msg := range messages {
		exportMsg, err := t.transformMessage(msg, int64(i+1), idMap)
		if err != nil {
			return nil, fmt.Errorf("transforming message %d: %w", msg.ID, err)
		}
		exportMessages = append(exportMessages, exportMsg)
	}

	return &models.ExportRoot{
		Name:     t.chatName,
		Type:     t.chatType,
		ID:       t.chatID,
		Messages: exportMessages,
	}, nil
}

func (t *Transformer) transformMessage(msg models.LegacyMessage, seqID int64, idMap map[int64]int64) (models.ExportMessage, error) {
	// Format timestamp
	date := msg.Timestamp.Format("2006-01-02T15:04:05")
	dateUnixtime := strconv.FormatInt(msg.Timestamp.Unix(), 10)

	// Format user ID
	fromID := fmt.Sprintf("user%d", msg.UserID)

	// Get display name
	from := msg.GetDisplayName()

	// Parse text content
	text, textEntities := t.parseContent(msg.Content, msg.MessageType)

	exportMsg := models.ExportMessage{
		ID:           seqID,
		Type:         "message",
		Date:         date,
		DateUnixtime: dateUnixtime,
		From:         from,
		FromID:       fromID,
		Text:         text,
		TextEntities: textEntities,
	}

	// Resolve reply_to_message_id
	if msg.ReplyToMessageID.Valid {
		if resolvedID, ok := idMap[msg.ReplyToMessageID.Int64]; ok {
			exportMsg.ReplyToMsgID = &resolvedID
		}
		// If original message not found in map, omit the reply reference
	}

	return exportMsg, nil
}

// parseContent extracts text from the legacy content JSONB field
func (t *Transformer) parseContent(content json.RawMessage, msgType string) (any, []models.ExportEntity) {
	if len(content) == 0 {
		return "", []models.ExportEntity{}
	}

	switch msgType {
	case "text":
		return t.parseTextContent(content)
	case "photo", "sticker", "video", "video_note", "voice", "animation", "document":
		// Leave media fields empty, return empty text
		return "", []models.ExportEntity{}
	case "generic":
		// Service messages - return empty
		return "", []models.ExportEntity{}
	default:
		return t.parseTextContent(content)
	}
}

// parseTextContent handles text message content
func (t *Transformer) parseTextContent(content json.RawMessage) (any, []models.ExportEntity) {
	// Try to unmarshal as a simple string (quoted text)
	var textStr string
	if err := json.Unmarshal(content, &textStr); err == nil {
		// Unescape common escape sequences
		textStr = t.unescapeText(textStr)
		return textStr, []models.ExportEntity{
			{Type: "plain", Text: textStr},
		}
	}

	// If not a simple string, return empty
	return "", []models.ExportEntity{}
}

// unescapeText handles escaped characters in legacy content
func (t *Transformer) unescapeText(s string) string {
	// The JSON unmarshal already handles most escaping (\", \\, etc.)
	// Handle literal \n that might be in the data as actual newlines
	s = strings.ReplaceAll(s, "\\n", "\n")
	return s
}
