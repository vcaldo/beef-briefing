package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const stateFileName = ".import-state.json"

// ImportState tracks the progress of an import operation
type ImportState struct {
	LastProcessedID int64            `json:"last_processed_id"`
	ChatID          int64            `json:"chat_id"`
	TotalMessages   int              `json:"total_messages"`
	ImportedCount   int              `json:"imported_count"`
	SkippedCount    int              `json:"skipped_count"`
	FailedCount     int              `json:"failed_count"`
	ReactionsCount  int              `json:"reactions_count"`
	StartedAt       string           `json:"started_at"`
	LastUpdatedAt   string           `json:"last_updated_at"`
	Users           map[int64]string `json:"users"`            // userID -> displayName
	Errors          []ImportError    `json:"errors,omitempty"` // Recent errors
	MediaStats      MediaStats       `json:"media_stats"`
	ReactionStats   ReactionStats    `json:"reaction_stats"`
}

// ImportError represents a single import error
type ImportError struct {
	MessageID int64  `json:"message_id"`
	Error     string `json:"error"`
	Timestamp string `json:"timestamp"`
}

// MediaStats tracks media import statistics
type MediaStats struct {
	PhotosImported     int `json:"photos_imported"`
	PhotosSkipped      int `json:"photos_skipped"`
	PhotosFailed       int `json:"photos_failed"`
	VideosImported     int `json:"videos_imported"`
	VideosSkipped      int `json:"videos_skipped"`
	VideosFailed       int `json:"videos_failed"`
	AnimationsImported int `json:"animations_imported"`
	AnimationsSkipped  int `json:"animations_skipped"`
	AnimationsFailed   int `json:"animations_failed"`
	VoicesImported     int `json:"voices_imported"`
	VoicesSkipped      int `json:"voices_skipped"`
	VoicesFailed       int `json:"voices_failed"`
	DocumentsImported  int `json:"documents_imported"`
	DocumentsSkipped   int `json:"documents_skipped"`
	DocumentsFailed    int `json:"documents_failed"`
}

// ReactionStats tracks reaction import statistics
type ReactionStats struct {
	UserReactionsImported int `json:"user_reactions_imported"`
	UserReactionsFailed   int `json:"user_reactions_failed"`
	CountUpdatesImported  int `json:"count_updates_imported"`
	CountUpdatesFailed    int `json:"count_updates_failed"`
}

// Manager handles state persistence
type Manager struct {
	exportPath string
	state      *ImportState
}

// NewManager creates a new state Manager
func NewManager(exportPath string) *Manager {
	return &Manager{
		exportPath: exportPath,
		state: &ImportState{
			Users: make(map[int64]string),
		},
	}
}

// Load loads the state from disk
func (m *Manager) Load() error {
	statePath := filepath.Join(m.exportPath, stateFileName)

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No state file, start fresh
			return nil
		}
		return fmt.Errorf("reading state file: %w", err)
	}

	if err := json.Unmarshal(data, m.state); err != nil {
		return fmt.Errorf("parsing state file: %w", err)
	}

	return nil
}

// Save persists the state to disk
func (m *Manager) Save() error {
	statePath := filepath.Join(m.exportPath, stateFileName)

	data, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}

	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("writing state file: %w", err)
	}

	return nil
}

// GetState returns the current state
func (m *Manager) GetState() *ImportState {
	return m.state
}

// SetLastProcessedID updates the last processed message ID
func (m *Manager) SetLastProcessedID(id int64) {
	m.state.LastProcessedID = id
}

// GetLastProcessedID returns the last processed message ID
func (m *Manager) GetLastProcessedID() int64 {
	return m.state.LastProcessedID
}

// SetChatID sets the chat ID being imported
func (m *Manager) SetChatID(id int64) {
	m.state.ChatID = id
}

// SetTotalMessages sets the total message count
func (m *Manager) SetTotalMessages(count int) {
	m.state.TotalMessages = count
}

// IncrementImported increments the imported message count
func (m *Manager) IncrementImported() {
	m.state.ImportedCount++
}

// IncrementSkipped increments the skipped message count
func (m *Manager) IncrementSkipped() {
	m.state.SkippedCount++
}

// IncrementFailed increments the failed message count
func (m *Manager) IncrementFailed() {
	m.state.FailedCount++
}

// AddError adds an error to the state
func (m *Manager) AddError(msgID int64, errMsg, timestamp string) {
	m.state.Errors = append(m.state.Errors, ImportError{
		MessageID: msgID,
		Error:     errMsg,
		Timestamp: timestamp,
	})
	// Keep only last 1000 errors
	if len(m.state.Errors) > 1000 {
		m.state.Errors = m.state.Errors[len(m.state.Errors)-1000:]
	}
}

// AddUser records a user mapping
func (m *Manager) AddUser(userID int64, displayName string) {
	if m.state.Users == nil {
		m.state.Users = make(map[int64]string)
	}
	m.state.Users[userID] = displayName
}

// SetStartedAt sets the import start time
func (m *Manager) SetStartedAt(timestamp string) {
	m.state.StartedAt = timestamp
}

// SetLastUpdatedAt sets the last update time
func (m *Manager) SetLastUpdatedAt(timestamp string) {
	m.state.LastUpdatedAt = timestamp
}

// GetMediaStats returns a pointer to the media stats
func (m *Manager) GetMediaStats() *MediaStats {
	return &m.state.MediaStats
}

// GetReactionStats returns a pointer to the reaction stats
func (m *Manager) GetReactionStats() *ReactionStats {
	return &m.state.ReactionStats
}

// IncrementReaction increments the reaction count
func (m *Manager) IncrementReaction() {
	m.state.ReactionsCount++
}

// ShouldSkip returns true if the message has already been processed
func (m *Manager) ShouldSkip(msgID int64) bool {
	return msgID <= m.state.LastProcessedID
}

// Reset clears the state for a fresh import
func (m *Manager) Reset() {
	m.state = &ImportState{
		Users: make(map[int64]string),
	}
}

// Delete removes the state file
func (m *Manager) Delete() error {
	statePath := filepath.Join(m.exportPath, stateFileName)
	if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing state file: %w", err)
	}
	return nil
}
