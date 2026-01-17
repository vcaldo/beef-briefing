package handlers

import (
	"context"
	"sync"

	"beef-briefing/apps/api-service/internal/models"
)

// mockProfilePhotoService is a test-only mock implementation of ProfilePhotoServiceInterface.
// This must be in the handlers package to avoid import cycles.
type mockProfilePhotoService struct {
	mu sync.RWMutex

	// ProcessUserPhotos fields
	processUserPhotosCalled bool
	lastUserID              int64
	lastUserPhotos          []models.ProfilePhotoMeta
	lastUserFiles           map[string][]byte
	processUserPhotosError  error

	// ProcessChatPhotos fields
	processChatPhotosCalled bool
	lastChatID              int64
	lastChatPhotos          []models.ProfilePhotoMeta
	lastChatFiles           map[string][]byte
	processChatPhotosError  error

	// GetAllUserIDs fields
	userIDs             []int64
	getAllUserIDsCalled bool
	getAllUserIDsError  error

	// GetAllChatIDs fields
	chatIDs             []int64
	getAllChatIDsCalled bool
	getAllChatIDsError  error

	// GetUserPhoto fields
	userPhotoURL        string
	getUserPhotoCalled  bool
	lastGetUserPhotoID  int64
	lastGetUserPhotoSize string
	getUserPhotoError   error

	// GetChatPhoto fields
	chatPhotoURL        string
	getChatPhotoCalled  bool
	lastGetChatPhotoID  int64
	lastGetChatPhotoSize string
	getChatPhotoError   error
}

// newMockProfilePhotoService creates a new mockProfilePhotoService.
func newMockProfilePhotoService() *mockProfilePhotoService {
	return &mockProfilePhotoService{
		userIDs: []int64{},
		chatIDs: []int64{},
	}
}

// ProcessUserPhotos stores profile photos for a user.
func (m *mockProfilePhotoService) ProcessUserPhotos(ctx context.Context, userID int64, photos []models.ProfilePhotoMeta, files map[string][]byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.processUserPhotosCalled = true
	m.lastUserID = userID
	m.lastUserPhotos = photos
	m.lastUserFiles = files

	return m.processUserPhotosError
}

// ProcessChatPhotos stores profile photos for a chat.
func (m *mockProfilePhotoService) ProcessChatPhotos(ctx context.Context, chatID int64, photos []models.ProfilePhotoMeta, files map[string][]byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.processChatPhotosCalled = true
	m.lastChatID = chatID
	m.lastChatPhotos = photos
	m.lastChatFiles = files

	return m.processChatPhotosError
}

// GetAllUserIDs returns all user IDs from the database.
func (m *mockProfilePhotoService) GetAllUserIDs(ctx context.Context) ([]int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getAllUserIDsCalled = true

	if m.getAllUserIDsError != nil {
		return nil, m.getAllUserIDsError
	}

	return m.userIDs, nil
}

// GetAllChatIDs returns all chat IDs from the database.
func (m *mockProfilePhotoService) GetAllChatIDs(ctx context.Context) ([]int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getAllChatIDsCalled = true

	if m.getAllChatIDsError != nil {
		return nil, m.getAllChatIDsError
	}

	return m.chatIDs, nil
}

// GetUserPhoto retrieves a user's profile photo by size.
func (m *mockProfilePhotoService) GetUserPhoto(ctx context.Context, userID int64, size string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getUserPhotoCalled = true
	m.lastGetUserPhotoID = userID
	m.lastGetUserPhotoSize = size

	if m.getUserPhotoError != nil {
		return "", m.getUserPhotoError
	}

	return m.userPhotoURL, nil
}

// GetChatPhoto retrieves a chat's profile photo by size.
func (m *mockProfilePhotoService) GetChatPhoto(ctx context.Context, chatID int64, size string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getChatPhotoCalled = true
	m.lastGetChatPhotoID = chatID
	m.lastGetChatPhotoSize = size

	if m.getChatPhotoError != nil {
		return "", m.getChatPhotoError
	}

	return m.chatPhotoURL, nil
}

// Setter methods for configuring mock behavior

func (m *mockProfilePhotoService) setProcessUserPhotosError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.processUserPhotosError = err
}

func (m *mockProfilePhotoService) setProcessChatPhotosError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.processChatPhotosError = err
}

func (m *mockProfilePhotoService) setUserIDs(ids []int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.userIDs = ids
}

func (m *mockProfilePhotoService) setGetAllUserIDsError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getAllUserIDsError = err
}

func (m *mockProfilePhotoService) setChatIDs(ids []int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chatIDs = ids
}

func (m *mockProfilePhotoService) setGetAllChatIDsError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getAllChatIDsError = err
}

func (m *mockProfilePhotoService) setUserPhotoURL(url string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.userPhotoURL = url
}

func (m *mockProfilePhotoService) setGetUserPhotoError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getUserPhotoError = err
}

func (m *mockProfilePhotoService) setChatPhotoURL(url string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chatPhotoURL = url
}

func (m *mockProfilePhotoService) setGetChatPhotoError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getChatPhotoError = err
}
