package models

// ExportRoot represents the root structure of a Telegram export JSON
type ExportRoot struct {
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	ID       int64           `json:"id"`
	Messages []ExportMessage `json:"messages"`
}

// ExportMessage represents a single message in Telegram export format
type ExportMessage struct {
	ID             int64          `json:"id"`
	Type           string         `json:"type"`
	Date           string         `json:"date"`
	DateUnixtime   string         `json:"date_unixtime"`
	From           string         `json:"from,omitempty"`
	FromID         string         `json:"from_id,omitempty"`
	ReplyToMsgID   *int64         `json:"reply_to_message_id,omitempty"`
	Text           any            `json:"text"`
	TextEntities   []ExportEntity `json:"text_entities"`
}

// ExportEntity represents a text entity in the export
type ExportEntity struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
