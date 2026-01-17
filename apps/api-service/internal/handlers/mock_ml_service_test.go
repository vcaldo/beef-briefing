package handlers

import (
	"context"
	"sync"

	"beef-briefing/apps/api-service/internal/services"
)

// mockMLService is a test-only mock implementation of MLServiceInterface.
// This must be in the handlers package to avoid import cycles (handlers -> services <- testutil).
type mockMLService struct {
	mu sync.RWMutex

	// GetUnprocessedMessages fields
	messages                     []services.MLMessage
	hasMore                      bool
	getUnprocessedMessagesError  error
	getUnprocessedMessagesCalled bool
	lastGetUnprocessedLimit      int

	// SaveResults fields
	saveResultsCalled bool
	lastSaveRequest   *services.SaveResultsRequest
	saveResultsError  error

	// GetProcessingStats fields
	stats                   *services.ProcessingStats
	getProcessingStatsCalled bool
	getProcessingStatsError  error
}

// newMockMLService creates a new mockMLService.
func newMockMLService() *mockMLService {
	return &mockMLService{
		messages: []services.MLMessage{},
		stats:    &services.ProcessingStats{},
	}
}

// GetUnprocessedMessages retrieves messages for ML processing.
func (m *mockMLService) GetUnprocessedMessages(ctx context.Context, limit int) (*services.GetMessagesResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getUnprocessedMessagesCalled = true
	m.lastGetUnprocessedLimit = limit

	if m.getUnprocessedMessagesError != nil {
		return nil, m.getUnprocessedMessagesError
	}

	return &services.GetMessagesResponse{
		Messages: m.messages,
		HasMore:  m.hasMore,
	}, nil
}

// SaveResults saves ML analysis results.
func (m *mockMLService) SaveResults(ctx context.Context, req *services.SaveResultsRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.saveResultsCalled = true
	m.lastSaveRequest = req

	return m.saveResultsError
}

// GetProcessingStats retrieves processing statistics.
func (m *mockMLService) GetProcessingStats(ctx context.Context) (*services.ProcessingStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getProcessingStatsCalled = true

	if m.getProcessingStatsError != nil {
		return nil, m.getProcessingStatsError
	}

	return m.stats, nil
}

// Setter methods for configuring mock behavior

func (m *mockMLService) setGetUnprocessedMessagesError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getUnprocessedMessagesError = err
}

func (m *mockMLService) setSaveResultsError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saveResultsError = err
}

func (m *mockMLService) setGetProcessingStatsError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getProcessingStatsError = err
}

func (m *mockMLService) setMessages(messages []services.MLMessage, hasMore bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = messages
	m.hasMore = hasMore
}

func (m *mockMLService) setStats(stats *services.ProcessingStats) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stats = stats
}
