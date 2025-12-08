package models

import (
	"database/sql"
	"encoding/json"
	"time"
)

// LegacyMessage represents a message from the legacy PostgreSQL database
type LegacyMessage struct {
	ID               int64
	MessageID        int64
	MessageType      string
	Timestamp        time.Time
	ChatID           int64
	UserID           int64
	ReplyToMessageID sql.NullInt64
	FirstName        sql.NullString
	LastName         sql.NullString
	Username         sql.NullString
	DisplayName      sql.NullString
	Content          json.RawMessage
	Moderated        bool
}

// GetDisplayName returns the best available display name for the user
func (m *LegacyMessage) GetDisplayName() string {
	if m.DisplayName.Valid && m.DisplayName.String != "" {
		return m.DisplayName.String
	}
	if m.FirstName.Valid && m.FirstName.String != "" {
		return m.FirstName.String
	}
	return ""
}

// GetUsername returns the username if available
func (m *LegacyMessage) GetUsername() string {
	if m.Username.Valid {
		return m.Username.String
	}
	return ""
}
