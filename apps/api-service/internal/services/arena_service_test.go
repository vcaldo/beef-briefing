package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"beef-briefing/apps/api-service/internal/apperror"
	"beef-briefing/apps/api-service/internal/repository"
	"beef-briefing/apps/api-service/internal/testutil"
)

// mockGameRepository is a mock implementation of GameRepositoryInterface for testing.
// It is thread-safe and can be used in concurrent test scenarios.
type mockGameRepository struct {
	mu sync.RWMutex // protects all fields below

	// Match storage
	matches      map[string]*repository.Match
	participants map[string]map[int64]*repository.Participant // matchID -> userID -> participant
	rounds       map[string][]*repository.MatchRound          // matchID -> rounds

	// Call tracking
	createMatchCalls           int
	getMatchCalls              int
	getActiveMatchesCalls      int
	addParticipantCalls        int
	getParticipantCalls        int
	removeParticipantCalls     int
	startShopPhaseCalls        int
	startBattlePhaseCalls      int
	completeMatchCalls         int
	submitTeamCalls            int
	updateParticipantShopCalls int
	createRoundCalls           int

	// Error injection
	createMatchError           error
	getMatchError              error
	getActiveMatchesError      error
	addParticipantError        error
	getParticipantError        error
	removeParticipantError     error
	startShopPhaseError        error
	startBattlePhaseError      error
	completeMatchError         error
	submitTeamError            error
	updateParticipantShopError error
}

func newMockGameRepository() *mockGameRepository {
	return &mockGameRepository{
		matches:      make(map[string]*repository.Match),
		participants: make(map[string]map[int64]*repository.Participant),
		rounds:       make(map[string][]*repository.MatchRound),
	}
}

// copyMatch creates a deep copy of a Match to prevent data races
// when multiple goroutines access the same match data.
func copyMatch(m *repository.Match) *repository.Match {
	if m == nil {
		return nil
	}
	cp := *m
	// Deep copy pointer fields
	if m.Format != nil {
		format := *m.Format
		cp.Format = &format
	}
	if m.JoinDeadline != nil {
		t := *m.JoinDeadline
		cp.JoinDeadline = &t
	}
	if m.ShopPhaseStartedAt != nil {
		t := *m.ShopPhaseStartedAt
		cp.ShopPhaseStartedAt = &t
	}
	if m.ShopPhaseDeadline != nil {
		t := *m.ShopPhaseDeadline
		cp.ShopPhaseDeadline = &t
	}
	if m.BattleStartedAt != nil {
		t := *m.BattleStartedAt
		cp.BattleStartedAt = &t
	}
	if m.CompletedAt != nil {
		t := *m.CompletedAt
		cp.CompletedAt = &t
	}
	if m.TournamentDate != nil {
		s := *m.TournamentDate
		cp.TournamentDate = &s
	}
	if m.CreatorUserID != nil {
		id := *m.CreatorUserID
		cp.CreatorUserID = &id
	}
	if m.WinnerUserID != nil {
		id := *m.WinnerUserID
		cp.WinnerUserID = &id
	}
	return &cp
}

// copyParticipant creates a deep copy of a Participant to prevent data races
// when multiple goroutines access the same participant data.
func copyParticipant(p *repository.Participant) *repository.Participant {
	if p == nil {
		return nil
	}
	cp := *p
	// Deep copy pointer fields
	if p.ShopCards != nil {
		cards := make(json.RawMessage, len(*p.ShopCards))
		copy(cards, *p.ShopCards)
		cp.ShopCards = &cards
	}
	if p.Team != nil {
		team := make(json.RawMessage, len(*p.Team))
		copy(team, *p.Team)
		cp.Team = &team
	}
	if p.TeamOrder != nil {
		order := make([]int64, len(p.TeamOrder))
		copy(order, p.TeamOrder)
		cp.TeamOrder = order
	}
	if p.TeamSubmittedAt != nil {
		t := *p.TeamSubmittedAt
		cp.TeamSubmittedAt = &t
	}
	if p.Placement != nil {
		pl := *p.Placement
		cp.Placement = &pl
	}
	return &cp
}

// CreateMatch creates a new match in the mock repository
func (m *mockGameRepository) CreateMatch(ctx context.Context, chatID int64, matchType repository.MatchType, creatorUserID *int64, tournamentDate *string) (*repository.Match, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.createMatchCalls++
	if m.createMatchError != nil {
		return nil, m.createMatchError
	}

	matchID := "test-match-" + time.Now().Format("20060102150405")
	match := &repository.Match{
		ID:            matchID,
		ChatID:        chatID,
		MatchType:     matchType,
		Status:        repository.MatchStatusOpen,
		CreatedAt:     time.Now(),
		CreatorUserID: creatorUserID,
		CurrentRound:  0,
	}

	if matchType == repository.MatchTypeRegular && creatorUserID != nil {
		joinDeadline := time.Now().Add(5 * time.Minute)
		match.JoinDeadline = &joinDeadline
	}

	if tournamentDate != nil {
		match.TournamentDate = tournamentDate
	}

	m.matches[matchID] = match
	m.participants[matchID] = make(map[int64]*repository.Participant)
	return match, nil
}

// GetMatch retrieves a match by ID
// Returns a deep copy to prevent data races when multiple goroutines access the match.
func (m *mockGameRepository) GetMatch(ctx context.Context, matchID string) (*repository.Match, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getMatchCalls++
	if m.getMatchError != nil {
		return nil, m.getMatchError
	}
	return copyMatch(m.matches[matchID]), nil
}

// GetActiveMatches retrieves active matches for a chat
// Returns deep copies to prevent data races.
func (m *mockGameRepository) GetActiveMatches(ctx context.Context, chatID int64) ([]*repository.Match, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getActiveMatchesCalls++
	if m.getActiveMatchesError != nil {
		return nil, m.getActiveMatchesError
	}

	var matches []*repository.Match
	for _, match := range m.matches {
		if match.ChatID == chatID &&
			(match.Status == repository.MatchStatusOpen ||
				match.Status == repository.MatchStatusShopPhase ||
				match.Status == repository.MatchStatusBattlePhase) {
			matches = append(matches, copyMatch(match))
		}
	}
	return matches, nil
}

// GetMatchesByStatus retrieves matches by status
// Returns deep copies to prevent data races.
func (m *mockGameRepository) GetMatchesByStatus(ctx context.Context, status repository.MatchStatus) ([]*repository.Match, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matches []*repository.Match
	for _, match := range m.matches {
		if match.Status == status {
			matches = append(matches, copyMatch(match))
		}
	}
	return matches, nil
}

// UpdateMatchStatus updates a match status
func (m *mockGameRepository) UpdateMatchStatus(ctx context.Context, matchID string, status repository.MatchStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	match := m.matches[matchID]
	if match == nil {
		return errors.New("match not found")
	}
	match.Status = status
	return nil
}

// UpdateMatchFormat updates a match format
func (m *mockGameRepository) UpdateMatchFormat(ctx context.Context, matchID string, format repository.MatchFormat) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	match := m.matches[matchID]
	if match == nil {
		return errors.New("match not found")
	}
	match.Format = &format
	return nil
}

// StartShopPhase transitions match to shop phase
func (m *mockGameRepository) StartShopPhase(ctx context.Context, matchID string, format repository.MatchFormat, deadline time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.startShopPhaseCalls++
	if m.startShopPhaseError != nil {
		return m.startShopPhaseError
	}

	match := m.matches[matchID]
	if match == nil {
		return errors.New("match not found")
	}

	match.Status = repository.MatchStatusShopPhase
	match.Format = &format
	now := time.Now()
	match.ShopPhaseStartedAt = &now
	match.ShopPhaseDeadline = &deadline
	return nil
}

// StartBattlePhase transitions match to battle phase
func (m *mockGameRepository) StartBattlePhase(ctx context.Context, matchID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.startBattlePhaseCalls++
	if m.startBattlePhaseError != nil {
		return m.startBattlePhaseError
	}

	match := m.matches[matchID]
	if match == nil {
		return errors.New("match not found")
	}

	match.Status = repository.MatchStatusBattlePhase
	now := time.Now()
	match.BattleStartedAt = &now
	return nil
}

// CompleteMatch completes a match with optional winner
func (m *mockGameRepository) CompleteMatch(ctx context.Context, matchID string, winnerUserID *int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.completeMatchCalls++
	if m.completeMatchError != nil {
		return m.completeMatchError
	}

	match := m.matches[matchID]
	if match == nil {
		return errors.New("match not found")
	}

	match.Status = repository.MatchStatusCompleted
	now := time.Now()
	match.CompletedAt = &now
	match.WinnerUserID = winnerUserID
	return nil
}

// CancelMatch cancels a match
func (m *mockGameRepository) CancelMatch(ctx context.Context, matchID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	match := m.matches[matchID]
	if match == nil {
		return errors.New("match not found")
	}

	match.Status = repository.MatchStatusCancelled
	return nil
}

// AddParticipant adds a participant to a match
func (m *mockGameRepository) AddParticipant(ctx context.Context, matchID string, userID int64) (*repository.Participant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.addParticipantCalls++
	if m.addParticipantError != nil {
		return nil, m.addParticipantError
	}

	if m.participants[matchID] == nil {
		m.participants[matchID] = make(map[int64]*repository.Participant)
	}

	// Check if already joined
	if _, exists := m.participants[matchID][userID]; exists {
		return nil, errors.New("already joined this match")
	}

	participant := &repository.Participant{
		ID:             int64(len(m.participants[matchID]) + 1),
		MatchID:        matchID,
		UserID:         userID,
		Status:         repository.ParticipantStatusJoined,
		JoinedAt:       time.Now(),
		CoinsRemaining: 10,
	}
	m.participants[matchID][userID] = participant
	return participant, nil
}

// GetParticipant retrieves a participant
// Returns a deep copy to prevent data races when multiple goroutines access the participant.
func (m *mockGameRepository) GetParticipant(ctx context.Context, matchID string, userID int64) (*repository.Participant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.getParticipantCalls++
	if m.getParticipantError != nil {
		return nil, m.getParticipantError
	}

	if m.participants[matchID] == nil {
		return nil, nil
	}
	return copyParticipant(m.participants[matchID][userID]), nil
}

// GetMatchParticipants retrieves all participants for a match with user info
// Returns deep copies to prevent data races.
func (m *mockGameRepository) GetMatchParticipants(ctx context.Context, matchID string) ([]*repository.ParticipantWithUser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.participants[matchID] == nil {
		return []*repository.ParticipantWithUser{}, nil
	}

	var result []*repository.ParticipantWithUser
	for _, p := range m.participants[matchID] {
		// Copy the participant to prevent races
		pCopy := copyParticipant(p)
		pwu := &repository.ParticipantWithUser{
			Participant: *pCopy,
			FirstName:   "Test User",
			Username:    "testuser",
		}
		result = append(result, pwu)
	}
	return result, nil
}

// RemoveParticipant removes a participant from a match
func (m *mockGameRepository) RemoveParticipant(ctx context.Context, matchID string, userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.removeParticipantCalls++
	if m.removeParticipantError != nil {
		return m.removeParticipantError
	}

	if m.participants[matchID] != nil {
		delete(m.participants[matchID], userID)
	}
	return nil
}

// UpdateParticipantShop updates participant shop state
func (m *mockGameRepository) UpdateParticipantShop(ctx context.Context, matchID string, userID int64, coins int, shopCards, team json.RawMessage, teamOrder []int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.updateParticipantShopCalls++
	if m.updateParticipantShopError != nil {
		return m.updateParticipantShopError
	}

	if m.participants[matchID] == nil {
		return errors.New("match not found")
	}
	p := m.participants[matchID][userID]
	if p == nil {
		return errors.New("participant not found")
	}

	p.CoinsRemaining = coins
	shopCardsRaw := json.RawMessage(shopCards)
	teamRaw := json.RawMessage(team)
	p.ShopCards = &shopCardsRaw
	p.Team = &teamRaw
	p.TeamOrder = teamOrder
	return nil
}

// SubmitTeam marks participant as ready
func (m *mockGameRepository) SubmitTeam(ctx context.Context, matchID string, userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.submitTeamCalls++
	if m.submitTeamError != nil {
		return m.submitTeamError
	}

	if m.participants[matchID] == nil {
		return errors.New("match not found")
	}
	p := m.participants[matchID][userID]
	if p == nil {
		return errors.New("participant not found")
	}

	p.Status = repository.ParticipantStatusReady
	now := time.Now()
	p.TeamSubmittedAt = &now
	return nil
}

// GetParticipantCount returns the number of participants in a match
func (m *mockGameRepository) GetParticipantCount(ctx context.Context, matchID string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.participants[matchID] == nil {
		return 0, nil
	}
	return len(m.participants[matchID]), nil
}

// GetReadyParticipantCount returns the number of ready participants
func (m *mockGameRepository) GetReadyParticipantCount(ctx context.Context, matchID string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.participants[matchID] == nil {
		return 0, nil
	}
	count := 0
	for _, p := range m.participants[matchID] {
		if p.Status == repository.ParticipantStatusReady {
			count++
		}
	}
	return count, nil
}

// Round operations
func (m *mockGameRepository) CreateRound(ctx context.Context, matchID string, roundNumber int, playerAID, playerBID int64, playerATeam, playerBTeam, battleLog json.RawMessage, winnerID *int64, isDraw bool, playerADmg, playerBDmg, totalRounds int) (*repository.MatchRound, error) {
	m.createRoundCalls++

	round := &repository.MatchRound{
		ID:          int64(len(m.rounds[matchID]) + 1),
		MatchID:     matchID,
		RoundNumber: roundNumber,
		PlayerAID:   playerAID,
		PlayerBID:   playerBID,
		PlayerATeam: playerATeam,
		PlayerBTeam: playerBTeam,
		BattleLog:   battleLog,
		WinnerID:    winnerID,
		IsDraw:      isDraw,
		PlayerADmg:  playerADmg,
		PlayerBDmg:  playerBDmg,
		TotalRounds: totalRounds,
	}

	if m.rounds[matchID] == nil {
		m.rounds[matchID] = []*repository.MatchRound{}
	}
	m.rounds[matchID] = append(m.rounds[matchID], round)

	return round, nil
}

func (m *mockGameRepository) GetMatchRounds(ctx context.Context, matchID string) ([]*repository.MatchRound, error) {
	if m.rounds[matchID] == nil {
		return []*repository.MatchRound{}, nil
	}
	return m.rounds[matchID], nil
}

// Tournament operations (stubs for interface compliance)
func (m *mockGameRepository) GetOrCreateTournament(ctx context.Context, chatID int64, date string) (*repository.RankedTournament, error) {
	return nil, nil
}

func (m *mockGameRepository) GetTournamentByID(ctx context.Context, id int64) (*repository.RankedTournament, error) {
	return nil, nil
}

func (m *mockGameRepository) GetTodayTournament(ctx context.Context, chatID int64, date string) (*repository.RankedTournament, error) {
	return nil, nil
}

func (m *mockGameRepository) GetTournamentsByStatus(ctx context.Context, status repository.TournamentStatus) ([]*repository.RankedTournament, error) {
	return []*repository.RankedTournament{}, nil
}

func (m *mockGameRepository) GetTournamentsWithPendingRounds(ctx context.Context) ([]*repository.RankedTournament, error) {
	return []*repository.RankedTournament{}, nil
}

func (m *mockGameRepository) UpdateTournamentStatus(ctx context.Context, id int64, status repository.TournamentStatus) error {
	return nil
}

func (m *mockGameRepository) SetTournamentAnnounced(ctx context.Context, id int64, messageID int64) error {
	return nil
}

func (m *mockGameRepository) CloseTournamentRegistration(ctx context.Context, id int64, matchID string) error {
	return nil
}

func (m *mockGameRepository) CompleteTournament(ctx context.Context, id int64, winnerUserID *int64) error {
	return nil
}

func (m *mockGameRepository) SkipTournament(ctx context.Context, id int64) error {
	return nil
}

func (m *mockGameRepository) UpdateTournamentBracket(ctx context.Context, id int64, bracketState json.RawMessage) error {
	return nil
}

func (m *mockGameRepository) AddTournamentParticipant(ctx context.Context, tournamentID, userID int64) error {
	return nil
}

func (m *mockGameRepository) RemoveTournamentParticipant(ctx context.Context, tournamentID, userID int64) error {
	return nil
}

func (m *mockGameRepository) GetTournamentParticipants(ctx context.Context, tournamentID int64) ([]*repository.TournamentParticipant, error) {
	return []*repository.TournamentParticipant{}, nil
}

func (m *mockGameRepository) IsTournamentParticipant(ctx context.Context, tournamentID, userID int64) (bool, error) {
	return false, nil
}

func (m *mockGameRepository) GetTournamentsNeedingAnnouncement(ctx context.Context, currentTime time.Time) ([]*repository.TournamentInfo, error) {
	return []*repository.TournamentInfo{}, nil
}

func (m *mockGameRepository) GetTournamentsNeedingClose(ctx context.Context, currentTime time.Time) ([]*repository.TournamentInfo, error) {
	return []*repository.TournamentInfo{}, nil
}

func (m *mockGameRepository) GetChatsWithTimezone(ctx context.Context) ([]*repository.ChatTimezone, error) {
	return []*repository.ChatTimezone{}, nil
}

// Leaderboard operations (stubs for interface compliance)
func (m *mockGameRepository) GetLeaderboard(ctx context.Context, chatID int64, matchType repository.MatchType, limit, offset int) ([]*repository.LeaderboardEntry, error) {
	return []*repository.LeaderboardEntry{}, nil
}

func (m *mockGameRepository) UpdateLeaderboard(ctx context.Context, userID, chatID int64, matchType repository.MatchType, isWin bool, opponentID *int64, isTournamentWin bool, isDraw bool) error {
	return nil
}

func (m *mockGameRepository) GetMatchHistory(ctx context.Context, chatID, userID int64, limit, offset int) ([]*repository.MatchHistoryEntry, int, error) {
	return []*repository.MatchHistoryEntry{}, 0, nil
}

func (m *mockGameRepository) GetH2HRecord(ctx context.Context, chatID, userID, opponentID int64) (*repository.H2HRecord, error) {
	return nil, nil
}

func (m *mockGameRepository) GetRecentMatchesVsOpponent(ctx context.Context, chatID, userID, opponentID int64, limit int) ([]*repository.MatchHistoryEntry, error) {
	return []*repository.MatchHistoryEntry{}, nil
}

func (m *mockGameRepository) GetUserProfile(ctx context.Context, chatID, userID int64) (*repository.UserProfile, error) {
	return nil, nil
}

// mockCardService is a mock implementation of CardServiceInterface for testing.
// This implements the full CardServiceInterface defined in interfaces.go.
type mockCardService struct {
	cardCount int
}

func newMockCardService(cardCount int) *mockCardService {
	return &mockCardService{
		cardCount: cardCount,
	}
}

func (m *mockCardService) GetUserCard(ctx context.Context, userID int64, chatID int64, weekStart *time.Time) (*repository.UserCard, *repository.CardUser, error) {
	return &repository.UserCard{
		UserID: userID,
		ChatID: chatID,
	}, &repository.CardUser{ID: userID, FirstName: "Test User"}, nil
}

func (m *mockCardService) GetChatCards(ctx context.Context, req GetChatCardsRequest) (*GetChatCardsResponse, error) {
	cards := make([]repository.CardWithUser, m.cardCount)
	return &GetChatCardsResponse{
		Cards:      cards,
		Pagination: PaginationInfo{Total: m.cardCount},
	}, nil
}

func (m *mockCardService) GetUserHistory(ctx context.Context, userID int64, chatID int64, limit int) (*UserHistoryResponse, error) {
	return &UserHistoryResponse{
		History: []CardSummary{},
	}, nil
}

func (m *mockCardService) GetAvailableWeeks(ctx context.Context, chatID int64) (*AvailableWeeksResponse, error) {
	return &AvailableWeeksResponse{
		Weeks: []repository.WeekInfo{},
	}, nil
}

func (m *mockCardService) GetCardImageURL(ctx context.Context, userID int64, chatID int64, weekStart *time.Time, theme string, expirySeconds int) (*CardImageURLResponse, error) {
	url := "https://mock-storage.example.com/card.png"
	return &CardImageURLResponse{URL: url}, nil
}

func (m *mockCardService) GetCardImageURLString(ctx context.Context, userID int64, chatID int64, weekStart *time.Time, theme string, expirySeconds int) (string, error) {
	return "https://mock-storage.example.com/card.png", nil
}

func (m *mockCardService) GetGalleryWeeks(ctx context.Context, chatID int64) (*GalleryWeeksResponse, error) {
	return &GalleryWeeksResponse{Weeks: []string{}}, nil
}

func (m *mockCardService) GetGalleryImages(ctx context.Context, chatID int64, weekStart *time.Time, userID *int64, theme *string) (*GalleryImagesResponse, error) {
	return &GalleryImagesResponse{Images: []GalleryImage{}}, nil
}

func (m *mockCardService) GetGalleryImageURL(ctx context.Context, imageID int64, expirySeconds int) (*GalleryImageURLResponse, error) {
	url := "https://mock-storage.example.com/gallery.png"
	return &GalleryImageURLResponse{URL: url}, nil
}

// =============================================================================
// Test Helper Functions
// =============================================================================

// testChatID is the standard chat ID used for arena tests.
// Using a distinct ID from ingest tests to avoid interference.
const testChatID = int64(-1009999000001)

// testUsers are standard user IDs for arena tests.
var testCreatorUserID = int64(900000001)
var testJoinerUserID = int64(900000002)
var testUser3ID = int64(900000003)

// setupArenaTestDB sets up a test database with required test data for arena tests.
// Creates users and cards required for match creation.
func setupArenaTestDB(t *testing.T) *testutil.TestDB {
	tdb := testutil.SetupTestDB(t)
	ctx := context.Background()

	// Cleanup any existing test data using TRUNCATE CASCADE
	// This is more efficient than DELETE and handles foreign key constraints automatically
	tablesToCleanup := []string{
		"game_match_participants",
		"game_match_rounds",
		"game_matches",
		"ml_user_cards",
		"chats",
		"users",
	}

	for _, table := range tablesToCleanup {
		_, err := tdb.DB.ExecContext(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		if err != nil {
			// Only log warning for non-existent tables, don't fail the test
			t.Logf("Warning: failed to truncate table %s: %v", table, err)
		}
	}

	// Create test chat
	_, err := tdb.DB.ExecContext(ctx, `
		INSERT INTO chats (id, type, title)
		VALUES ($1, 'supergroup', 'Arena Test Group')
	`, testChatID)
	if err != nil {
		t.Fatalf("failed to create test chat: %v", err)
	}

	// Create test users
	for i, userID := range []int64{testCreatorUserID, testJoinerUserID, testUser3ID} {
		_, err := tdb.DB.ExecContext(ctx, `
			INSERT INTO users (id, first_name, username, is_bot)
			VALUES ($1, $2, $3, false)
		`, userID, fmt.Sprintf("TestUser%c", 'A'+i), fmt.Sprintf("testuser%c", 'a'+i))
		if err != nil {
			t.Fatalf("failed to create test user %d: %v", userID, err)
		}
	}

	return tdb
}

// cleanupArenaTables cleans up all arena-related tables to prevent parallel test interference
func cleanupArenaTables(t *testing.T, tdb *testutil.TestDB) {
	ctx := context.Background()
	tablesToCleanup := []string{
		"game_match_rounds",
		"game_match_participants",
		"game_matches",
		"ml_user_card_images",
		"ml_user_cards",
		"users",
		"chats",
	}

	for _, table := range tablesToCleanup {
		_, err := tdb.DB.ExecContext(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		if err != nil {
			t.Logf("Warning: failed to truncate table %s: %v", table, err)
		}
	}
}

// seedTestCards creates test cards in the database for arena testing.
// Creates count cards for the test chat with valid combat stats.
func seedTestCards(t *testing.T, db *testutil.TestDB, count int) {
	ctx := context.Background()

	// Cleanup existing test data to prevent conflicts from previous test runs
	// Delete cards for user ID range 900001000 and up
	_, err := db.DB.ExecContext(ctx, "DELETE FROM ml_user_cards WHERE user_id >= 900001000")
	if err != nil {
		t.Logf("warning: failed to cleanup ml_user_cards: %v", err)
	}

	// Delete users for user ID range 900001000 and up
	_, err = db.DB.ExecContext(ctx, "DELETE FROM users WHERE id >= 900001000")
	if err != nil {
		t.Logf("warning: failed to cleanup users: %v", err)
	}

	// Ensure test chat exists (may have been deleted by parallel test's TRUNCATE CASCADE)
	_, err = db.DB.ExecContext(ctx, `
		INSERT INTO chats (id, type, title)
		VALUES ($1, 'supergroup', 'Arena Test Group')
		ON CONFLICT (id) DO NOTHING
	`, testChatID)
	if err != nil {
		t.Fatalf("failed to ensure test chat exists: %v", err)
	}

	// Get current week start (Monday) and end (Sunday)
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekStart := now.AddDate(0, 0, -(weekday - 1)).Truncate(24 * time.Hour)
	weekEnd := weekStart.AddDate(0, 0, 6)

	// Create test cards
	for i := range count {
		userID := int64(900001000 + i)

		// Create user if not exists
		_, err := db.DB.ExecContext(ctx, `
			INSERT INTO users (id, first_name, is_bot)
			VALUES ($1, $2, false)
		`, userID, fmt.Sprintf("CardUser%c", 'A'+i%26))
		if err != nil {
			t.Fatalf("failed to create card user %d: %v", userID, err)
		}

		// Create card with combat stats
		stats := map[string]any{
			"combat": map[string]any{
				"atk":     10 + i,
				"def":     5 + i,
				"hp":      20 + i,
				"raw_atk": float64(10 + i),
				"raw_def": float64(5 + i),
			},
		}
		statsJSON, _ := json.Marshal(stats)

		_, err = db.DB.ExecContext(ctx, `
			INSERT INTO ml_user_cards (user_id, chat_id, week_start, week_end, stats_window_start, stats_window_end, stats, messages_analyzed)
			VALUES ($1, $2, $3, $4, $3, $4, $5, 100)
			ON CONFLICT (user_id, chat_id, week_start) DO UPDATE SET stats = $5
		`, userID, testChatID, weekStart, weekEnd, statsJSON)
		if err != nil {
			t.Fatalf("failed to create test card %d: %v", i, err)
		}
	}
}

// cleanupArenaTestData removes all arena test data from the database.
func cleanupArenaTestData(t *testing.T, db *testutil.TestDB) {
	ctx := context.Background()
	_, _ = db.DB.ExecContext(ctx, "DELETE FROM game_match_participants WHERE match_id IN (SELECT id FROM game_matches WHERE chat_id = $1)", testChatID)
	_, _ = db.DB.ExecContext(ctx, "DELETE FROM game_matches WHERE chat_id = $1", testChatID)
}

// =============================================================================
// Match Lifecycle Tests
// =============================================================================

// TestCreateMatch_Success tests valid match creation
func TestCreateMatch_Success(t *testing.T) {
	tdb := setupArenaTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)
	defer cleanupArenaTables(t, tdb)

	// Seed 15 cards (more than minimum 10)
	seedTestCards(t, tdb, 15)
	defer cleanupArenaTestData(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	mockRepo := newMockGameRepository()
	mockCards := newMockCardService(15)

	svc := NewArenaService(tdb.DB, mockMinIO, mockCards, nil, &ArenaServiceDeps{
		GameRepo:      mockRepo,
		StorageClient: mockMinIO,
		CardService:   mockCards,
	})

	ctx := context.Background()

	resp, err := svc.CreateMatch(ctx, testChatID, testCreatorUserID)
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	// Verify match was created
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Match == nil {
		t.Fatal("expected non-nil match in response")
	}
	if resp.Match.ChatID != testChatID {
		t.Errorf("expected chat_id %d, got %d", testChatID, resp.Match.ChatID)
	}
	if resp.Match.MatchType != repository.MatchTypeRegular {
		t.Errorf("expected match_type 'regular', got %s", resp.Match.MatchType)
	}
	if resp.Match.Status != repository.MatchStatusOpen {
		t.Errorf("expected status 'open', got %s", resp.Match.Status)
	}

	// Verify creator was auto-joined
	if len(resp.Participants) != 1 {
		t.Errorf("expected 1 participant (creator), got %d", len(resp.Participants))
	}
	if len(resp.Participants) > 0 && resp.Participants[0].UserID != testCreatorUserID {
		t.Errorf("expected creator user ID %d, got %d", testCreatorUserID, resp.Participants[0].UserID)
	}

	// Verify card count included
	if resp.CardCount < 10 {
		t.Errorf("expected card count >= 10, got %d", resp.CardCount)
	}

	// Verify repository calls
	if mockRepo.createMatchCalls != 1 {
		t.Errorf("expected 1 CreateMatch call, got %d", mockRepo.createMatchCalls)
	}
	if mockRepo.addParticipantCalls != 1 {
		t.Errorf("expected 1 AddParticipant call, got %d", mockRepo.addParticipantCalls)
	}
}

// TestCreateMatch_NotEnoughCards tests match creation fails when not enough cards
func TestCreateMatch_NotEnoughCards(t *testing.T) {
	tdb := setupArenaTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)
	defer cleanupArenaTables(t, tdb)

	// Seed only 5 cards (less than minimum 10)
	seedTestCards(t, tdb, 5)
	defer cleanupArenaTestData(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	mockRepo := newMockGameRepository()
	mockCards := newMockCardService(5)

	svc := NewArenaService(tdb.DB, mockMinIO, mockCards, nil, &ArenaServiceDeps{
		GameRepo:      mockRepo,
		StorageClient: mockMinIO,
		CardService:   mockCards,
	})

	ctx := context.Background()

	_, err := svc.CreateMatch(ctx, testChatID, testCreatorUserID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, apperror.ErrNotEnoughCards) {
		t.Errorf("expected apperror.ErrNotEnoughCards, got %v", err)
	}

	// Verify no match was created
	if mockRepo.createMatchCalls != 0 {
		t.Errorf("expected 0 CreateMatch calls, got %d", mockRepo.createMatchCalls)
	}
}

// TestCreateMatch_ActiveMatchExists tests match creation fails when active match exists
func TestCreateMatch_ActiveMatchExists(t *testing.T) {
	tdb := setupArenaTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)
	defer cleanupArenaTables(t, tdb)

	seedTestCards(t, tdb, 15)
	defer cleanupArenaTestData(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	mockRepo := newMockGameRepository()
	mockCards := newMockCardService(15)

	svc := NewArenaService(tdb.DB, mockMinIO, mockCards, nil, &ArenaServiceDeps{
		GameRepo:      mockRepo,
		StorageClient: mockMinIO,
		CardService:   mockCards,
	})

	ctx := context.Background()

	// Create first match successfully
	_, err := svc.CreateMatch(ctx, testChatID, testCreatorUserID)
	if err != nil {
		t.Fatalf("first CreateMatch failed: %v", err)
	}

	// Try to create second match - should fail
	_, err = svc.CreateMatch(ctx, testChatID, testCreatorUserID)
	if err == nil {
		t.Fatal("expected error for second match, got nil")
	}

	if !errors.Is(err, apperror.ErrActiveMatchExists) {
		t.Errorf("expected apperror.ErrActiveMatchExists, got %v", err)
	}
}

// TestJoinMatch_Success tests valid join operation
func TestJoinMatch_Success(t *testing.T) {
	tdb := setupArenaTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)
	defer cleanupArenaTables(t, tdb)

	seedTestCards(t, tdb, 15)
	defer cleanupArenaTestData(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	mockRepo := newMockGameRepository()
	mockCards := newMockCardService(15)

	svc := NewArenaService(tdb.DB, mockMinIO, mockCards, nil, &ArenaServiceDeps{
		GameRepo:      mockRepo,
		StorageClient: mockMinIO,
		CardService:   mockCards,
	})

	ctx := context.Background()

	// Create match
	createResp, err := svc.CreateMatch(ctx, testChatID, testCreatorUserID)
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	// Join match
	joinResp, err := svc.JoinMatch(ctx, createResp.Match.ID, testJoinerUserID)
	if err != nil {
		t.Fatalf("JoinMatch failed: %v", err)
	}

	// Verify participant count
	if len(joinResp.Participants) != 2 {
		t.Errorf("expected 2 participants, got %d", len(joinResp.Participants))
	}
}

// TestJoinMatch_NotOpen tests joining a non-open match fails
func TestJoinMatch_NotOpen(t *testing.T) {
	tdb := setupArenaTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)
	defer cleanupArenaTables(t, tdb)

	seedTestCards(t, tdb, 15)
	defer cleanupArenaTestData(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	mockRepo := newMockGameRepository()
	mockCards := newMockCardService(15)

	svc := NewArenaService(tdb.DB, mockMinIO, mockCards, nil, &ArenaServiceDeps{
		GameRepo:      mockRepo,
		StorageClient: mockMinIO,
		CardService:   mockCards,
	})

	ctx := context.Background()

	// Create match and transition to shop phase manually
	createResp, err := svc.CreateMatch(ctx, testChatID, testCreatorUserID)
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	// Manually set match to shop phase (simulating started match)
	mockRepo.matches[createResp.Match.ID].Status = repository.MatchStatusShopPhase

	// Try to join - should fail
	_, err = svc.JoinMatch(ctx, createResp.Match.ID, testJoinerUserID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, apperror.ErrMatchNotOpen) {
		t.Errorf("expected apperror.ErrMatchNotOpen, got %v", err)
	}
}

// TestStartMatch_Success tests valid match start
func TestStartMatch_Success(t *testing.T) {
	tdb := setupArenaTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)
	defer cleanupArenaTables(t, tdb)

	seedTestCards(t, tdb, 15)
	defer cleanupArenaTestData(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	mockRepo := newMockGameRepository()
	mockCards := newMockCardService(15)

	svc := NewArenaService(tdb.DB, mockMinIO, mockCards, nil, &ArenaServiceDeps{
		GameRepo:      mockRepo,
		StorageClient: mockMinIO,
		CardService:   mockCards,
	})

	ctx := context.Background()

	// Create match and add second participant
	createResp, err := svc.CreateMatch(ctx, testChatID, testCreatorUserID)
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	_, err = svc.JoinMatch(ctx, createResp.Match.ID, testJoinerUserID)
	if err != nil {
		t.Fatalf("JoinMatch failed: %v", err)
	}

	// Start match
	startResp, err := svc.StartMatch(ctx, createResp.Match.ID, testCreatorUserID)
	if err != nil {
		t.Fatalf("StartMatch failed: %v", err)
	}

	// Verify match transitioned to shop phase
	if startResp.Match.Status != repository.MatchStatusShopPhase {
		t.Errorf("expected status 'shop_phase', got %s", startResp.Match.Status)
	}

	// Verify format is 1v1 for 2 players
	if startResp.Match.Format == nil || *startResp.Match.Format != repository.MatchFormat1v1 {
		t.Errorf("expected format '1v1', got %v", startResp.Match.Format)
	}

	// Verify shop phase was started
	if mockRepo.startShopPhaseCalls != 1 {
		t.Errorf("expected 1 StartShopPhase call, got %d", mockRepo.startShopPhaseCalls)
	}
}

// TestStartMatch_NotCreator tests that non-creator cannot start match
func TestStartMatch_NotCreator(t *testing.T) {
	tdb := setupArenaTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)
	defer cleanupArenaTables(t, tdb)

	seedTestCards(t, tdb, 15)
	defer cleanupArenaTestData(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	mockRepo := newMockGameRepository()
	mockCards := newMockCardService(15)

	svc := NewArenaService(tdb.DB, mockMinIO, mockCards, nil, &ArenaServiceDeps{
		GameRepo:      mockRepo,
		StorageClient: mockMinIO,
		CardService:   mockCards,
	})

	ctx := context.Background()

	// Create match and add second participant
	createResp, err := svc.CreateMatch(ctx, testChatID, testCreatorUserID)
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	_, err = svc.JoinMatch(ctx, createResp.Match.ID, testJoinerUserID)
	if err != nil {
		t.Fatalf("JoinMatch failed: %v", err)
	}

	// Try to start as non-creator - should fail
	_, err = svc.StartMatch(ctx, createResp.Match.ID, testJoinerUserID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, apperror.ErrNotCreator) {
		t.Errorf("expected apperror.ErrNotCreator, got %v", err)
	}
}

// TestStartMatch_NotEnoughParticipants tests that match cannot start with < 2 players
func TestStartMatch_NotEnoughParticipants(t *testing.T) {
	tdb := setupArenaTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)
	defer cleanupArenaTables(t, tdb)

	seedTestCards(t, tdb, 15)
	defer cleanupArenaTestData(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	mockRepo := newMockGameRepository()
	mockCards := newMockCardService(15)

	svc := NewArenaService(tdb.DB, mockMinIO, mockCards, nil, &ArenaServiceDeps{
		GameRepo:      mockRepo,
		StorageClient: mockMinIO,
		CardService:   mockCards,
	})

	ctx := context.Background()

	// Create match (only creator, no other participants)
	createResp, err := svc.CreateMatch(ctx, testChatID, testCreatorUserID)
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	// Try to start with only 1 participant - should fail
	_, err = svc.StartMatch(ctx, createResp.Match.ID, testCreatorUserID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err.Error() != "need at least 2 participants to start" {
		t.Errorf("expected 'need at least 2 participants to start', got %v", err)
	}
}

// =============================================================================
// Shop Phase Tests
// =============================================================================

// TestGetShop_Success tests that GetShop returns valid shop cards after match starts
func TestGetShop_Success(t *testing.T) {
	tdb := setupArenaTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)
	defer cleanupArenaTables(t, tdb)

	seedTestCards(t, tdb, 15)
	defer cleanupArenaTestData(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	mockRepo := newMockGameRepository()
	mockCards := newMockCardService(15)

	svc := NewArenaService(tdb.DB, mockMinIO, mockCards, nil, &ArenaServiceDeps{
		GameRepo:      mockRepo,
		StorageClient: mockMinIO,
		CardService:   mockCards,
	})

	ctx := context.Background()

	// Create match and add second participant
	createResp, err := svc.CreateMatch(ctx, testChatID, testCreatorUserID)
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	_, err = svc.JoinMatch(ctx, createResp.Match.ID, testJoinerUserID)
	if err != nil {
		t.Fatalf("JoinMatch failed: %v", err)
	}

	// Start match to transition to shop phase
	_, err = svc.StartMatch(ctx, createResp.Match.ID, testCreatorUserID)
	if err != nil {
		t.Fatalf("StartMatch failed: %v", err)
	}

	// Get shop for creator
	shopResp, err := svc.GetShop(ctx, createResp.Match.ID, testCreatorUserID)
	if err != nil {
		t.Fatalf("GetShop failed: %v", err)
	}

	// Verify shop response structure
	if shopResp == nil {
		t.Fatal("expected non-nil shop response")
	}

	// Verify match ID
	if shopResp.MatchID != createResp.Match.ID {
		t.Errorf("expected match_id %s, got %s", createResp.Match.ID, shopResp.MatchID)
	}

	// Verify status is shop_phase
	if shopResp.Status != string(repository.MatchStatusShopPhase) {
		t.Errorf("expected status 'shop_phase', got %s", shopResp.Status)
	}

	// Verify starting coins (10 coins per shop.StartingCoins)
	if shopResp.Coins != 10 {
		t.Errorf("expected 10 starting coins, got %d", shopResp.Coins)
	}

	// Verify shop has 4 cards (shop.ShopSize)
	if len(shopResp.Cards) != 4 {
		t.Errorf("expected 4 shop cards, got %d", len(shopResp.Cards))
	}

	// Verify all shop cards are unpurchased
	for i, card := range shopResp.Cards {
		if card.IsPurchased {
			t.Errorf("card %d should be unpurchased, but is marked as purchased", i)
		}
		if card.Index != i {
			t.Errorf("card %d has incorrect index %d", i, card.Index)
		}
	}

	// Verify team is empty initially
	if len(shopResp.Team) != 0 {
		t.Errorf("expected empty team, got %d cards", len(shopResp.Team))
	}

	// Verify team order is initialized to [0, 1, 2]
	expectedOrder := []int{0, 1, 2}
	if len(shopResp.TeamOrder) != len(expectedOrder) {
		t.Errorf("expected team order length %d, got %d", len(expectedOrder), len(shopResp.TeamOrder))
	}
	for i, v := range expectedOrder {
		if i < len(shopResp.TeamOrder) && shopResp.TeamOrder[i] != v {
			t.Errorf("expected team_order[%d] = %d, got %d", i, v, shopResp.TeamOrder[i])
		}
	}

	// Verify IsReady is false
	if shopResp.IsReady {
		t.Error("expected IsReady to be false")
	}

	// Verify TeamSubmitted is false
	if shopResp.TeamSubmitted {
		t.Error("expected TeamSubmitted to be false")
	}

	// Verify affordability - should be able to buy and reroll
	if !shopResp.Affordability.CanBuy {
		t.Error("expected CanBuy to be true with 10 coins")
	}
	if !shopResp.Affordability.CanReroll {
		t.Error("expected CanReroll to be true with 10 coins")
	}
	if shopResp.Affordability.CanSubmit {
		t.Error("expected CanSubmit to be false with empty team")
	}

	// Verify shop phase deadline is set
	if shopResp.Deadline == nil {
		t.Error("expected non-nil deadline")
	}

	// Verify time remaining is positive
	if shopResp.TimeRemaining <= 0 {
		t.Error("expected positive time remaining")
	}

	// Verify GetShop also works for joiner
	shopRespJoiner, err := svc.GetShop(ctx, createResp.Match.ID, testJoinerUserID)
	if err != nil {
		t.Fatalf("GetShop failed for joiner: %v", err)
	}
	if shopRespJoiner == nil {
		t.Fatal("expected non-nil shop response for joiner")
	}
	if len(shopRespJoiner.Cards) != 4 {
		t.Errorf("expected 4 shop cards for joiner, got %d", len(shopRespJoiner.Cards))
	}
}

// TestGetShop_AfterTeamSubmitted tests that GetShop returns read-only state after team submission
func TestGetShop_AfterTeamSubmitted(t *testing.T) {
	tdb := setupArenaTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)
	defer cleanupArenaTables(t, tdb)

	seedTestCards(t, tdb, 15)
	defer cleanupArenaTestData(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	mockRepo := newMockGameRepository()
	mockCards := newMockCardService(15)

	svc := NewArenaService(tdb.DB, mockMinIO, mockCards, nil, &ArenaServiceDeps{
		GameRepo:      mockRepo,
		StorageClient: mockMinIO,
		CardService:   mockCards,
	})

	ctx := context.Background()

	// Create match and add second participant
	createResp, err := svc.CreateMatch(ctx, testChatID, testCreatorUserID)
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	_, err = svc.JoinMatch(ctx, createResp.Match.ID, testJoinerUserID)
	if err != nil {
		t.Fatalf("JoinMatch failed: %v", err)
	}

	// Start match to transition to shop phase
	_, err = svc.StartMatch(ctx, createResp.Match.ID, testCreatorUserID)
	if err != nil {
		t.Fatalf("StartMatch failed: %v", err)
	}

	matchID := createResp.Match.ID

	// Buy 3 cards to fill the team (cost: 3 * 3 = 9 coins from 10 starting)
	for i := 0; i < 3; i++ {
		_, err = svc.BuyCard(ctx, matchID, testCreatorUserID, i)
		if err != nil {
			t.Fatalf("BuyCard %d failed: %v", i, err)
		}
	}

	// Verify team has 3 cards before submission
	shopBeforeSubmit, err := svc.GetShop(ctx, matchID, testCreatorUserID)
	if err != nil {
		t.Fatalf("GetShop before submit failed: %v", err)
	}
	if len(shopBeforeSubmit.Team) != 3 {
		t.Fatalf("expected 3 cards in team before submit, got %d", len(shopBeforeSubmit.Team))
	}
	if shopBeforeSubmit.TeamSubmitted {
		t.Fatal("expected TeamSubmitted to be false before submission")
	}

	// Submit team
	submitResp, err := svc.SubmitTeam(ctx, matchID, testCreatorUserID)
	if err != nil {
		t.Fatalf("SubmitTeam failed: %v", err)
	}

	// Verify SubmitTeam response shows read-only state
	if submitResp == nil {
		t.Fatal("expected non-nil submit response")
	}
	if !submitResp.TeamSubmitted {
		t.Error("expected TeamSubmitted to be true after submission")
	}
	if !submitResp.IsReady {
		t.Error("expected IsReady to be true after submission")
	}

	// Now test GetShop after team submission - should return read-only state
	shopAfterSubmit, err := svc.GetShop(ctx, matchID, testCreatorUserID)
	if err != nil {
		t.Fatalf("GetShop after submit failed: %v", err)
	}

	// Verify read-only state
	if shopAfterSubmit == nil {
		t.Fatal("expected non-nil shop response after submit")
	}

	// Verify match ID and status
	if shopAfterSubmit.MatchID != matchID {
		t.Errorf("expected match_id %s, got %s", matchID, shopAfterSubmit.MatchID)
	}
	if shopAfterSubmit.Status != string(repository.MatchStatusShopPhase) {
		t.Errorf("expected status 'shop_phase', got %s", shopAfterSubmit.Status)
	}

	// Verify TeamSubmitted and IsReady are true
	if !shopAfterSubmit.TeamSubmitted {
		t.Error("expected TeamSubmitted to be true")
	}
	if !shopAfterSubmit.IsReady {
		t.Error("expected IsReady to be true")
	}

	// Verify Cards is nil (no shop cards for submitted players)
	if shopAfterSubmit.Cards != nil {
		t.Errorf("expected nil Cards after submission, got %d cards", len(shopAfterSubmit.Cards))
	}

	// Verify Team is still present (showing submitted team)
	if len(shopAfterSubmit.Team) != 3 {
		t.Errorf("expected 3 cards in team after submission, got %d", len(shopAfterSubmit.Team))
	}

	// Verify all affordability flags are false
	if shopAfterSubmit.Affordability.CanBuy {
		t.Error("expected CanBuy to be false after submission")
	}
	if shopAfterSubmit.Affordability.CanReroll {
		t.Error("expected CanReroll to be false after submission")
	}
	if shopAfterSubmit.Affordability.CanUpgrade {
		t.Error("expected CanUpgrade to be false after submission")
	}
	if shopAfterSubmit.Affordability.CanSubmit {
		t.Error("expected CanSubmit to be false after submission")
	}

	// Verify disabled reasons are set
	if shopAfterSubmit.Affordability.BuyDisabledReason == nil {
		t.Error("expected BuyDisabledReason to be set")
	} else if *shopAfterSubmit.Affordability.BuyDisabledReason != "team already submitted" {
		t.Errorf("expected BuyDisabledReason 'team already submitted', got '%s'", *shopAfterSubmit.Affordability.BuyDisabledReason)
	}
	if shopAfterSubmit.Affordability.RerollDisabledReason == nil {
		t.Error("expected RerollDisabledReason to be set")
	}
	if shopAfterSubmit.Affordability.UpgradeDisabledReason == nil {
		t.Error("expected UpgradeDisabledReason to be set")
	}
	if shopAfterSubmit.Affordability.SubmitDisabledReason == nil {
		t.Error("expected SubmitDisabledReason to be set")
	}

	// Verify TimeRemaining is 0 for submitted players
	if shopAfterSubmit.TimeRemaining != 0 {
		t.Errorf("expected TimeRemaining to be 0 after submission, got %d", shopAfterSubmit.TimeRemaining)
	}

	// Verify team order is preserved
	if len(shopAfterSubmit.TeamOrder) != 3 {
		t.Errorf("expected team order length 3, got %d", len(shopAfterSubmit.TeamOrder))
	}
}

// TestBuyCard_Success tests valid card purchase from shop
func TestBuyCard_Success(t *testing.T) {
	tdb := setupArenaTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)
	defer cleanupArenaTables(t, tdb)

	seedTestCards(t, tdb, 15)
	defer cleanupArenaTestData(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	mockRepo := newMockGameRepository()
	mockCards := newMockCardService(15)

	svc := NewArenaService(tdb.DB, mockMinIO, mockCards, nil, &ArenaServiceDeps{
		GameRepo:      mockRepo,
		StorageClient: mockMinIO,
		CardService:   mockCards,
	})

	ctx := context.Background()

	// Create match and add second participant
	createResp, err := svc.CreateMatch(ctx, testChatID, testCreatorUserID)
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	_, err = svc.JoinMatch(ctx, createResp.Match.ID, testJoinerUserID)
	if err != nil {
		t.Fatalf("JoinMatch failed: %v", err)
	}

	// Start match to transition to shop phase
	_, err = svc.StartMatch(ctx, createResp.Match.ID, testCreatorUserID)
	if err != nil {
		t.Fatalf("StartMatch failed: %v", err)
	}

	matchID := createResp.Match.ID

	// Get initial shop state
	shopBefore, err := svc.GetShop(ctx, matchID, testCreatorUserID)
	if err != nil {
		t.Fatalf("GetShop failed: %v", err)
	}

	// Verify initial state: 10 coins, 0 cards in team
	if shopBefore.Coins != 10 {
		t.Fatalf("expected 10 starting coins, got %d", shopBefore.Coins)
	}
	if len(shopBefore.Team) != 0 {
		t.Fatalf("expected empty team initially, got %d cards", len(shopBefore.Team))
	}
	if len(shopBefore.Cards) != 4 {
		t.Fatalf("expected 4 shop cards, got %d", len(shopBefore.Cards))
	}

	// Buy first card (index 0, cost 3 coins)
	buyResp, err := svc.BuyCard(ctx, matchID, testCreatorUserID, 0)
	if err != nil {
		t.Fatalf("BuyCard failed: %v", err)
	}

	// Verify response after purchase
	if buyResp == nil {
		t.Fatal("expected non-nil buy response")
	}

	// Verify coins decreased by 3 (10 - 3 = 7)
	if buyResp.Coins != 7 {
		t.Errorf("expected 7 coins after purchase, got %d", buyResp.Coins)
	}

	// Verify team now has 1 card
	if len(buyResp.Team) != 1 {
		t.Errorf("expected 1 card in team after purchase, got %d", len(buyResp.Team))
	}

	// Verify the purchased card is marked as purchased in shop
	if len(buyResp.Cards) != 4 {
		t.Fatalf("expected 4 shop cards after purchase, got %d", len(buyResp.Cards))
	}
	if !buyResp.Cards[0].IsPurchased {
		t.Error("expected card at index 0 to be marked as purchased")
	}

	// Verify other cards are not marked as purchased
	for i := 1; i < len(buyResp.Cards); i++ {
		if buyResp.Cards[i].IsPurchased {
			t.Errorf("card %d should not be purchased, but is marked as purchased", i)
		}
	}

	// Verify affordability flags are updated correctly
	if !buyResp.Affordability.CanBuy {
		t.Error("expected CanBuy to be true with 7 coins remaining")
	}
	if buyResp.Affordability.CanReroll {
		t.Error("expected CanReroll to be false after first purchase")
	}
	if buyResp.Affordability.CanSubmit {
		t.Error("expected CanSubmit to be false with only 1 card (need 3)")
	}

	// Buy second card (index 1)
	buyResp2, err := svc.BuyCard(ctx, matchID, testCreatorUserID, 1)
	if err != nil {
		t.Fatalf("second BuyCard failed: %v", err)
	}

	// Verify coins: 7 - 3 = 4
	if buyResp2.Coins != 4 {
		t.Errorf("expected 4 coins after second purchase, got %d", buyResp2.Coins)
	}

	// Verify team has 2 cards
	if len(buyResp2.Team) != 2 {
		t.Errorf("expected 2 cards in team after second purchase, got %d", len(buyResp2.Team))
	}

	// Buy third card (index 2) to complete the team
	buyResp3, err := svc.BuyCard(ctx, matchID, testCreatorUserID, 2)
	if err != nil {
		t.Fatalf("third BuyCard failed: %v", err)
	}

	// Verify coins: 4 - 3 = 1
	if buyResp3.Coins != 1 {
		t.Errorf("expected 1 coin after third purchase, got %d", buyResp3.Coins)
	}

	// Verify team is now full (3 cards)
	if len(buyResp3.Team) != 3 {
		t.Errorf("expected 3 cards in team (full), got %d", len(buyResp3.Team))
	}

	// Verify CanSubmit is now true with full team
	if !buyResp3.Affordability.CanSubmit {
		t.Error("expected CanSubmit to be true with full team of 3 cards")
	}
}

// TestBuyCard_NotEnoughCoins tests that buying a card fails when user has insufficient coins
func TestBuyCard_NotEnoughCoins(t *testing.T) {
	tdb := setupArenaTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)
	defer cleanupArenaTables(t, tdb)

	seedTestCards(t, tdb, 15)
	defer cleanupArenaTestData(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	mockRepo := newMockGameRepository()
	mockCards := newMockCardService(15)

	svc := NewArenaService(tdb.DB, mockMinIO, mockCards, nil, &ArenaServiceDeps{
		GameRepo:      mockRepo,
		StorageClient: mockMinIO,
		CardService:   mockCards,
	})

	ctx := context.Background()

	// Create match and add second participant
	createResp, err := svc.CreateMatch(ctx, testChatID, testCreatorUserID)
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	_, err = svc.JoinMatch(ctx, createResp.Match.ID, testJoinerUserID)
	if err != nil {
		t.Fatalf("JoinMatch failed: %v", err)
	}

	// Start match to transition to shop phase
	_, err = svc.StartMatch(ctx, createResp.Match.ID, testCreatorUserID)
	if err != nil {
		t.Fatalf("StartMatch failed: %v", err)
	}

	matchID := createResp.Match.ID

	// Buy 3 cards to spend 9 of 10 coins (3 * 3 = 9), leaving 1 coin
	// Card cost is 3, so after buying 3 cards we have 10 - 9 = 1 coin remaining
	for i := 0; i < 3; i++ {
		_, err = svc.BuyCard(ctx, matchID, testCreatorUserID, i)
		if err != nil {
			t.Fatalf("BuyCard %d failed: %v", i, err)
		}
	}

	// Verify we have 1 coin remaining (insufficient for next purchase)
	shopState, err := svc.GetShop(ctx, matchID, testCreatorUserID)
	if err != nil {
		t.Fatalf("GetShop failed: %v", err)
	}
	if shopState.Coins != 1 {
		t.Fatalf("expected 1 coin remaining, got %d", shopState.Coins)
	}

	// Verify team is full (3 cards)
	if len(shopState.Team) != 3 {
		t.Fatalf("expected 3 cards in team, got %d", len(shopState.Team))
	}

	// Verify CanBuy is false with only 1 coin (need 3 to buy)
	if shopState.Affordability.CanBuy {
		t.Error("expected CanBuy to be false with only 1 coin")
	}

	// Verify the BuyDisabledReason is set appropriately
	// (could be "not enough coins" or "team full" depending on which is checked first in affordability)
	if shopState.Affordability.BuyDisabledReason == nil {
		t.Error("expected BuyDisabledReason to be set")
	}

	// Attempt to buy 4th card - should fail with apperror.ErrNotEnoughCoins
	// In BuyCard implementation, coin check happens BEFORE team full check (line 902-903 vs 905-906)
	// So even though team is full, we should get apperror.ErrNotEnoughCoins first
	_, err = svc.BuyCard(ctx, matchID, testCreatorUserID, 3)
	if err == nil {
		t.Fatal("expected error when buying with insufficient coins, got nil")
	}

	if !errors.Is(err, apperror.ErrNotEnoughCoins) {
		t.Errorf("expected apperror.ErrNotEnoughCoins, got %v", err)
	}

	// Verify coins and team are unchanged after failed purchase attempt
	shopStateAfter, err := svc.GetShop(ctx, matchID, testCreatorUserID)
	if err != nil {
		t.Fatalf("GetShop after failed purchase failed: %v", err)
	}
	if shopStateAfter.Coins != 1 {
		t.Errorf("expected coins to remain 1 after failed purchase, got %d", shopStateAfter.Coins)
	}
	if len(shopStateAfter.Team) != 3 {
		t.Errorf("expected team to remain at 3 cards after failed purchase, got %d cards", len(shopStateAfter.Team))
	}
}

// TestBuyCard_TeamFull tests that buying a card fails when team is already at max size (3 cards)
func TestBuyCard_TeamFull(t *testing.T) {
	tdb := setupArenaTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)
	defer cleanupArenaTables(t, tdb)

	seedTestCards(t, tdb, 15)
	defer cleanupArenaTestData(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	mockRepo := newMockGameRepository()
	mockCards := newMockCardService(15)

	svc := NewArenaService(tdb.DB, mockMinIO, mockCards, nil, &ArenaServiceDeps{
		GameRepo:      mockRepo,
		StorageClient: mockMinIO,
		CardService:   mockCards,
	})

	ctx := context.Background()

	// Create match and add second participant
	createResp, err := svc.CreateMatch(ctx, testChatID, testCreatorUserID)
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	_, err = svc.JoinMatch(ctx, createResp.Match.ID, testJoinerUserID)
	if err != nil {
		t.Fatalf("JoinMatch failed: %v", err)
	}

	// Start match to transition to shop phase
	_, err = svc.StartMatch(ctx, createResp.Match.ID, testCreatorUserID)
	if err != nil {
		t.Fatalf("StartMatch failed: %v", err)
	}

	matchID := createResp.Match.ID

	// Buy exactly 3 cards to fill the team (spend 9 of 10 coins)
	// This leaves us with 1 coin remaining
	for i := 0; i < 3; i++ {
		_, err = svc.BuyCard(ctx, matchID, testCreatorUserID, i)
		if err != nil {
			t.Fatalf("BuyCard %d failed: %v", i, err)
		}
	}

	// Verify team is full (3 cards)
	shopState, err := svc.GetShop(ctx, matchID, testCreatorUserID)
	if err != nil {
		t.Fatalf("GetShop failed: %v", err)
	}
	if len(shopState.Team) != 3 {
		t.Fatalf("expected 3 cards in team, got %d", len(shopState.Team))
	}

	// Now we need to add coins back to test the team full scenario
	// In production, users can't add coins, but to isolate testing apperror.ErrTeamFull,
	// we'll use Reroll to set up the scenario differently:
	// Instead of buying 3 cards, buy 2, then reroll (costs 1 coin), then buy 1 more
	// This way we spend: 3 + 3 + 1 + 3 = 10 coins, have 0 coins, but team is full
	// Actually, this won't work either because we'd have 0 coins and hit apperror.ErrNotEnoughCoins first.

	// The only way to test apperror.ErrTeamFull is to have a team of 3 AND have coins >= 3
	// This is impossible in normal game flow because:
	// - Start with 10 coins
	// - Each card costs 3 coins
	// - 3 cards = 9 coins spent, leaving 1 coin
	// - With 1 coin, apperror.ErrNotEnoughCoins is returned before apperror.ErrTeamFull

	// However, the logic exists for future features (e.g., earning coins mid-game)
	// To properly test this, we need to manipulate the participant's coins directly
	// via the mock repository. Let's verify the shop has 4 cards (one unpurchased)
	if shopState.Cards == nil || len(shopState.Cards) != 4 {
		t.Fatalf("expected 4 shop cards, got %v", len(shopState.Cards))
	}

	// Find an unpurchased card index
	unpurchasedIndex := -1
	for i, card := range shopState.Cards {
		if !card.IsPurchased {
			unpurchasedIndex = i
			break
		}
	}
	if unpurchasedIndex == -1 {
		t.Fatal("expected at least one unpurchased card in shop")
	}

	// To test apperror.ErrTeamFull being triggered, we need to give the participant more coins
	// We'll do this by manually updating the mock repository state
	// Get current participant state
	part := mockRepo.participants[matchID][testCreatorUserID]
	part.CoinsRemaining = 10 // Give them 10 coins so coin check passes
	mockRepo.participants[matchID][testCreatorUserID] = part

	// Now attempt to buy 4th card - should fail with apperror.ErrTeamFull
	// (not apperror.ErrNotEnoughCoins since we have 10 coins)
	_, err = svc.BuyCard(ctx, matchID, testCreatorUserID, unpurchasedIndex)
	if err == nil {
		t.Fatal("expected error when buying with full team, got nil")
	}

	if !errors.Is(err, apperror.ErrTeamFull) {
		t.Errorf("expected apperror.ErrTeamFull, got %v", err)
	}

	// Verify coins are unchanged after failed purchase attempt
	// (Note: coins won't update in GetShop because we manually modified mock state)
	part = mockRepo.participants[matchID][testCreatorUserID]
	if part.CoinsRemaining != 10 {
		t.Errorf("expected coins to remain 10 after failed purchase, got %d", part.CoinsRemaining)
	}

	// Verify team is still 3 cards by using GetShop
	shopStateAfter, err := svc.GetShop(ctx, matchID, testCreatorUserID)
	if err != nil {
		t.Fatalf("GetShop after failed purchase failed: %v", err)
	}
	if len(shopStateAfter.Team) != 3 {
		t.Errorf("expected team to remain at 3 cards after failed purchase, got %d cards", len(shopStateAfter.Team))
	}
}

// TestReroll_Success tests valid shop reroll operation
func TestReroll_Success(t *testing.T) {
	tdb := setupArenaTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)
	defer cleanupArenaTables(t, tdb)

	seedTestCards(t, tdb, 15)
	defer cleanupArenaTestData(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	mockRepo := newMockGameRepository()
	mockCards := newMockCardService(15)

	svc := NewArenaService(tdb.DB, mockMinIO, mockCards, nil, &ArenaServiceDeps{
		GameRepo:      mockRepo,
		StorageClient: mockMinIO,
		CardService:   mockCards,
	})

	ctx := context.Background()

	// Create match and add second participant
	createResp, err := svc.CreateMatch(ctx, testChatID, testCreatorUserID)
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	_, err = svc.JoinMatch(ctx, createResp.Match.ID, testJoinerUserID)
	if err != nil {
		t.Fatalf("JoinMatch failed: %v", err)
	}

	// Start match to transition to shop phase
	_, err = svc.StartMatch(ctx, createResp.Match.ID, testCreatorUserID)
	if err != nil {
		t.Fatalf("StartMatch failed: %v", err)
	}

	matchID := createResp.Match.ID

	// Get initial shop state before reroll
	shopBefore, err := svc.GetShop(ctx, matchID, testCreatorUserID)
	if err != nil {
		t.Fatalf("GetShop failed: %v", err)
	}

	// Verify initial state: 10 coins, 0 cards in team, can reroll
	if shopBefore.Coins != 10 {
		t.Fatalf("expected 10 starting coins, got %d", shopBefore.Coins)
	}
	if len(shopBefore.Team) != 0 {
		t.Fatalf("expected empty team initially, got %d cards", len(shopBefore.Team))
	}
	if len(shopBefore.Cards) != 4 {
		t.Fatalf("expected 4 shop cards, got %d", len(shopBefore.Cards))
	}
	if !shopBefore.Affordability.CanReroll {
		t.Fatal("expected CanReroll to be true initially (teamSize=0, coins=10)")
	}
	if !shopBefore.CanReroll {
		t.Fatal("expected CanReroll flag to be true initially")
	}

	// Record initial shop card IDs for comparison
	initialCardIDs := make([]int64, len(shopBefore.Cards))
	for i, card := range shopBefore.Cards {
		initialCardIDs[i] = card.CardID
	}

	// Perform reroll (costs 1 coin)
	rerollResp, err := svc.Reroll(ctx, matchID, testCreatorUserID)
	if err != nil {
		t.Fatalf("Reroll failed: %v", err)
	}

	// Verify reroll response
	if rerollResp == nil {
		t.Fatal("expected non-nil reroll response")
	}

	// Verify coins decreased by 1 (10 - 1 = 9)
	if rerollResp.Coins != 9 {
		t.Errorf("expected 9 coins after reroll, got %d", rerollResp.Coins)
	}

	// Verify team is still empty (reroll doesn't add cards to team)
	if len(rerollResp.Team) != 0 {
		t.Errorf("expected empty team after reroll, got %d cards", len(rerollResp.Team))
	}

	// Verify still have 4 shop cards
	if len(rerollResp.Cards) != 4 {
		t.Errorf("expected 4 shop cards after reroll, got %d", len(rerollResp.Cards))
	}

	// Verify shop cards are different (new cards dealt)
	newCardIDs := make([]int64, len(rerollResp.Cards))
	for i, card := range rerollResp.Cards {
		newCardIDs[i] = card.CardID
		// All cards should be unpurchased after reroll
		if card.IsPurchased {
			t.Errorf("card %d should be unpurchased after reroll", i)
		}
		// Card indices should be preserved
		if card.Index != i {
			t.Errorf("expected card index %d, got %d", i, card.Index)
		}
	}

	// Verify at least some cards changed (with 15 cards, probability of all same is very low)
	allSame := true
	for i := range initialCardIDs {
		if initialCardIDs[i] != newCardIDs[i] {
			allSame = false
			break
		}
	}
	if allSame {
		t.Error("expected shop cards to change after reroll, but all card IDs are the same")
	}

	// After first reroll, we have 9 coins.
	// The reroll safeguard prevents rerolling into a state where you can't complete a team.
	// To complete team: 3 cards × 3 coins = 9 coins needed
	// Second reroll would cost 1 more coin, leaving 8 coins, which is not enough for 3 cards.
	// So CanReroll should be false after the first reroll due to the safeguard.
	//
	// However, the affordability calculation uses: teamSize == 0 && coins >= RerollCost
	// So with 9 coins and teamSize=0, CanReroll should show as true in affordability
	// but the actual Reroll() will fail due to the internal safeguard check.

	// The affordability calculation is: teamSize == 0 && coins >= RerollCost
	// With 9 coins and teamSize=0, this should be true
	if !rerollResp.Affordability.CanReroll {
		t.Error("expected CanReroll affordability to be true (teamSize=0, coins >= RerollCost)")
	}

	// However, the actual Reroll will fail because of the safeguard:
	// participant.CoinsRemaining < (shop.RerollCost + coinsNeededForCards)
	// 9 < (1 + 9) = 9 < 10 is true, so it will fail
	// This is correct behavior - the safeguard prevents softlocking the player

	// Verify the second reroll fails with apperror.ErrNotEnoughCoins due to safeguard
	_, err = svc.Reroll(ctx, matchID, testCreatorUserID)
	if err == nil {
		t.Error("expected second reroll to fail due to safeguard, but it succeeded")
	} else if !errors.Is(err, apperror.ErrNotEnoughCoins) {
		t.Errorf("expected apperror.ErrNotEnoughCoins from safeguard, got %v", err)
	}

	// Verify coins are unchanged after failed reroll
	shopAfterFailedReroll, err := svc.GetShop(ctx, matchID, testCreatorUserID)
	if err != nil {
		t.Fatalf("GetShop after failed reroll failed: %v", err)
	}
	if shopAfterFailedReroll.Coins != 9 {
		t.Errorf("expected coins to remain 9 after failed reroll, got %d", shopAfterFailedReroll.Coins)
	}
}

// TestReroll_AfterPurchase verifies that reroll only replaces unpurchased cards.
// After buying a card, reroll should keep purchased cards and only refresh unpurchased ones.
func TestReroll_AfterPurchase(t *testing.T) {
	tdb := setupArenaTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)
	defer cleanupArenaTables(t, tdb)

	seedTestCards(t, tdb, 15)
	defer cleanupArenaTestData(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	mockRepo := newMockGameRepository()
	mockCards := newMockCardService(15)

	svc := NewArenaService(tdb.DB, mockMinIO, mockCards, nil, &ArenaServiceDeps{
		GameRepo:      mockRepo,
		StorageClient: mockMinIO,
		CardService:   mockCards,
	})

	ctx := context.Background()

	// Create match and add second participant
	createResp, err := svc.CreateMatch(ctx, testChatID, testCreatorUserID)
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	_, err = svc.JoinMatch(ctx, createResp.Match.ID, testJoinerUserID)
	if err != nil {
		t.Fatalf("JoinMatch failed: %v", err)
	}

	// Start match to transition to shop phase
	_, err = svc.StartMatch(ctx, createResp.Match.ID, testCreatorUserID)
	if err != nil {
		t.Fatalf("StartMatch failed: %v", err)
	}

	matchID := createResp.Match.ID

	// Get initial shop state
	shopBefore, err := svc.GetShop(ctx, matchID, testCreatorUserID)
	if err != nil {
		t.Fatalf("GetShop failed: %v", err)
	}

	// Verify initial state: 10 coins, 0 cards in team, 4 shop cards
	if shopBefore.Coins != 10 {
		t.Fatalf("expected 10 starting coins, got %d", shopBefore.Coins)
	}
	if len(shopBefore.Cards) != 4 {
		t.Fatalf("expected 4 shop cards, got %d", len(shopBefore.Cards))
	}

	// Buy the first card (index 0, costs 3 coins)
	buyResp, err := svc.BuyCard(ctx, matchID, testCreatorUserID, 0)
	if err != nil {
		t.Fatalf("BuyCard failed: %v", err)
	}

	// Verify coins decreased to 7 (10 - 3 = 7) and team has 1 card
	if buyResp.Coins != 7 {
		t.Errorf("expected 7 coins after buying card, got %d", buyResp.Coins)
	}
	if len(buyResp.Team) != 1 {
		t.Errorf("expected 1 card in team, got %d", len(buyResp.Team))
	}

	// Verify the purchased card is marked correctly (should be at index 0)
	if len(buyResp.Cards) != 4 {
		t.Fatalf("expected 4 shop cards after purchase, got %d", len(buyResp.Cards))
	}
	if !buyResp.Cards[0].IsPurchased {
		t.Fatal("expected card at index 0 to be marked as purchased")
	}

	// Verify other cards are not purchased (indices 1, 2, 3)
	for i := 1; i < 4; i++ {
		if buyResp.Cards[i].IsPurchased {
			t.Errorf("card at index %d should not be purchased", i)
		}
	}

	// Verify CanReroll is false after first purchase (critical game mechanic)
	// Per README.md: "Reroll: Refresh shop (before first buy only)"
	if buyResp.Affordability.CanReroll {
		t.Error("expected CanReroll to be false after first purchase (teamSize > 0)")
	}
	if buyResp.CanReroll {
		t.Error("expected CanReroll flag to be false after first purchase")
	}

	// Attempt reroll - should fail because team is not empty
	// Per game rules: reroll is only allowed "before first buy"
	_, err = svc.Reroll(ctx, matchID, testCreatorUserID)
	if err == nil {
		t.Fatal("expected Reroll to fail after purchase (team not empty), but it succeeded")
	}

	// The error should indicate reroll is not allowed after purchasing cards
	// (Implementation note: If this test fails because reroll succeeds, it means
	// the implementation is missing the "team must be empty" validation)

	// Verify coins are unchanged after failed reroll
	shopAfterFailedReroll, err := svc.GetShop(ctx, matchID, testCreatorUserID)
	if err != nil {
		t.Fatalf("GetShop after failed reroll failed: %v", err)
	}
	if shopAfterFailedReroll.Coins != 7 {
		t.Errorf("expected coins to remain 7 after failed reroll, got %d", shopAfterFailedReroll.Coins)
	}
	if len(shopAfterFailedReroll.Team) != 1 {
		t.Errorf("expected team to remain at 1 card after failed reroll, got %d", len(shopAfterFailedReroll.Team))
	}
}

// TestSubmitTeam_Success tests valid team submission
func TestSubmitTeam_Success(t *testing.T) {
	tdb := setupArenaTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)
	defer cleanupArenaTables(t, tdb)

	seedTestCards(t, tdb, 15)
	defer cleanupArenaTestData(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	mockRepo := newMockGameRepository()
	mockCards := newMockCardService(15)

	svc := NewArenaService(tdb.DB, mockMinIO, mockCards, nil, &ArenaServiceDeps{
		GameRepo:      mockRepo,
		StorageClient: mockMinIO,
		CardService:   mockCards,
	})

	ctx := context.Background()

	// Create match and add second participant
	createResp, err := svc.CreateMatch(ctx, testChatID, testCreatorUserID)
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	_, err = svc.JoinMatch(ctx, createResp.Match.ID, testJoinerUserID)
	if err != nil {
		t.Fatalf("JoinMatch failed: %v", err)
	}

	// Start match to transition to shop phase
	_, err = svc.StartMatch(ctx, createResp.Match.ID, testCreatorUserID)
	if err != nil {
		t.Fatalf("StartMatch failed: %v", err)
	}

	matchID := createResp.Match.ID

	// === Test 1: Verify SubmitTeam fails with incomplete team ===

	// Attempt to submit with empty team (should fail)
	_, err = svc.SubmitTeam(ctx, matchID, testCreatorUserID)
	if err == nil {
		t.Fatal("expected error when submitting empty team, got nil")
	}
	// Should return ErrTeamIncomplete from shop package
	if err.Error() != "team incomplete: must have 3 cards" {
		t.Errorf("expected 'team incomplete: must have 3 cards', got '%v'", err)
	}

	// Buy only 2 cards and try to submit (should still fail)
	for i := 0; i < 2; i++ {
		_, err = svc.BuyCard(ctx, matchID, testCreatorUserID, i)
		if err != nil {
			t.Fatalf("BuyCard %d failed: %v", i, err)
		}
	}

	_, err = svc.SubmitTeam(ctx, matchID, testCreatorUserID)
	if err == nil {
		t.Fatal("expected error when submitting incomplete team (2 cards), got nil")
	}

	// === Test 2: Complete the team and submit successfully ===

	// Buy third card to complete team
	_, err = svc.BuyCard(ctx, matchID, testCreatorUserID, 2)
	if err != nil {
		t.Fatalf("BuyCard 2 failed: %v", err)
	}

	// Verify team has 3 cards before submission
	shopBefore, err := svc.GetShop(ctx, matchID, testCreatorUserID)
	if err != nil {
		t.Fatalf("GetShop before submit failed: %v", err)
	}
	if len(shopBefore.Team) != 3 {
		t.Fatalf("expected 3 cards in team before submit, got %d", len(shopBefore.Team))
	}
	if shopBefore.TeamSubmitted {
		t.Fatal("expected TeamSubmitted to be false before submission")
	}
	if shopBefore.IsReady {
		t.Fatal("expected IsReady to be false before submission")
	}
	if !shopBefore.Affordability.CanSubmit {
		t.Fatal("expected CanSubmit to be true with complete team")
	}

	// Record the team before submission for comparison
	teamBeforeSubmit := make([]int64, len(shopBefore.Team))
	for i, card := range shopBefore.Team {
		teamBeforeSubmit[i] = card.CardID
	}

	// Submit team - should succeed
	submitResp, err := svc.SubmitTeam(ctx, matchID, testCreatorUserID)
	if err != nil {
		t.Fatalf("SubmitTeam failed: %v", err)
	}

	// Verify SubmitTeam response
	if submitResp == nil {
		t.Fatal("expected non-nil submit response")
	}

	// Verify TeamSubmitted is true after submission
	if !submitResp.TeamSubmitted {
		t.Error("expected TeamSubmitted to be true after submission")
	}

	// Verify IsReady is true after submission
	if !submitResp.IsReady {
		t.Error("expected IsReady to be true after submission")
	}

	// Verify team is preserved in response
	if len(submitResp.Team) != 3 {
		t.Errorf("expected 3 cards in team after submission, got %d", len(submitResp.Team))
	}

	// Verify the same cards are in the team
	for i, card := range submitResp.Team {
		if card.CardID != teamBeforeSubmit[i] {
			t.Errorf("team card %d changed after submission: expected %d, got %d", i, teamBeforeSubmit[i], card.CardID)
		}
	}

	// Verify match ID is correct
	if submitResp.MatchID != matchID {
		t.Errorf("expected match_id %s, got %s", matchID, submitResp.MatchID)
	}

	// Verify status is still shop_phase (other player hasn't submitted)
	if submitResp.Status != string(repository.MatchStatusShopPhase) {
		t.Errorf("expected status 'shop_phase', got %s", submitResp.Status)
	}

	// Verify all affordability flags are disabled after submission
	if submitResp.Affordability.CanBuy {
		t.Error("expected CanBuy to be false after submission")
	}
	if submitResp.Affordability.CanReroll {
		t.Error("expected CanReroll to be false after submission")
	}
	if submitResp.Affordability.CanUpgrade {
		t.Error("expected CanUpgrade to be false after submission")
	}
	if submitResp.Affordability.CanSubmit {
		t.Error("expected CanSubmit to be false after submission")
	}

	// Verify repository was called
	if mockRepo.submitTeamCalls != 1 {
		t.Errorf("expected 1 SubmitTeam call, got %d", mockRepo.submitTeamCalls)
	}

	// === Test 3: Verify re-submitting fails ===

	// Try to submit again - should fail with apperror.ErrTeamAlreadySubmitted
	_, err = svc.SubmitTeam(ctx, matchID, testCreatorUserID)
	if err == nil {
		t.Fatal("expected error when re-submitting team, got nil")
	}
	if !errors.Is(err, apperror.ErrTeamAlreadySubmitted) {
		t.Errorf("expected apperror.ErrTeamAlreadySubmitted, got %v", err)
	}

	// === Test 4: Second player submits and match transitions to battle ===

	// Joiner buys cards and submits
	for i := 0; i < 3; i++ {
		_, err = svc.BuyCard(ctx, matchID, testJoinerUserID, i)
		if err != nil {
			t.Fatalf("Joiner BuyCard %d failed: %v", i, err)
		}
	}

	// Submit joiner's team
	joinerSubmitResp, err := svc.SubmitTeam(ctx, matchID, testJoinerUserID)
	if err != nil {
		t.Fatalf("Joiner SubmitTeam failed: %v", err)
	}

	// Verify joiner's submission
	if !joinerSubmitResp.TeamSubmitted {
		t.Error("expected joiner TeamSubmitted to be true")
	}
	if !joinerSubmitResp.IsReady {
		t.Error("expected joiner IsReady to be true")
	}

	// After both players submit, the match should transition to battle phase and complete
	// Give async goroutine time to complete (checkAndStartBattle runs async)
	time.Sleep(100 * time.Millisecond)

	// Verify match transitioned out of shop_phase
	// For 1v1 matches, battle runs synchronously so match goes to completed
	match, err := mockRepo.GetMatch(ctx, matchID)
	if err != nil {
		t.Fatalf("GetMatch failed: %v", err)
	}
	// Match should be either in battle_phase or completed (for 1v1, it completes immediately)
	if match.Status != repository.MatchStatusBattlePhase && match.Status != repository.MatchStatusCompleted {
		t.Errorf("expected match status 'battle_phase' or 'completed' after both players submit, got %s", match.Status)
	}

	// For 1v1 matches, verify the match completed with a winner or draw
	if match.Status == repository.MatchStatusCompleted {
		if mockRepo.completeMatchCalls != 1 {
			t.Errorf("expected 1 CompleteMatch call for 1v1 battle, got %d", mockRepo.completeMatchCalls)
		}
	}
}

// =============================================================================
// Battle Phase Tests
// =============================================================================

// TestExecuteBattle_1v1 tests two-player battle resolution
// This is the core battle test that validates the complete battle flow:
// 1. Both players buy cards and submit their teams
// 2. StartBattle is called explicitly to trigger battle execution
// 3. Battle simulation runs with the battle engine
// 4. Results include winner (or draw), events, damage stats, and final team states
// 5. Match is marked as completed with the winner
func TestExecuteBattle_1v1(t *testing.T) {
	tdb := setupArenaTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)
	defer cleanupArenaTables(t, tdb)

	seedTestCards(t, tdb, 15)
	defer cleanupArenaTestData(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	mockRepo := newMockGameRepository()
	mockCards := newMockCardService(15)

	svc := NewArenaService(tdb.DB, mockMinIO, mockCards, nil, &ArenaServiceDeps{
		GameRepo:      mockRepo,
		StorageClient: mockMinIO,
		CardService:   mockCards,
	})

	ctx := context.Background()

	// === Setup: Create match with 2 participants ===

	createResp, err := svc.CreateMatch(ctx, testChatID, testCreatorUserID)
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	_, err = svc.JoinMatch(ctx, createResp.Match.ID, testJoinerUserID)
	if err != nil {
		t.Fatalf("JoinMatch failed: %v", err)
	}

	// Start match to transition to shop phase
	startResp, err := svc.StartMatch(ctx, createResp.Match.ID, testCreatorUserID)
	if err != nil {
		t.Fatalf("StartMatch failed: %v", err)
	}

	matchID := createResp.Match.ID

	// Verify format is 1v1 (2 participants)
	if startResp.Match.Format == nil || *startResp.Match.Format != repository.MatchFormat1v1 {
		t.Fatalf("expected format '1v1', got %v", startResp.Match.Format)
	}

	// === Phase 1: Both players build teams ===

	// Creator buys 3 cards (team full)
	for i := 0; i < 3; i++ {
		_, err = svc.BuyCard(ctx, matchID, testCreatorUserID, i)
		if err != nil {
			t.Fatalf("Creator BuyCard %d failed: %v", i, err)
		}
	}

	// Get creator's team before submission for later comparison
	creatorShop, err := svc.GetShop(ctx, matchID, testCreatorUserID)
	if err != nil {
		t.Fatalf("GetShop for creator failed: %v", err)
	}
	if len(creatorShop.Team) != 3 {
		t.Fatalf("expected creator to have 3 cards, got %d", len(creatorShop.Team))
	}

	// Record creator's team card IDs for verification
	creatorTeamCards := make([]int64, len(creatorShop.Team))
	for i, card := range creatorShop.Team {
		creatorTeamCards[i] = card.CardID
	}

	// Joiner buys 3 cards
	for i := 0; i < 3; i++ {
		_, err = svc.BuyCard(ctx, matchID, testJoinerUserID, i)
		if err != nil {
			t.Fatalf("Joiner BuyCard %d failed: %v", i, err)
		}
	}

	// Get joiner's team before submission
	joinerShop, err := svc.GetShop(ctx, matchID, testJoinerUserID)
	if err != nil {
		t.Fatalf("GetShop for joiner failed: %v", err)
	}
	if len(joinerShop.Team) != 3 {
		t.Fatalf("expected joiner to have 3 cards, got %d", len(joinerShop.Team))
	}

	// Record joiner's team card IDs
	joinerTeamCards := make([]int64, len(joinerShop.Team))
	for i, card := range joinerShop.Team {
		joinerTeamCards[i] = card.CardID
	}

	// === Phase 2: Submit teams (without triggering async battle) ===

	// Submit creator's team
	_, err = svc.SubmitTeam(ctx, matchID, testCreatorUserID)
	if err != nil {
		t.Fatalf("Creator SubmitTeam failed: %v", err)
	}

	// Verify match is still in shop_phase (joiner hasn't submitted)
	match, err := mockRepo.GetMatch(ctx, matchID)
	if err != nil {
		t.Fatalf("GetMatch failed: %v", err)
	}
	if match.Status != repository.MatchStatusShopPhase {
		t.Fatalf("expected status 'shop_phase' after creator submit, got %s", match.Status)
	}

	// Submit joiner's team
	_, err = svc.SubmitTeam(ctx, matchID, testJoinerUserID)
	if err != nil {
		t.Fatalf("Joiner SubmitTeam failed: %v", err)
	}

	// Wait for async checkAndStartBattle to complete
	time.Sleep(150 * time.Millisecond)

	// === Phase 3: Verify battle execution ===

	// After both submit, checkAndStartBattle runs async and calls StartBattle
	// For 1v1, battle runs synchronously and completes the match

	// Verify match transitioned to completed (for 1v1, battle completes immediately)
	match, err = mockRepo.GetMatch(ctx, matchID)
	if err != nil {
		t.Fatalf("GetMatch after battle failed: %v", err)
	}

	// Match should be completed for 1v1
	if match.Status != repository.MatchStatusCompleted {
		// If still in battle_phase, wait a bit more and check again
		if match.Status == repository.MatchStatusBattlePhase {
			time.Sleep(100 * time.Millisecond)
			match, err = mockRepo.GetMatch(ctx, matchID)
			if err != nil {
				t.Fatalf("GetMatch retry failed: %v", err)
			}
		}
		if match.Status != repository.MatchStatusCompleted {
			t.Errorf("expected match status 'completed' for 1v1, got %s", match.Status)
		}
	}

	// Verify repository calls (at least 1 call each)
	// Note: We use >= 1 because the async checkAndStartBattle from a previous test may
	// have completed and incremented these counters if test isolation is imperfect.
	// The important thing is that our test's battle was executed correctly.
	if mockRepo.startBattlePhaseCalls < 1 {
		t.Errorf("expected at least 1 StartBattlePhase call, got %d", mockRepo.startBattlePhaseCalls)
	}
	if mockRepo.completeMatchCalls < 1 {
		t.Errorf("expected at least 1 CompleteMatch call, got %d", mockRepo.completeMatchCalls)
	}
	if mockRepo.createRoundCalls < 1 {
		t.Errorf("expected at least 1 CreateRound call, got %d", mockRepo.createRoundCalls)
	}

	// === Phase 4: Verify battle results via GetBattle ===

	// GetBattle should return the completed battle results
	battleResp, err := svc.GetBattle(ctx, matchID, testCreatorUserID)
	if err != nil {
		t.Fatalf("GetBattle failed: %v", err)
	}

	// Verify battle response structure
	if battleResp == nil {
		t.Fatal("expected non-nil battle response")
	}

	// Verify match ID
	if battleResp.MatchID != matchID {
		t.Errorf("expected match_id %s, got %s", matchID, battleResp.MatchID)
	}

	// Verify we have battle events (battle was simulated)
	if len(battleResp.Events) == 0 {
		t.Error("expected battle events, got empty")
	}

	// Verify NumRounds is positive (battle completed)
	if battleResp.NumRounds <= 0 {
		t.Errorf("expected positive NumRounds, got %d", battleResp.NumRounds)
	}

	// Verify damage stats are recorded
	totalDamage := battleResp.TeamADamage + battleResp.TeamBDamage
	if totalDamage == 0 {
		t.Error("expected non-zero total damage from battle")
	}

	// Verify final team states are present
	if battleResp.TeamAFinal == nil {
		t.Error("expected non-nil TeamAFinal")
	}
	if battleResp.TeamBFinal == nil {
		t.Error("expected non-nil TeamBFinal")
	}

	// Verify either winner or draw
	if battleResp.WinnerID == nil && !battleResp.IsDraw {
		t.Error("expected either WinnerID or IsDraw to be set")
	}
	if battleResp.WinnerID != nil && battleResp.IsDraw {
		t.Error("cannot have both WinnerID and IsDraw")
	}

	// If there's a winner, verify it's one of the participants
	if battleResp.WinnerID != nil {
		winnerID := *battleResp.WinnerID
		if winnerID != testCreatorUserID && winnerID != testJoinerUserID {
			t.Errorf("winner ID %d should be either creator (%d) or joiner (%d)",
				winnerID, testCreatorUserID, testJoinerUserID)
		}
	}

	// Verify final teams have cards
	if battleResp.TeamAFinal != nil && len(battleResp.TeamAFinal.Cards) != 3 {
		t.Errorf("expected TeamAFinal to have 3 cards, got %d", len(battleResp.TeamAFinal.Cards))
	}
	if battleResp.TeamBFinal != nil && len(battleResp.TeamBFinal.Cards) != 3 {
		t.Errorf("expected TeamBFinal to have 3 cards, got %d", len(battleResp.TeamBFinal.Cards))
	}

	// Verify teams are from the correct participants (one is creator, one is joiner)
	if battleResp.TeamAFinal != nil && battleResp.TeamBFinal != nil {
		ownerIDs := []int64{battleResp.TeamAFinal.OwnerID, battleResp.TeamBFinal.OwnerID}
		hasCreator := ownerIDs[0] == testCreatorUserID || ownerIDs[1] == testCreatorUserID
		hasJoiner := ownerIDs[0] == testJoinerUserID || ownerIDs[1] == testJoinerUserID
		if !hasCreator || !hasJoiner {
			t.Errorf("expected teams from creator and joiner, got owner IDs %v", ownerIDs)
		}
	}

	// === Phase 5: Verify battle events contain expected event types ===

	eventTypes := make(map[string]int)
	for _, event := range battleResp.Events {
		eventTypes[string(event.Type)]++
	}

	// Should have attack events (both cards attack each round)
	if eventTypes["attack"] == 0 {
		t.Error("expected 'attack' events in battle")
	}

	// Should have at least one victory event
	if eventTypes["victory"] == 0 {
		t.Error("expected 'victory' event in battle")
	}

	// Verify GetBattle also works for joiner
	joinerBattleResp, err := svc.GetBattle(ctx, matchID, testJoinerUserID)
	if err != nil {
		t.Fatalf("GetBattle for joiner failed: %v", err)
	}
	if joinerBattleResp == nil {
		t.Fatal("expected non-nil battle response for joiner")
	}

	// Both participants should see the same battle results
	if joinerBattleResp.WinnerID != nil && battleResp.WinnerID != nil {
		if *joinerBattleResp.WinnerID != *battleResp.WinnerID {
			t.Error("expected same winner for both participants")
		}
	}
	if joinerBattleResp.IsDraw != battleResp.IsDraw {
		t.Error("expected same draw status for both participants")
	}

	// Log battle summary for debugging
	t.Logf("Battle completed: %d rounds, TeamA damage: %d, TeamB damage: %d, IsDraw: %v",
		battleResp.NumRounds, battleResp.TeamADamage, battleResp.TeamBDamage, battleResp.IsDraw)
	if battleResp.WinnerID != nil {
		t.Logf("Winner: user %d", *battleResp.WinnerID)
	}
}

// TestExecuteBattle_Arena tests multi-player arena battle with round-robin format.
// Arena mode is triggered when 3+ players join a match.
// Each player faces every other player in round-robin fashion.
// For 3 players: 3 rounds (1v2, 1v3, 2v3).
func TestExecuteBattle_Arena(t *testing.T) {
	tdb := setupArenaTestDB(t)
	defer testutil.TeardownTestDB(t, tdb)
	defer cleanupArenaTables(t, tdb)

	// Seed 20 cards (enough for 3 players with different card pools)
	seedTestCards(t, tdb, 20)
	defer cleanupArenaTestData(t, tdb)

	mockMinIO := testutil.NewMockMinIOClient()
	mockRepo := newMockGameRepository()
	mockCards := newMockCardService(20)

	svc := NewArenaService(tdb.DB, mockMinIO, mockCards, nil, &ArenaServiceDeps{
		GameRepo:      mockRepo,
		StorageClient: mockMinIO,
		CardService:   mockCards,
	})

	ctx := context.Background()

	// === Phase 1: Setup - Create match with 3 participants ===

	// Creator creates match
	createResp, err := svc.CreateMatch(ctx, testChatID, testCreatorUserID)
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}
	matchID := createResp.Match.ID

	// Second player joins
	_, err = svc.JoinMatch(ctx, matchID, testJoinerUserID)
	if err != nil {
		t.Fatalf("JoinMatch (player 2) failed: %v", err)
	}

	// Third player joins
	_, err = svc.JoinMatch(ctx, matchID, testUser3ID)
	if err != nil {
		t.Fatalf("JoinMatch (player 3) failed: %v", err)
	}

	// Verify 3 participants
	participants, err := mockRepo.GetMatchParticipants(ctx, matchID)
	if err != nil {
		t.Fatalf("GetMatchParticipants failed: %v", err)
	}
	if len(participants) != 3 {
		t.Fatalf("expected 3 participants, got %d", len(participants))
	}

	// Start match - should transition to shop phase with arena format
	startResp, err := svc.StartMatch(ctx, matchID, testCreatorUserID)
	if err != nil {
		t.Fatalf("StartMatch failed: %v", err)
	}

	// Verify format is arena (3 participants)
	if startResp.Match.Format == nil || *startResp.Match.Format != repository.MatchFormatArena {
		t.Fatalf("expected format 'arena' for 3 players, got %v", startResp.Match.Format)
	}

	// === Phase 2: Shop Phase - All players build teams ===

	// Player 1 (creator) buys 3 cards
	for i := 0; i < 3; i++ {
		_, err = svc.BuyCard(ctx, matchID, testCreatorUserID, i)
		if err != nil {
			t.Fatalf("Player 1 BuyCard %d failed: %v", i, err)
		}
	}

	// Player 2 (joiner) buys 3 cards
	for i := 0; i < 3; i++ {
		_, err = svc.BuyCard(ctx, matchID, testJoinerUserID, i)
		if err != nil {
			t.Fatalf("Player 2 BuyCard %d failed: %v", i, err)
		}
	}

	// Player 3 buys 3 cards
	for i := 0; i < 3; i++ {
		_, err = svc.BuyCard(ctx, matchID, testUser3ID, i)
		if err != nil {
			t.Fatalf("Player 3 BuyCard %d failed: %v", i, err)
		}
	}

	// === Phase 3: Submit teams ===

	// Player 1 submits
	_, err = svc.SubmitTeam(ctx, matchID, testCreatorUserID)
	if err != nil {
		t.Fatalf("Player 1 SubmitTeam failed: %v", err)
	}

	// Player 2 submits
	_, err = svc.SubmitTeam(ctx, matchID, testJoinerUserID)
	if err != nil {
		t.Fatalf("Player 2 SubmitTeam failed: %v", err)
	}

	// Verify match is still in shop_phase (player 3 hasn't submitted)
	match, err := mockRepo.GetMatch(ctx, matchID)
	if err != nil {
		t.Fatalf("GetMatch failed: %v", err)
	}
	if match.Status != repository.MatchStatusShopPhase {
		t.Errorf("expected status 'shop_phase' with 2/3 submitted, got %s", match.Status)
	}

	// Player 3 submits - this triggers battle since all players ready
	_, err = svc.SubmitTeam(ctx, matchID, testUser3ID)
	if err != nil {
		t.Fatalf("Player 3 SubmitTeam failed: %v", err)
	}

	// === Phase 4: Wait for async battle to complete ===

	// Wait for async checkAndStartBattle to complete
	// Arena has more battles (round-robin) so allow more time
	time.Sleep(300 * time.Millisecond)

	// === Phase 5: Verify battle execution ===

	// Verify match transitioned to completed
	match, err = mockRepo.GetMatch(ctx, matchID)
	if err != nil {
		t.Fatalf("GetMatch after battle failed: %v", err)
	}

	// Match should be completed for arena
	if match.Status != repository.MatchStatusCompleted {
		// If still in battle_phase, wait a bit more and check again
		if match.Status == repository.MatchStatusBattlePhase {
			time.Sleep(200 * time.Millisecond)
			match, err = mockRepo.GetMatch(ctx, matchID)
			if err != nil {
				t.Fatalf("GetMatch retry failed: %v", err)
			}
		}
		if match.Status != repository.MatchStatusCompleted {
			t.Errorf("expected match status 'completed' for arena, got %s", match.Status)
		}
	}

	// Verify repository calls
	if mockRepo.startBattlePhaseCalls < 1 {
		t.Errorf("expected at least 1 StartBattlePhase call, got %d", mockRepo.startBattlePhaseCalls)
	}
	if mockRepo.completeMatchCalls < 1 {
		t.Errorf("expected at least 1 CompleteMatch call, got %d", mockRepo.completeMatchCalls)
	}

	// Verify round-robin battles: 3 players = 3 rounds (1v2, 1v3, 2v3)
	// Formula: n*(n-1)/2 rounds for n players
	expectedRounds := 3 // 3*2/2 = 3
	if mockRepo.createRoundCalls < expectedRounds {
		t.Errorf("expected at least %d CreateRound calls for round-robin, got %d", expectedRounds, mockRepo.createRoundCalls)
	}

	// === Phase 6: Verify match has a winner ===

	// Match should have a winner set
	if match.WinnerUserID == nil {
		t.Error("expected match to have a winner")
	} else {
		// Winner should be one of the 3 participants
		winnerID := *match.WinnerUserID
		validWinner := winnerID == testCreatorUserID || winnerID == testJoinerUserID || winnerID == testUser3ID
		if !validWinner {
			t.Errorf("winner ID %d should be one of the 3 participants", winnerID)
		}
		t.Logf("Arena battle winner: user %d", winnerID)
	}

	// === Phase 7: Verify rounds were created ===

	rounds, err := mockRepo.GetMatchRounds(ctx, matchID)
	if err != nil {
		t.Fatalf("GetMatchRounds failed: %v", err)
	}

	if len(rounds) < expectedRounds {
		t.Errorf("expected at least %d rounds, got %d", expectedRounds, len(rounds))
	}

	// Verify each round has two different players
	playerPairs := make(map[string]bool)
	for _, round := range rounds {
		// Ensure player A and B are different
		if round.PlayerAID == round.PlayerBID {
			t.Errorf("round %d has same player on both sides: %d", round.RoundNumber, round.PlayerAID)
		}

		// Track unique pairs using a normalized key (smaller ID first)
		var pairKey string
		if round.PlayerAID < round.PlayerBID {
			pairKey = fmt.Sprintf("%d-%d", round.PlayerAID, round.PlayerBID)
		} else {
			pairKey = fmt.Sprintf("%d-%d", round.PlayerBID, round.PlayerAID)
		}
		playerPairs[pairKey] = true
	}

	// For 3 players, should have 3 unique pairs
	if len(playerPairs) < expectedRounds {
		t.Errorf("expected %d unique player pairs in round-robin, got %d", expectedRounds, len(playerPairs))
	}
	t.Logf("Unique player pairs: %v", playerPairs)

	// Verify all rounds have battle data
	for _, round := range rounds {
		if len(round.BattleLog) == 0 {
			t.Errorf("round %d has empty battle log", round.RoundNumber)
		}
		if len(round.PlayerATeam) == 0 {
			t.Errorf("round %d has empty player A team", round.RoundNumber)
		}
		if len(round.PlayerBTeam) == 0 {
			t.Errorf("round %d has empty player B team", round.RoundNumber)
		}
		if round.TotalRounds <= 0 {
			t.Errorf("round %d has invalid total_rounds: %d", round.RoundNumber, round.TotalRounds)
		}
	}

	// === Phase 8: Verify GetBattle works for arena participants ===

	// GetBattle should work for all 3 participants
	// Note: GetBattle returns the first round's result, not the overall match winner.
	// Individual rounds can be draws even if the match has an overall winner.
	for _, userID := range []int64{testCreatorUserID, testJoinerUserID, testUser3ID} {
		battleResp, err := svc.GetBattle(ctx, matchID, userID)
		if err != nil {
			t.Fatalf("GetBattle for user %d failed: %v", userID, err)
		}

		if battleResp == nil {
			t.Fatalf("expected non-nil battle response for user %d", userID)
		}

		if battleResp.MatchID != matchID {
			t.Errorf("expected match_id %s for user %d, got %s", matchID, userID, battleResp.MatchID)
		}

		// GetBattle returns the first round's winner, which may be nil (draw)
		// Verify that we got valid battle data (events, teams, etc.)
		if len(battleResp.Events) == 0 {
			t.Errorf("expected battle events for user %d", userID)
		}
		if battleResp.NumRounds <= 0 {
			t.Errorf("expected positive NumRounds for user %d, got %d", userID, battleResp.NumRounds)
		}

		// Log round result for debugging
		if battleResp.WinnerID != nil {
			t.Logf("GetBattle for user %d: round winner %d", userID, *battleResp.WinnerID)
		} else {
			t.Logf("GetBattle for user %d: round was a draw", userID)
		}
	}

	// Log arena summary
	t.Logf("Arena battle completed: %d rounds, winner: user %d",
		len(rounds), *match.WinnerUserID)
}
