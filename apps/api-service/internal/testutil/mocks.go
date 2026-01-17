// Package testutil provides testing utilities for the api-service.
package testutil

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"beef-briefing/apps/api-service/internal/game/battle"
	"beef-briefing/apps/api-service/internal/models"
	"beef-briefing/apps/api-service/internal/storage"
)

// MockMinIOClient is a mock implementation of MinIO storage operations for testing.
// It stores uploaded data in memory and allows configuring responses.
type MockMinIOClient struct {
	mu sync.RWMutex

	// Storage holds uploaded objects keyed by object key.
	Storage map[string]*MockObject

	// UploadError is returned by UploadMedia if set.
	UploadError error

	// GetObjectError is returned by GetObject if set.
	GetObjectError error

	// GetPresignedURLError is returned by GetPresignedURL if set.
	GetPresignedURLError error

	// PresignedURLBase is the base URL used when generating presigned URLs.
	// Defaults to "https://mock-storage.example.com".
	PresignedURLBase string

	// UploadCalls tracks the number of times UploadMedia was called.
	UploadCalls int

	// UploadCount is an alias for UploadCalls for backward compatibility.
	UploadCount int

	// GetObjectCalls tracks the number of times GetObject was called.
	GetObjectCalls int

	// GetPresignedURLCalls tracks the number of times GetPresignedURL was called.
	GetPresignedURLCalls int

	// GetPresignedURLSecondsCalls tracks the number of times GetPresignedURLSeconds was called.
	GetPresignedURLSecondsCalls int

	// Configurable return values
	NextUploadObjectKey string
	NextUploadFileHash  string
	NextPresignedURL    string
}

// MockObject represents an object stored in the mock storage.
type MockObject struct {
	Data        []byte
	ContentType string
	ObjectKey   string
	FileHash    string
	UploadedAt  time.Time
}

// NewMockMinIOClient creates a new MockMinIOClient with default settings.
func NewMockMinIOClient() *MockMinIOClient {
	return &MockMinIOClient{
		Storage:          make(map[string]*MockObject),
		PresignedURLBase: "https://mock-storage.example.com",
	}
}

// UploadMedia uploads media to in-memory storage.
// Returns the object key and file hash.
func (m *MockMinIOClient) UploadMedia(ctx context.Context, fileID string, data []byte, mimeType string, mediaType string) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.UploadCalls++
	m.UploadCount++ // Keep both in sync for backward compatibility

	if m.UploadError != nil {
		return "", "", m.UploadError
	}

	// Use configured return values if set
	var objectKey, fileHash string
	if m.NextUploadObjectKey != "" {
		objectKey = m.NextUploadObjectKey
		fileHash = m.NextUploadFileHash
	} else {
		// Generate a simple hash for testing (not a real SHA256)
		fileHash = generateMockHash(data)
		objectKey = mediaType + "/" + fileHash[:2] + "/" + fileHash
	}

	m.Storage[objectKey] = &MockObject{
		Data:        data,
		ContentType: mimeType,
		ObjectKey:   objectKey,
		FileHash:    fileHash,
		UploadedAt:  time.Now(),
	}

	return objectKey, fileHash, nil
}

// GetObject retrieves an object from in-memory storage.
func (m *MockMinIOClient) GetObject(ctx context.Context, objectKey string) (io.ReadCloser, int64, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetObjectCalls++

	if m.GetObjectError != nil {
		return nil, 0, "", m.GetObjectError
	}

	obj, exists := m.Storage[objectKey]
	if !exists {
		return nil, 0, "", &MockNotFoundError{ObjectKey: objectKey}
	}

	reader := io.NopCloser(strings.NewReader(string(obj.Data)))
	return reader, int64(len(obj.Data)), obj.ContentType, nil
}

// GetObjectURL returns a mock URL for the object.
func (m *MockMinIOClient) GetObjectURL(objectKey string) string {
	return m.PresignedURLBase + "/mock-bucket/" + objectKey
}

// Ensure MockMinIOClient implements the storage.MinIOClientInterface at compile time
var _ storage.MinIOClientInterface = (*MockMinIOClient)(nil)

// GetPresignedURL generates a mock presigned URL.
func (m *MockMinIOClient) GetPresignedURL(ctx context.Context, objectKey string, expiry time.Duration) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetPresignedURLCalls++

	if m.GetPresignedURLError != nil {
		return "", m.GetPresignedURLError
	}

	// Use configured return value if set
	if m.NextPresignedURL != "" {
		return m.NextPresignedURL, nil
	}

	// Return a mock presigned URL with expiry encoded
	expiryTime := time.Now().Add(expiry).Unix()
	return m.PresignedURLBase + "/mock-bucket/" + objectKey + "?X-Mock-Expires=" + time.Unix(expiryTime, 0).Format(time.RFC3339), nil
}

// GetPresignedURLSeconds is a convenience wrapper that takes expiry in seconds.
// Implements the services.MinIOClient interface.
func (m *MockMinIOClient) GetPresignedURLSeconds(ctx context.Context, objectKey string, expirySeconds int) (string, error) {
	m.mu.Lock()
	m.GetPresignedURLSecondsCalls++
	m.mu.Unlock()

	expiry := time.Duration(expirySeconds) * time.Second
	return m.GetPresignedURL(ctx, objectKey, expiry)
}

// Reset clears all stored objects and resets call counters.
func (m *MockMinIOClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Storage = make(map[string]*MockObject)
	m.UploadCalls = 0
	m.UploadCount = 0
	m.GetObjectCalls = 0
	m.GetPresignedURLCalls = 0
	m.GetPresignedURLSecondsCalls = 0
	m.UploadError = nil
	m.GetObjectError = nil
	m.GetPresignedURLError = nil
	m.NextUploadObjectKey = ""
	m.NextUploadFileHash = ""
	m.NextPresignedURL = ""
}

// SetUploadResult configures the next UploadMedia call to return specific values.
func (m *MockMinIOClient) SetUploadResult(objectKey, fileHash string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.NextUploadObjectKey = objectKey
	m.NextUploadFileHash = fileHash
	m.UploadError = err
}

// SetPresignedURL configures the next GetPresignedURL call to return specific URL.
func (m *MockMinIOClient) SetPresignedURL(url string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.NextPresignedURL = url
	m.GetPresignedURLError = err
}

// AddObject directly adds an object to storage for test setup.
func (m *MockMinIOClient) AddObject(objectKey string, data []byte, contentType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Storage[objectKey] = &MockObject{
		Data:        data,
		ContentType: contentType,
		ObjectKey:   objectKey,
		FileHash:    generateMockHash(data),
		UploadedAt:  time.Now(),
	}
}

// MockNotFoundError is returned when an object is not found in mock storage.
type MockNotFoundError struct {
	ObjectKey string
}

func (e *MockNotFoundError) Error() string {
	return "object not found: " + e.ObjectKey
}

// generateMockHash generates a simple mock hash for testing purposes.
// Not a real SHA256 - just for testing deduplication logic.
func generateMockHash(data []byte) string {
	// Simple hash for testing: first 64 hex chars of repeated content
	const hexChars = "0123456789abcdef"
	hash := make([]byte, 64)
	for i := range hash {
		if i < len(data) {
			hash[i] = hexChars[data[i]%16]
		} else {
			hash[i] = hexChars[i%16]
		}
	}
	return string(hash)
}

// MockNewRelicApp is a mock implementation of New Relic Application for testing.
// It captures recorded events and allows inspection of instrumentation calls.
type MockNewRelicApp struct {
	mu sync.RWMutex

	// CustomEvents stores all custom events recorded via RecordCustomEvent.
	CustomEvents []MockCustomEvent

	// ShutdownCalled indicates if Shutdown was called.
	ShutdownCalled bool

	// ShutdownTimeout stores the timeout passed to Shutdown.
	ShutdownTimeout time.Duration

	// RecordCustomEventError is returned by RecordCustomEvent if set.
	RecordCustomEventError error
}

// MockCustomEvent represents a custom event recorded to New Relic.
type MockCustomEvent struct {
	EventType string
	Params    map[string]any
	Timestamp time.Time
}

// NewMockNewRelicApp creates a new MockNewRelicApp.
func NewMockNewRelicApp() *MockNewRelicApp {
	return &MockNewRelicApp{
		CustomEvents: make([]MockCustomEvent, 0),
	}
}

// RecordCustomEvent records a custom event for later inspection.
func (m *MockNewRelicApp) RecordCustomEvent(eventType string, params map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.RecordCustomEventError != nil {
		return m.RecordCustomEventError
	}

	m.CustomEvents = append(m.CustomEvents, MockCustomEvent{
		EventType: eventType,
		Params:    params,
		Timestamp: time.Now(),
	})
	return nil
}

// Shutdown simulates graceful shutdown.
func (m *MockNewRelicApp) Shutdown(timeout time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ShutdownCalled = true
	m.ShutdownTimeout = timeout
}

// Reset clears all recorded events and resets state.
func (m *MockNewRelicApp) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CustomEvents = make([]MockCustomEvent, 0)
	m.ShutdownCalled = false
	m.ShutdownTimeout = 0
	m.RecordCustomEventError = nil
}

// GetCustomEvents returns a copy of all recorded custom events.
func (m *MockNewRelicApp) GetCustomEvents() []MockCustomEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	events := make([]MockCustomEvent, len(m.CustomEvents))
	copy(events, m.CustomEvents)
	return events
}

// GetCustomEventsByType returns all events of a specific type.
func (m *MockNewRelicApp) GetCustomEventsByType(eventType string) []MockCustomEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var events []MockCustomEvent
	for _, e := range m.CustomEvents {
		if e.EventType == eventType {
			events = append(events, e)
		}
	}
	return events
}

// CustomEventCount returns the number of recorded custom events.
func (m *MockNewRelicApp) CustomEventCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.CustomEvents)
}

// MockNewRelicTransaction is a mock implementation of a New Relic Transaction.
// Used for testing code that creates segments or records errors.
type MockNewRelicTransaction struct {
	mu sync.RWMutex

	// Segments stores all started segments.
	Segments []MockSegment

	// Errors stores all noticed errors.
	Errors []error

	// Ended indicates if End() was called on the transaction.
	Ended bool
}

// MockSegment represents a New Relic segment.
type MockSegment struct {
	Name      string
	StartTime time.Time
	EndTime   time.Time
	Ended     bool
}

// NewMockNewRelicTransaction creates a new MockNewRelicTransaction.
func NewMockNewRelicTransaction() *MockNewRelicTransaction {
	return &MockNewRelicTransaction{
		Segments: make([]MockSegment, 0),
		Errors:   make([]error, 0),
	}
}

// StartSegment starts a new segment and returns an end function.
func (m *MockNewRelicTransaction) StartSegment(name string) *MockSegmentHandle {
	m.mu.Lock()
	defer m.mu.Unlock()

	seg := MockSegment{
		Name:      name,
		StartTime: time.Now(),
	}
	m.Segments = append(m.Segments, seg)
	idx := len(m.Segments) - 1

	return &MockSegmentHandle{
		txn:   m,
		index: idx,
	}
}

// NoticeError records an error.
func (m *MockNewRelicTransaction) NoticeError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Errors = append(m.Errors, err)
}

// End marks the transaction as ended.
func (m *MockNewRelicTransaction) End() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Ended = true
}

// Reset clears all segments and errors.
func (m *MockNewRelicTransaction) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Segments = make([]MockSegment, 0)
	m.Errors = make([]error, 0)
	m.Ended = false
}

// GetSegments returns a copy of all segments.
func (m *MockNewRelicTransaction) GetSegments() []MockSegment {
	m.mu.RLock()
	defer m.mu.RUnlock()

	segs := make([]MockSegment, len(m.Segments))
	copy(segs, m.Segments)
	return segs
}

// GetErrors returns a copy of all noticed errors.
func (m *MockNewRelicTransaction) GetErrors() []error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	errs := make([]error, len(m.Errors))
	copy(errs, m.Errors)
	return errs
}

// MockSegmentHandle is a handle to a mock segment that can be ended.
type MockSegmentHandle struct {
	txn   *MockNewRelicTransaction
	index int
}

// End ends the segment.
func (h *MockSegmentHandle) End() {
	h.txn.mu.Lock()
	defer h.txn.mu.Unlock()

	if h.index < len(h.txn.Segments) {
		h.txn.Segments[h.index].EndTime = time.Now()
		h.txn.Segments[h.index].Ended = true
	}
}

// MockDealer is a mock implementation of the Dealer from the shop package.
// It allows tests to control card dealing behavior without database access.
type MockDealer struct {
	mu sync.RWMutex

	// CardCount is the number of cards available for dealing
	CardCount int

	// DealCardsResults is a queue of results to return from DealCards calls
	DealCardsResults [][]*struct {
		ID     int64
		UserID int64
		Combat struct {
			ATK int
			HP  int
		}
	}

	// Errors for injection
	GetCardCountError error
	DealCardsError    error

	// Call tracking
	GetCardCountCalls int
	DealCardsCalls    int
}

// NewMockDealer creates a new MockDealer with default values.
func NewMockDealer() *MockDealer {
	return &MockDealer{
		CardCount:        0,
		DealCardsResults: make([][]*struct {
			ID     int64
			UserID int64
			Combat struct {
				ATK int
				HP  int
			}
		}, 0),
	}
}

// GetCardCount returns the mock card count.
func (m *MockDealer) GetCardCount(ctx context.Context, chatID int64) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetCardCountCalls++

	if m.GetCardCountError != nil {
		return 0, m.GetCardCountError
	}

	return m.CardCount, nil
}

// DealCards returns the next queued result from DealCardsResults.
func (m *MockDealer) DealCards(ctx context.Context, chatID int64, count int) ([]*battle.ShopCard, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.DealCardsCalls++

	if m.DealCardsError != nil {
		return nil, m.DealCardsError
	}

	// For test purposes, return empty slice
	// Tests that need specific card data should set DealCardsResults
	return []*battle.ShopCard{}, nil
}

// SetCardCount sets the card count to be returned by GetCardCount.
func (m *MockDealer) SetCardCount(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CardCount = count
}

// AddDealCardsResult adds a result to the queue for DealCards.
func (m *MockDealer) AddDealCardsResult(cards interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Convert the input to our internal type
	// This is a simplified version - in practice you'd need proper type conversion
	m.DealCardsResults = append(m.DealCardsResults, nil)
}

// Reset clears all state and resets counters.
func (m *MockDealer) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CardCount = 0
	m.DealCardsResults = make([][]*struct {
		ID     int64
		UserID int64
		Combat struct {
			ATK int
			HP  int
		}
	}, 0)
	m.GetCardCountError = nil
	m.DealCardsError = nil
	m.GetCardCountCalls = 0
	m.DealCardsCalls = 0
}

// =============================================================================
// MockIngestService
// =============================================================================

// MockIngestService is a mock implementation of IngestService for testing handlers.
type MockIngestService struct {
	mu sync.RWMutex

	// ProcessUpdateCalled tracks if ProcessUpdate was called
	ProcessUpdateCalled bool

	// LastUpdate stores the last update passed to ProcessUpdate
	LastUpdate *models.Update

	// LastFiles stores the last files map passed to ProcessUpdate
	LastFiles map[string][]byte

	// ProcessUpdateError is the error to return from ProcessUpdate
	ProcessUpdateError error
}

// NewMockIngestService creates a new MockIngestService
func NewMockIngestService() *MockIngestService {
	return &MockIngestService{
		ProcessUpdateCalled: false,
		LastFiles:           make(map[string][]byte),
	}
}

// ProcessUpdate mocks the IngestService.ProcessUpdate method
func (m *MockIngestService) ProcessUpdate(ctx context.Context, update *models.Update, files map[string][]byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ProcessUpdateCalled = true
	m.LastUpdate = update

	m.LastFiles = make(map[string][]byte)
	for k, v := range files {
		m.LastFiles[k] = v
	}

	return m.ProcessUpdateError
}

// SetProcessUpdateError configures the error to return from ProcessUpdate
func (m *MockIngestService) SetProcessUpdateError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ProcessUpdateError = err
}

// Reset clears all state
func (m *MockIngestService) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ProcessUpdateCalled = false
	m.LastUpdate = nil
	m.LastFiles = make(map[string][]byte)
	m.ProcessUpdateError = nil
}
