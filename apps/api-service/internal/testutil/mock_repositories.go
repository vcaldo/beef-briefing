// Package testutil provides testing utilities for the api-service.
// This file contains mock repository implementations for testing services.
package testutil

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"beef-briefing/apps/api-service/internal/models"
	"beef-briefing/apps/api-service/internal/repository"
)

// MockGameRepository is a mock implementation of GameRepositoryInterface for testing.
// It is thread-safe and can be used in concurrent test scenarios.
type MockGameRepository struct {
	mu sync.RWMutex // protects all fields below

	// Match storage
	Matches            map[string]*repository.Match
	Participants       map[string]map[int64]*repository.Participant       // matchID -> userID -> participant
	ParticipantsWithUser map[string]map[int64]*repository.ParticipantWithUser // matchID -> userID -> participant with user info
	Rounds             map[string][]*repository.MatchRound                // matchID -> rounds

	// Tournament storage
	Tournaments            map[int64]*repository.RankedTournament           // tournamentID -> tournament
	TournamentParticipants map[int64]map[int64]*repository.TournamentParticipant // tournamentID -> userID -> participant
	TournamentNextID       int64                                            // auto-increment counter for tournament IDs

	// Call tracking
	CreateMatchCalls           int
	GetMatchCalls              int
	GetActiveMatchesCalls      int
	AddParticipantCalls        int
	GetParticipantCalls        int
	RemoveParticipantCalls     int
	StartShopPhaseCalls        int
	StartBattlePhaseCalls      int
	CompleteMatchCalls         int
	SubmitTeamCalls            int
	UpdateParticipantShopCalls int
	CreateRoundCalls           int

	// Tournament call tracking
	GetOrCreateTournamentCalls        int
	GetTournamentByIDCalls            int
	GetTodayTournamentCalls           int
	AddTournamentParticipantCalls     int
	RemoveTournamentParticipantCalls  int
	GetTournamentParticipantsCalls    int
	IsTournamentParticipantCalls      int
	SetTournamentAnnouncedCalls       int
	CloseTournamentRegistrationCalls  int
	SkipTournamentCalls               int
	CompleteTournamentCalls           int

	// Error injection
	CreateMatchError           error
	GetMatchError              error
	GetActiveMatchesError      error
	AddParticipantError        error
	GetParticipantError        error
	RemoveParticipantError     error
	StartShopPhaseError        error
	StartBattlePhaseError      error
	CompleteMatchError         error
	SubmitTeamError            error
	UpdateParticipantShopError error
	CreateRoundError           error

	// Tournament error injection
	GetOrCreateTournamentError       error
	GetTournamentByIDError           error
	GetTodayTournamentError          error
	AddTournamentParticipantError    error
	RemoveTournamentParticipantError error
	GetTournamentParticipantsError   error
	IsTournamentParticipantError     error
	SetTournamentAnnouncedError      error
	CloseTournamentRegistrationError error
	SkipTournamentError              error
	CompleteTournamentError          error
}

// NewMockGameRepository creates a new MockGameRepository with initialized storage.
func NewMockGameRepository() *MockGameRepository {
	return &MockGameRepository{
		Matches:                make(map[string]*repository.Match),
		Participants:           make(map[string]map[int64]*repository.Participant),
		ParticipantsWithUser:   make(map[string]map[int64]*repository.ParticipantWithUser),
		Rounds:                 make(map[string][]*repository.MatchRound),
		Tournaments:            make(map[int64]*repository.RankedTournament),
		TournamentParticipants: make(map[int64]map[int64]*repository.TournamentParticipant),
		TournamentNextID:       1,
	}
}

// Reset clears all storage and resets call counters and errors.
func (m *MockGameRepository) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Matches = make(map[string]*repository.Match)
	m.Participants = make(map[string]map[int64]*repository.Participant)
	m.ParticipantsWithUser = make(map[string]map[int64]*repository.ParticipantWithUser)
	m.Rounds = make(map[string][]*repository.MatchRound)
	m.Tournaments = make(map[int64]*repository.RankedTournament)
	m.TournamentParticipants = make(map[int64]map[int64]*repository.TournamentParticipant)
	m.TournamentNextID = 1

	m.CreateMatchCalls = 0
	m.GetMatchCalls = 0
	m.GetActiveMatchesCalls = 0
	m.AddParticipantCalls = 0
	m.GetParticipantCalls = 0
	m.RemoveParticipantCalls = 0
	m.StartShopPhaseCalls = 0
	m.StartBattlePhaseCalls = 0
	m.CompleteMatchCalls = 0
	m.SubmitTeamCalls = 0
	m.UpdateParticipantShopCalls = 0
	m.CreateRoundCalls = 0

	// Tournament call tracking
	m.GetOrCreateTournamentCalls = 0
	m.GetTournamentByIDCalls = 0
	m.GetTodayTournamentCalls = 0
	m.AddTournamentParticipantCalls = 0
	m.RemoveTournamentParticipantCalls = 0
	m.GetTournamentParticipantsCalls = 0
	m.IsTournamentParticipantCalls = 0
	m.SetTournamentAnnouncedCalls = 0
	m.CloseTournamentRegistrationCalls = 0
	m.SkipTournamentCalls = 0
	m.CompleteTournamentCalls = 0

	m.CreateMatchError = nil
	m.GetMatchError = nil
	m.GetActiveMatchesError = nil
	m.AddParticipantError = nil
	m.GetParticipantError = nil
	m.RemoveParticipantError = nil
	m.StartShopPhaseError = nil
	m.StartBattlePhaseError = nil
	m.CompleteMatchError = nil
	m.SubmitTeamError = nil
	m.UpdateParticipantShopError = nil
	m.CreateRoundError = nil

	// Tournament error injection
	m.GetOrCreateTournamentError = nil
	m.GetTournamentByIDError = nil
	m.GetTodayTournamentError = nil
	m.AddTournamentParticipantError = nil
	m.RemoveTournamentParticipantError = nil
	m.GetTournamentParticipantsError = nil
	m.IsTournamentParticipantError = nil
	m.SetTournamentAnnouncedError = nil
	m.CloseTournamentRegistrationError = nil
	m.SkipTournamentError = nil
	m.CompleteTournamentError = nil
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

// =============================================================================
// Match Operations
// =============================================================================

// CreateMatch creates a new match in the mock repository.
func (m *MockGameRepository) CreateMatch(ctx context.Context, chatID int64, matchType repository.MatchType, creatorUserID *int64, tournamentDate *string) (*repository.Match, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CreateMatchCalls++
	if m.CreateMatchError != nil {
		return nil, m.CreateMatchError
	}

	matchID := "test-match-" + time.Now().Format("20060102150405.000000000")
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

	m.Matches[matchID] = match
	m.Participants[matchID] = make(map[int64]*repository.Participant)
	return copyMatch(match), nil
}

// GetMatch retrieves a match by ID.
// Returns a deep copy to prevent data races when multiple goroutines access the match.
func (m *MockGameRepository) GetMatch(ctx context.Context, matchID string) (*repository.Match, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetMatchCalls++
	if m.GetMatchError != nil {
		return nil, m.GetMatchError
	}
	return copyMatch(m.Matches[matchID]), nil
}

// GetActiveMatches retrieves active matches for a chat.
// Returns deep copies to prevent data races.
func (m *MockGameRepository) GetActiveMatches(ctx context.Context, chatID int64) ([]*repository.Match, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetActiveMatchesCalls++
	if m.GetActiveMatchesError != nil {
		return nil, m.GetActiveMatchesError
	}

	var matches []*repository.Match
	for _, match := range m.Matches {
		if match.ChatID == chatID &&
			(match.Status == repository.MatchStatusOpen ||
				match.Status == repository.MatchStatusShopPhase ||
				match.Status == repository.MatchStatusBattlePhase) {
			matches = append(matches, copyMatch(match))
		}
	}
	return matches, nil
}

// GetMatchesByStatus retrieves matches by status.
// Returns deep copies to prevent data races.
func (m *MockGameRepository) GetMatchesByStatus(ctx context.Context, status repository.MatchStatus) ([]*repository.Match, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matches []*repository.Match
	for _, match := range m.Matches {
		if match.Status == status {
			matches = append(matches, copyMatch(match))
		}
	}
	return matches, nil
}

// UpdateMatchStatus updates a match status.
func (m *MockGameRepository) UpdateMatchStatus(ctx context.Context, matchID string, status repository.MatchStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	match := m.Matches[matchID]
	if match == nil {
		return errors.New("match not found")
	}
	match.Status = status
	return nil
}

// UpdateMatchFormat updates a match format.
func (m *MockGameRepository) UpdateMatchFormat(ctx context.Context, matchID string, format repository.MatchFormat) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	match := m.Matches[matchID]
	if match == nil {
		return errors.New("match not found")
	}
	match.Format = &format
	return nil
}

// StartShopPhase transitions match to shop phase.
func (m *MockGameRepository) StartShopPhase(ctx context.Context, matchID string, format repository.MatchFormat, deadline time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.StartShopPhaseCalls++
	if m.StartShopPhaseError != nil {
		return m.StartShopPhaseError
	}

	match := m.Matches[matchID]
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

// StartBattlePhase transitions match to battle phase.
func (m *MockGameRepository) StartBattlePhase(ctx context.Context, matchID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.StartBattlePhaseCalls++
	if m.StartBattlePhaseError != nil {
		return m.StartBattlePhaseError
	}

	match := m.Matches[matchID]
	if match == nil {
		return errors.New("match not found")
	}

	match.Status = repository.MatchStatusBattlePhase
	now := time.Now()
	match.BattleStartedAt = &now
	return nil
}

// CompleteMatch completes a match with optional winner.
func (m *MockGameRepository) CompleteMatch(ctx context.Context, matchID string, winnerUserID *int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CompleteMatchCalls++
	if m.CompleteMatchError != nil {
		return m.CompleteMatchError
	}

	match := m.Matches[matchID]
	if match == nil {
		return errors.New("match not found")
	}

	match.Status = repository.MatchStatusCompleted
	now := time.Now()
	match.CompletedAt = &now
	match.WinnerUserID = winnerUserID
	return nil
}

// CancelMatch cancels a match.
func (m *MockGameRepository) CancelMatch(ctx context.Context, matchID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	match := m.Matches[matchID]
	if match == nil {
		return errors.New("match not found")
	}

	match.Status = repository.MatchStatusCancelled
	return nil
}

// =============================================================================
// Participant Operations
// =============================================================================

// AddParticipant adds a participant to a match.
func (m *MockGameRepository) AddParticipant(ctx context.Context, matchID string, userID int64) (*repository.Participant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.AddParticipantCalls++
	if m.AddParticipantError != nil {
		return nil, m.AddParticipantError
	}

	if m.Participants[matchID] == nil {
		m.Participants[matchID] = make(map[int64]*repository.Participant)
	}

	// Check if already joined
	if _, exists := m.Participants[matchID][userID]; exists {
		return nil, errors.New("already joined this match")
	}

	participant := &repository.Participant{
		ID:             int64(len(m.Participants[matchID]) + 1),
		MatchID:        matchID,
		UserID:         userID,
		Status:         repository.ParticipantStatusJoined,
		JoinedAt:       time.Now(),
		CoinsRemaining: 10,
	}
	m.Participants[matchID][userID] = participant

	// Also add to ParticipantsWithUser for GetMatchParticipants
	if m.ParticipantsWithUser[matchID] == nil {
		m.ParticipantsWithUser[matchID] = make(map[int64]*repository.ParticipantWithUser)
	}
	pwu := &repository.ParticipantWithUser{
		Participant: *participant,
		FirstName:   "Test User",
		Username:    "testuser",
	}
	m.ParticipantsWithUser[matchID][userID] = pwu

	return copyParticipant(participant), nil
}

// GetParticipant retrieves a participant.
// Returns a deep copy to prevent data races when multiple goroutines access the participant.
func (m *MockGameRepository) GetParticipant(ctx context.Context, matchID string, userID int64) (*repository.Participant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetParticipantCalls++
	if m.GetParticipantError != nil {
		return nil, m.GetParticipantError
	}

	if m.Participants[matchID] == nil {
		return nil, nil
	}
	return copyParticipant(m.Participants[matchID][userID]), nil
}

// GetMatchParticipants retrieves all participants for a match with user info.
// Returns deep copies to prevent data races.
func (m *MockGameRepository) GetMatchParticipants(ctx context.Context, matchID string) ([]*repository.ParticipantWithUser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Prefer ParticipantsWithUser if available, otherwise construct from Participants
	if pwu, ok := m.ParticipantsWithUser[matchID]; ok && len(pwu) > 0 {
		var result []*repository.ParticipantWithUser
		for _, p := range pwu {
			// Copy to prevent races
			pCopy := *p
			result = append(result, &pCopy)
		}
		return result, nil
	}

	if m.Participants[matchID] == nil {
		return []*repository.ParticipantWithUser{}, nil
	}

	var result []*repository.ParticipantWithUser
	for _, p := range m.Participants[matchID] {
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

// RemoveParticipant removes a participant from a match.
func (m *MockGameRepository) RemoveParticipant(ctx context.Context, matchID string, userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.RemoveParticipantCalls++
	if m.RemoveParticipantError != nil {
		return m.RemoveParticipantError
	}

	if m.Participants[matchID] != nil {
		delete(m.Participants[matchID], userID)
	}
	return nil
}

// UpdateParticipantShop updates participant shop state.
func (m *MockGameRepository) UpdateParticipantShop(ctx context.Context, matchID string, userID int64, coins int, shopCards, team json.RawMessage, teamOrder []int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.UpdateParticipantShopCalls++
	if m.UpdateParticipantShopError != nil {
		return m.UpdateParticipantShopError
	}

	if m.Participants[matchID] == nil {
		return errors.New("match not found")
	}
	p := m.Participants[matchID][userID]
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

// SubmitTeam marks participant as ready.
func (m *MockGameRepository) SubmitTeam(ctx context.Context, matchID string, userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SubmitTeamCalls++
	if m.SubmitTeamError != nil {
		return m.SubmitTeamError
	}

	if m.Participants[matchID] == nil {
		return errors.New("match not found")
	}
	p := m.Participants[matchID][userID]
	if p == nil {
		return errors.New("participant not found")
	}

	p.Status = repository.ParticipantStatusReady
	now := time.Now()
	p.TeamSubmittedAt = &now
	return nil
}

// GetParticipantCount returns the number of participants in a match.
func (m *MockGameRepository) GetParticipantCount(ctx context.Context, matchID string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.Participants[matchID] == nil {
		return 0, nil
	}
	return len(m.Participants[matchID]), nil
}

// GetReadyParticipantCount returns the number of ready participants.
func (m *MockGameRepository) GetReadyParticipantCount(ctx context.Context, matchID string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.Participants[matchID] == nil {
		return 0, nil
	}
	count := 0
	for _, p := range m.Participants[matchID] {
		if p.Status == repository.ParticipantStatusReady {
			count++
		}
	}
	return count, nil
}

// =============================================================================
// Round Operations
// =============================================================================

// CreateRound creates a new match round.
func (m *MockGameRepository) CreateRound(ctx context.Context, matchID string, roundNumber int, playerAID, playerBID int64, playerATeam, playerBTeam, battleLog json.RawMessage, winnerID *int64, isDraw bool, playerADmg, playerBDmg, totalRounds int) (*repository.MatchRound, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CreateRoundCalls++
	if m.CreateRoundError != nil {
		return nil, m.CreateRoundError
	}

	round := &repository.MatchRound{
		ID:          int64(len(m.Rounds[matchID]) + 1),
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

	if m.Rounds[matchID] == nil {
		m.Rounds[matchID] = []*repository.MatchRound{}
	}
	m.Rounds[matchID] = append(m.Rounds[matchID], round)

	return round, nil
}

// GetMatchRounds retrieves all rounds for a match.
func (m *MockGameRepository) GetMatchRounds(ctx context.Context, matchID string) ([]*repository.MatchRound, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.Rounds[matchID] == nil {
		return []*repository.MatchRound{}, nil
	}
	return m.Rounds[matchID], nil
}

// =============================================================================
// Tournament Operations (Mock Implementation)
// =============================================================================

// copyTournament creates a deep copy of a tournament to prevent data races.
func copyTournament(t *repository.RankedTournament) *repository.RankedTournament {
	if t == nil {
		return nil
	}
	cp := *t
	// Deep copy pointer fields
	if t.AnnouncementMessageID != nil {
		id := *t.AnnouncementMessageID
		cp.AnnouncementMessageID = &id
	}
	if t.AnnouncedAt != nil {
		at := *t.AnnouncedAt
		cp.AnnouncedAt = &at
	}
	if t.RegistrationClosedAt != nil {
		at := *t.RegistrationClosedAt
		cp.RegistrationClosedAt = &at
	}
	if t.CompletedAt != nil {
		at := *t.CompletedAt
		cp.CompletedAt = &at
	}
	if t.MatchID != nil {
		id := *t.MatchID
		cp.MatchID = &id
	}
	if t.WinnerUserID != nil {
		id := *t.WinnerUserID
		cp.WinnerUserID = &id
	}
	return &cp
}

// AddTournament adds a tournament to the mock storage for test setup.
func (m *MockGameRepository) AddTournament(t *repository.RankedTournament) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Tournaments[t.ID] = copyTournament(t)
	if m.TournamentParticipants[t.ID] == nil {
		m.TournamentParticipants[t.ID] = make(map[int64]*repository.TournamentParticipant)
	}
}

// GetOrCreateTournament gets an existing tournament or creates a new one.
func (m *MockGameRepository) GetOrCreateTournament(ctx context.Context, chatID int64, date string) (*repository.RankedTournament, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetOrCreateTournamentCalls++

	if m.GetOrCreateTournamentError != nil {
		return nil, m.GetOrCreateTournamentError
	}

	// Look for existing tournament
	for _, t := range m.Tournaments {
		if t.ChatID == chatID && t.TournamentDate == date {
			return copyTournament(t), nil
		}
	}

	// Create new tournament
	id := m.TournamentNextID
	m.TournamentNextID++

	tournament := &repository.RankedTournament{
		ID:               id,
		ChatID:           chatID,
		TournamentDate:   date,
		Status:           repository.TournamentStatusScheduled,
		ParticipantCount: 0,
		CreatedAt:        time.Now(),
	}

	m.Tournaments[id] = tournament
	m.TournamentParticipants[id] = make(map[int64]*repository.TournamentParticipant)

	return copyTournament(tournament), nil
}

// GetTournamentByID retrieves a tournament by ID.
func (m *MockGameRepository) GetTournamentByID(ctx context.Context, id int64) (*repository.RankedTournament, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetTournamentByIDCalls++

	if m.GetTournamentByIDError != nil {
		return nil, m.GetTournamentByIDError
	}

	t, exists := m.Tournaments[id]
	if !exists {
		return nil, nil
	}

	return copyTournament(t), nil
}

// GetTodayTournament retrieves today's tournament for a chat.
func (m *MockGameRepository) GetTodayTournament(ctx context.Context, chatID int64, date string) (*repository.RankedTournament, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetTodayTournamentCalls++

	if m.GetTodayTournamentError != nil {
		return nil, m.GetTodayTournamentError
	}

	for _, t := range m.Tournaments {
		if t.ChatID == chatID && t.TournamentDate == date {
			return copyTournament(t), nil
		}
	}

	return nil, nil
}

// GetTournamentsByStatus returns tournaments with a specific status.
func (m *MockGameRepository) GetTournamentsByStatus(ctx context.Context, status repository.TournamentStatus) ([]*repository.RankedTournament, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*repository.RankedTournament
	for _, t := range m.Tournaments {
		if t.Status == status {
			result = append(result, copyTournament(t))
		}
	}
	return result, nil
}

// GetTournamentsWithPendingRounds returns tournaments that need next round execution.
func (m *MockGameRepository) GetTournamentsWithPendingRounds(ctx context.Context) ([]*repository.RankedTournament, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*repository.RankedTournament
	for _, t := range m.Tournaments {
		if t.Status == repository.TournamentStatusInProgress {
			result = append(result, copyTournament(t))
		}
	}
	return result, nil
}

// UpdateTournamentStatus updates the status of a tournament.
func (m *MockGameRepository) UpdateTournamentStatus(ctx context.Context, id int64, status repository.TournamentStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, exists := m.Tournaments[id]
	if !exists {
		return errors.New("tournament not found")
	}

	t.Status = status
	return nil
}

// SetTournamentAnnounced marks a tournament as announced.
func (m *MockGameRepository) SetTournamentAnnounced(ctx context.Context, id int64, messageID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SetTournamentAnnouncedCalls++

	if m.SetTournamentAnnouncedError != nil {
		return m.SetTournamentAnnouncedError
	}

	t, exists := m.Tournaments[id]
	if !exists {
		return errors.New("tournament not found")
	}

	t.Status = repository.TournamentStatusOpen
	t.AnnouncementMessageID = &messageID
	now := time.Now()
	t.AnnouncedAt = &now

	return nil
}

// CloseTournamentRegistration closes registration and links the match.
func (m *MockGameRepository) CloseTournamentRegistration(ctx context.Context, id int64, matchID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CloseTournamentRegistrationCalls++

	if m.CloseTournamentRegistrationError != nil {
		return m.CloseTournamentRegistrationError
	}

	t, exists := m.Tournaments[id]
	if !exists {
		return errors.New("tournament not found")
	}

	t.Status = repository.TournamentStatusInProgress
	t.MatchID = &matchID
	now := time.Now()
	t.RegistrationClosedAt = &now

	return nil
}

// CompleteTournament marks a tournament as completed.
func (m *MockGameRepository) CompleteTournament(ctx context.Context, id int64, winnerUserID *int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CompleteTournamentCalls++

	if m.CompleteTournamentError != nil {
		return m.CompleteTournamentError
	}

	t, exists := m.Tournaments[id]
	if !exists {
		return errors.New("tournament not found")
	}

	t.Status = repository.TournamentStatusCompleted
	t.WinnerUserID = winnerUserID
	now := time.Now()
	t.CompletedAt = &now

	return nil
}

// SkipTournament marks a tournament as skipped.
func (m *MockGameRepository) SkipTournament(ctx context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SkipTournamentCalls++

	if m.SkipTournamentError != nil {
		return m.SkipTournamentError
	}

	t, exists := m.Tournaments[id]
	if !exists {
		return errors.New("tournament not found")
	}

	t.Status = repository.TournamentStatusSkipped

	return nil
}

// UpdateTournamentBracket updates the bracket state.
func (m *MockGameRepository) UpdateTournamentBracket(ctx context.Context, id int64, bracketState json.RawMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, exists := m.Tournaments[id]
	if !exists {
		return errors.New("tournament not found")
	}

	t.BracketState = &bracketState
	return nil
}

// AddTournamentParticipant adds a user to a tournament.
func (m *MockGameRepository) AddTournamentParticipant(ctx context.Context, tournamentID, userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.AddTournamentParticipantCalls++

	if m.AddTournamentParticipantError != nil {
		return m.AddTournamentParticipantError
	}

	if m.TournamentParticipants[tournamentID] == nil {
		m.TournamentParticipants[tournamentID] = make(map[int64]*repository.TournamentParticipant)
	}

	m.TournamentParticipants[tournamentID][userID] = &repository.TournamentParticipant{
		ID:           int64(len(m.TournamentParticipants[tournamentID]) + 1),
		TournamentID: tournamentID,
		UserID:       userID,
		JoinedAt:     time.Now(),
	}

	// Update participant count on tournament
	if t, exists := m.Tournaments[tournamentID]; exists {
		t.ParticipantCount = len(m.TournamentParticipants[tournamentID])
	}

	return nil
}

// RemoveTournamentParticipant removes a user from a tournament.
func (m *MockGameRepository) RemoveTournamentParticipant(ctx context.Context, tournamentID, userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.RemoveTournamentParticipantCalls++

	if m.RemoveTournamentParticipantError != nil {
		return m.RemoveTournamentParticipantError
	}

	if m.TournamentParticipants[tournamentID] != nil {
		delete(m.TournamentParticipants[tournamentID], userID)

		// Update participant count on tournament
		if t, exists := m.Tournaments[tournamentID]; exists {
			t.ParticipantCount = len(m.TournamentParticipants[tournamentID])
		}
	}

	return nil
}

// GetTournamentParticipants retrieves all participants for a tournament.
func (m *MockGameRepository) GetTournamentParticipants(ctx context.Context, tournamentID int64) ([]*repository.TournamentParticipant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetTournamentParticipantsCalls++

	if m.GetTournamentParticipantsError != nil {
		return nil, m.GetTournamentParticipantsError
	}

	var result []*repository.TournamentParticipant
	if participants, exists := m.TournamentParticipants[tournamentID]; exists {
		for _, p := range participants {
			cp := *p
			result = append(result, &cp)
		}
	}

	return result, nil
}

// IsTournamentParticipant checks if a user is registered for a tournament.
func (m *MockGameRepository) IsTournamentParticipant(ctx context.Context, tournamentID, userID int64) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.IsTournamentParticipantCalls++

	if m.IsTournamentParticipantError != nil {
		return false, m.IsTournamentParticipantError
	}

	if participants, exists := m.TournamentParticipants[tournamentID]; exists {
		_, isParticipant := participants[userID]
		return isParticipant, nil
	}

	return false, nil
}

// GetTournamentsNeedingAnnouncement returns tournaments that need to be announced.
func (m *MockGameRepository) GetTournamentsNeedingAnnouncement(ctx context.Context, currentTime time.Time) ([]*repository.TournamentInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*repository.TournamentInfo
	for _, t := range m.Tournaments {
		if t.Status == repository.TournamentStatusScheduled {
			result = append(result, &repository.TournamentInfo{
				TournamentID:     t.ID,
				ChatID:           t.ChatID,
				TournamentDate:   t.TournamentDate,
				ParticipantCount: int64(t.ParticipantCount),
			})
		}
	}
	return result, nil
}

// GetTournamentsNeedingClose returns tournaments that need registration closed.
func (m *MockGameRepository) GetTournamentsNeedingClose(ctx context.Context, currentTime time.Time) ([]*repository.TournamentInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*repository.TournamentInfo
	for _, t := range m.Tournaments {
		if t.Status == repository.TournamentStatusOpen {
			result = append(result, &repository.TournamentInfo{
				TournamentID:     t.ID,
				ChatID:           t.ChatID,
				TournamentDate:   t.TournamentDate,
				ParticipantCount: int64(t.ParticipantCount),
			})
		}
	}
	return result, nil
}

// GetChatsWithTimezone returns all chats with their timezone settings.
func (m *MockGameRepository) GetChatsWithTimezone(ctx context.Context) ([]*repository.ChatTimezone, error) {
	return []*repository.ChatTimezone{}, nil
}

// =============================================================================
// Leaderboard Operations (Stubs)
// =============================================================================

// GetLeaderboard is a stub for interface compliance.
func (m *MockGameRepository) GetLeaderboard(ctx context.Context, chatID int64, matchType repository.MatchType, limit, offset int) ([]*repository.LeaderboardEntry, error) {
	return []*repository.LeaderboardEntry{}, nil
}

// UpdateLeaderboard is a stub for interface compliance.
func (m *MockGameRepository) UpdateLeaderboard(ctx context.Context, userID, chatID int64, matchType repository.MatchType, isWin bool, opponentID *int64, isTournamentWin bool, isDraw bool) error {
	return nil
}

// GetMatchHistory is a stub for interface compliance.
func (m *MockGameRepository) GetMatchHistory(ctx context.Context, chatID, userID int64, limit, offset int) ([]*repository.MatchHistoryEntry, int, error) {
	return []*repository.MatchHistoryEntry{}, 0, nil
}

// GetH2HRecord is a stub for interface compliance.
func (m *MockGameRepository) GetH2HRecord(ctx context.Context, chatID, userID, opponentID int64) (*repository.H2HRecord, error) {
	return nil, nil
}

// GetRecentMatchesVsOpponent is a stub for interface compliance.
func (m *MockGameRepository) GetRecentMatchesVsOpponent(ctx context.Context, chatID, userID, opponentID int64, limit int) ([]*repository.MatchHistoryEntry, error) {
	return []*repository.MatchHistoryEntry{}, nil
}

// GetUserProfile is a stub for interface compliance.
func (m *MockGameRepository) GetUserProfile(ctx context.Context, chatID, userID int64) (*repository.UserProfile, error) {
	return nil, nil
}

// Ensure MockGameRepository implements the interface at compile time
var _ repository.GameRepositoryInterface = (*MockGameRepository)(nil)

// =============================================================================
// MockCardRepository
// =============================================================================

// MockCardRepository is a mock implementation of CardRepositoryInterface for testing.
// It is thread-safe and can be used in concurrent test scenarios.
type MockCardRepository struct {
	mu sync.RWMutex // protects all fields below

	// UserCards storage: chatID -> userID -> weekStart -> card
	UserCards map[int64]map[int64]map[string]*repository.UserCard
	// CardImages storage: chatID -> userID -> weekStart -> theme -> image
	CardImages map[int64]map[int64]map[string]map[string]*repository.CardImage
	// Users storage: userID -> user
	Users map[int64]*repository.CardUser
	// GalleryImages storage: imageID -> image
	GalleryImages map[int64]*repository.GalleryImage

	// Call tracking
	GetUserCardCalls         int
	GetUserInfoCalls         int
	GetChatCardsCalls        int
	GetUserHistoryCalls      int
	GetAvailableWeeksCalls   int
	GetCardImageCalls        int
	GetGalleryWeeksCalls     int
	GetGalleryImagesCalls    int
	GetGalleryImageByIDCalls int

	// Error injection
	GetUserCardError         error
	GetUserInfoError         error
	GetChatCardsError        error
	GetUserHistoryError      error
	GetAvailableWeeksError   error
	GetCardImageError        error
	GetGalleryWeeksError     error
	GetGalleryImagesError    error
	GetGalleryImageByIDError error
}

// NewMockCardRepository creates a new MockCardRepository with initialized storage.
func NewMockCardRepository() *MockCardRepository {
	return &MockCardRepository{
		UserCards:     make(map[int64]map[int64]map[string]*repository.UserCard),
		CardImages:    make(map[int64]map[int64]map[string]map[string]*repository.CardImage),
		Users:         make(map[int64]*repository.CardUser),
		GalleryImages: make(map[int64]*repository.GalleryImage),
	}
}

// Reset clears all storage and resets call counters and errors.
func (m *MockCardRepository) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.UserCards = make(map[int64]map[int64]map[string]*repository.UserCard)
	m.CardImages = make(map[int64]map[int64]map[string]map[string]*repository.CardImage)
	m.Users = make(map[int64]*repository.CardUser)
	m.GalleryImages = make(map[int64]*repository.GalleryImage)

	m.GetUserCardCalls = 0
	m.GetUserInfoCalls = 0
	m.GetChatCardsCalls = 0
	m.GetUserHistoryCalls = 0
	m.GetAvailableWeeksCalls = 0
	m.GetCardImageCalls = 0
	m.GetGalleryWeeksCalls = 0
	m.GetGalleryImagesCalls = 0
	m.GetGalleryImageByIDCalls = 0

	m.GetUserCardError = nil
	m.GetUserInfoError = nil
	m.GetChatCardsError = nil
	m.GetUserHistoryError = nil
	m.GetAvailableWeeksError = nil
	m.GetCardImageError = nil
	m.GetGalleryWeeksError = nil
	m.GetGalleryImagesError = nil
	m.GetGalleryImageByIDError = nil
}

// AddUserCard adds a user card to the mock storage.
func (m *MockCardRepository) AddUserCard(card *repository.UserCard) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.UserCards[card.ChatID] == nil {
		m.UserCards[card.ChatID] = make(map[int64]map[string]*repository.UserCard)
	}
	if m.UserCards[card.ChatID][card.UserID] == nil {
		m.UserCards[card.ChatID][card.UserID] = make(map[string]*repository.UserCard)
	}
	m.UserCards[card.ChatID][card.UserID][card.WeekStart] = card
}

// AddCardImage adds a card image to the mock storage.
func (m *MockCardRepository) AddCardImage(img *repository.CardImage) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.CardImages[img.ChatID] == nil {
		m.CardImages[img.ChatID] = make(map[int64]map[string]map[string]*repository.CardImage)
	}
	if m.CardImages[img.ChatID][img.UserID] == nil {
		m.CardImages[img.ChatID][img.UserID] = make(map[string]map[string]*repository.CardImage)
	}
	if m.CardImages[img.ChatID][img.UserID][img.WeekStart] == nil {
		m.CardImages[img.ChatID][img.UserID][img.WeekStart] = make(map[string]*repository.CardImage)
	}
	m.CardImages[img.ChatID][img.UserID][img.WeekStart][img.Theme] = img
}

// AddUser adds a user to the mock storage.
func (m *MockCardRepository) AddUser(user *repository.CardUser) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Users[user.ID] = user
}

// AddGalleryImage adds a gallery image to the mock storage.
func (m *MockCardRepository) AddGalleryImage(img *repository.GalleryImage) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GalleryImages[img.ID] = img
}

// copyUserCard creates a deep copy of a UserCard.
func copyUserCard(c *repository.UserCard) *repository.UserCard {
	if c == nil {
		return nil
	}
	cp := *c
	if c.Stats != nil {
		stats := make([]byte, len(c.Stats))
		copy(stats, c.Stats)
		cp.Stats = stats
	}
	if c.Trends != nil {
		trends := make([]byte, len(c.Trends))
		copy(trends, c.Trends)
		cp.Trends = trends
	}
	if c.Timezone != nil {
		tz := *c.Timezone
		cp.Timezone = &tz
	}
	return &cp
}

// copyCardImage creates a deep copy of a CardImage.
func copyCardImage(c *repository.CardImage) *repository.CardImage {
	if c == nil {
		return nil
	}
	cp := *c
	return &cp
}

// copyCardUser creates a deep copy of a CardUser.
func copyCardUser(c *repository.CardUser) *repository.CardUser {
	if c == nil {
		return nil
	}
	cp := *c
	return &cp
}

// copyGalleryImage creates a deep copy of a GalleryImage.
func copyGalleryImage(g *repository.GalleryImage) *repository.GalleryImage {
	if g == nil {
		return nil
	}
	cp := *g
	if g.FirstName != nil {
		fn := *g.FirstName
		cp.FirstName = &fn
	}
	if g.LastName != nil {
		ln := *g.LastName
		cp.LastName = &ln
	}
	if g.Username != nil {
		un := *g.Username
		cp.Username = &un
	}
	return &cp
}

// GetUserCard retrieves a single card for a user.
func (m *MockCardRepository) GetUserCard(ctx context.Context, userID int64, chatID int64, weekStart *time.Time) (*repository.UserCard, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetUserCardCalls++
	if m.GetUserCardError != nil {
		return nil, m.GetUserCardError
	}

	if m.UserCards[chatID] == nil || m.UserCards[chatID][userID] == nil {
		return nil, nil
	}

	userCards := m.UserCards[chatID][userID]
	if weekStart != nil {
		weekStr := weekStart.Format("2006-01-02")
		return copyUserCard(userCards[weekStr]), nil
	}

	// Return the latest card (most recent week_start)
	var latestCard *repository.UserCard
	var latestWeek string
	for week, card := range userCards {
		if latestCard == nil || week > latestWeek {
			latestWeek = week
			latestCard = card
		}
	}
	return copyUserCard(latestCard), nil
}

// GetUserInfo retrieves user display info.
func (m *MockCardRepository) GetUserInfo(ctx context.Context, userID int64) (*repository.CardUser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetUserInfoCalls++
	if m.GetUserInfoError != nil {
		return nil, m.GetUserInfoError
	}

	return copyCardUser(m.Users[userID]), nil
}

// GetChatCards retrieves all cards for a chat with sorting and pagination.
func (m *MockCardRepository) GetChatCards(ctx context.Context, q repository.ChatCardsQuery) ([]repository.CardWithUser, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetChatCardsCalls++
	if m.GetChatCardsError != nil {
		return nil, 0, m.GetChatCardsError
	}

	chatCards := m.UserCards[q.ChatID]
	if chatCards == nil {
		return []repository.CardWithUser{}, 0, nil
	}

	// Determine week to query
	var weekStr string
	if q.WeekStart != nil {
		weekStr = q.WeekStart.Format("2006-01-02")
	} else {
		// Find the latest week
		for _, userCards := range chatCards {
			for week := range userCards {
				if weekStr == "" || week > weekStr {
					weekStr = week
				}
			}
		}
	}

	if weekStr == "" {
		return []repository.CardWithUser{}, 0, nil
	}

	// Collect all cards for this week
	var cards []repository.CardWithUser
	for userID, userCards := range chatCards {
		card, exists := userCards[weekStr]
		if !exists {
			continue
		}

		user := m.Users[userID]
		cwu := repository.CardWithUser{
			UserID:           card.UserID,
			WeekStart:        card.WeekStart,
			WeekEnd:          card.WeekEnd,
			Stats:            card.Stats,
			Trends:           card.Trends,
			MessagesAnalyzed: card.MessagesAnalyzed,
		}
		if user != nil {
			cwu.User = *user
		}
		cards = append(cards, cwu)
	}

	total := len(cards)

	// Apply pagination
	start := q.Offset
	if start >= len(cards) {
		return []repository.CardWithUser{}, total, nil
	}
	end := start + q.Limit
	if end > len(cards) {
		end = len(cards)
	}

	// Set ranks
	for i := range cards[start:end] {
		cards[start+i].Rank = start + i + 1
	}

	return cards[start:end], total, nil
}

// GetUserHistory retrieves a user's card history.
func (m *MockCardRepository) GetUserHistory(ctx context.Context, userID int64, chatID int64, limit int) ([]repository.UserCard, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetUserHistoryCalls++
	if m.GetUserHistoryError != nil {
		return nil, m.GetUserHistoryError
	}

	if m.UserCards[chatID] == nil || m.UserCards[chatID][userID] == nil {
		return []repository.UserCard{}, nil
	}

	userCards := m.UserCards[chatID][userID]

	// Collect and sort by week descending
	var weeks []string
	for week := range userCards {
		weeks = append(weeks, week)
	}
	// Sort descending
	for i := 0; i < len(weeks)-1; i++ {
		for j := i + 1; j < len(weeks); j++ {
			if weeks[j] > weeks[i] {
				weeks[i], weeks[j] = weeks[j], weeks[i]
			}
		}
	}

	// Apply limit
	if limit > 0 && limit < len(weeks) {
		weeks = weeks[:limit]
	}

	var history []repository.UserCard
	for _, week := range weeks {
		card := copyUserCard(userCards[week])
		if card != nil {
			history = append(history, *card)
		}
	}

	return history, nil
}

// GetAvailableWeeks returns weeks with generated cards for a chat.
func (m *MockCardRepository) GetAvailableWeeks(ctx context.Context, chatID int64) ([]repository.WeekInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetAvailableWeeksCalls++
	if m.GetAvailableWeeksError != nil {
		return nil, m.GetAvailableWeeksError
	}

	chatCards := m.UserCards[chatID]
	if chatCards == nil {
		return []repository.WeekInfo{}, nil
	}

	// Count cards per week
	weekCounts := make(map[string]int)
	weekEnds := make(map[string]string)
	for _, userCards := range chatCards {
		for _, card := range userCards {
			weekCounts[card.WeekStart]++
			weekEnds[card.WeekStart] = card.WeekEnd
		}
	}

	var weeks []repository.WeekInfo
	for weekStart, count := range weekCounts {
		weeks = append(weeks, repository.WeekInfo{
			WeekStart: weekStart,
			WeekEnd:   weekEnds[weekStart],
			CardCount: count,
		})
	}

	// Sort by week descending
	for i := 0; i < len(weeks)-1; i++ {
		for j := i + 1; j < len(weeks); j++ {
			if weeks[j].WeekStart > weeks[i].WeekStart {
				weeks[i], weeks[j] = weeks[j], weeks[i]
			}
		}
	}

	return weeks, nil
}

// GetCardImage retrieves a card image by user, chat, and optional week.
func (m *MockCardRepository) GetCardImage(ctx context.Context, userID int64, chatID int64, weekStart *time.Time, theme string) (*repository.CardImage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetCardImageCalls++
	if m.GetCardImageError != nil {
		return nil, m.GetCardImageError
	}

	if theme == "" {
		theme = "gaming"
	}

	if m.CardImages[chatID] == nil || m.CardImages[chatID][userID] == nil {
		return nil, nil
	}

	userImages := m.CardImages[chatID][userID]

	if weekStart != nil {
		weekStr := weekStart.Format("2006-01-02")
		if userImages[weekStr] == nil {
			return nil, nil
		}
		return copyCardImage(userImages[weekStr][theme]), nil
	}

	// Return the latest image (most recent week_start) for the theme
	var latestImage *repository.CardImage
	var latestWeek string
	for week, themeImages := range userImages {
		img, exists := themeImages[theme]
		if exists && (latestImage == nil || week > latestWeek) {
			latestWeek = week
			latestImage = img
		}
	}
	return copyCardImage(latestImage), nil
}

// GetGalleryWeeks returns weeks with generated card images for a chat.
func (m *MockCardRepository) GetGalleryWeeks(ctx context.Context, chatID int64) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetGalleryWeeksCalls++
	if m.GetGalleryWeeksError != nil {
		return nil, m.GetGalleryWeeksError
	}

	chatImages := m.CardImages[chatID]
	if chatImages == nil {
		return []string{}, nil
	}

	// Collect unique weeks
	weeksMap := make(map[string]bool)
	for _, userImages := range chatImages {
		for week := range userImages {
			weeksMap[week] = true
		}
	}

	var weeks []string
	for week := range weeksMap {
		weeks = append(weeks, week)
	}

	// Sort descending
	for i := 0; i < len(weeks)-1; i++ {
		for j := i + 1; j < len(weeks); j++ {
			if weeks[j] > weeks[i] {
				weeks[i], weeks[j] = weeks[j], weeks[i]
			}
		}
	}

	return weeks, nil
}

// GetGalleryImages returns card images for a chat/week with user info.
func (m *MockCardRepository) GetGalleryImages(ctx context.Context, chatID int64, weekStart *time.Time, userID *int64, theme *string) ([]repository.GalleryImage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetGalleryImagesCalls++
	if m.GetGalleryImagesError != nil {
		return nil, m.GetGalleryImagesError
	}

	chatImages := m.CardImages[chatID]
	if chatImages == nil {
		return []repository.GalleryImage{}, nil
	}

	// Determine week
	var weekStr string
	if weekStart != nil {
		weekStr = weekStart.Format("2006-01-02")
	} else {
		// Find latest week
		for _, userImages := range chatImages {
			for week := range userImages {
				if weekStr == "" || week > weekStr {
					weekStr = week
				}
			}
		}
	}

	if weekStr == "" {
		return []repository.GalleryImage{}, nil
	}

	var images []repository.GalleryImage
	for uid, userImages := range chatImages {
		// Filter by userID if provided
		if userID != nil && uid != *userID {
			continue
		}

		weekImages, exists := userImages[weekStr]
		if !exists {
			continue
		}

		for themeName, img := range weekImages {
			// Filter by theme if provided
			if theme != nil && themeName != *theme {
				continue
			}

			user := m.Users[uid]
			gi := repository.GalleryImage{
				ID:          img.ID,
				UserID:      img.UserID,
				ChatID:      img.ChatID,
				WeekStart:   img.WeekStart,
				StoragePath: img.StoragePath,
				Theme:       img.Theme,
				GeneratedAt: img.GeneratedAt,
			}
			if user != nil {
				gi.FirstName = &user.FirstName
				if user.LastName != "" {
					gi.LastName = &user.LastName
				}
				if user.Username != "" {
					gi.Username = &user.Username
				}
			}
			images = append(images, gi)
		}
	}

	return images, nil
}

// GetGalleryImageByID returns a single gallery image by ID.
func (m *MockCardRepository) GetGalleryImageByID(ctx context.Context, imageID int64) (*repository.GalleryImage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetGalleryImageByIDCalls++
	if m.GetGalleryImageByIDError != nil {
		return nil, m.GetGalleryImageByIDError
	}

	return copyGalleryImage(m.GalleryImages[imageID]), nil
}

// Ensure MockCardRepository implements the interface at compile time
var _ repository.CardRepositoryInterface = (*MockCardRepository)(nil)

// =============================================================================
// MockMiniAppRepository
// =============================================================================

// MockMiniAppRepository is a mock implementation of MiniAppRepositoryInterface for testing.
// It is thread-safe and can be used in concurrent test scenarios.
type MockMiniAppRepository struct {
	mu sync.RWMutex // protects all fields below

	// Storage maps
	// OverviewStats: chatID -> stats
	OverviewStats map[int64]*repository.OverviewStats
	// DailyActivity: chatID -> activities
	DailyActivity map[int64][]repository.DailyActivity
	// UserDailyActivity: chatID -> userID -> activities
	UserDailyActivity map[int64]map[int64][]repository.DailyActivity
	// UserRankings: chatID -> metric -> rankings
	UserRankings map[int64]map[string][]repository.UserRanking
	// UserRankingsTotal: chatID -> total count
	UserRankingsTotal map[int64]int
	// TopReactions: chatID -> reactions
	TopReactions map[int64][]repository.TopReaction
	// ReactionGivers: chatID -> users
	ReactionGivers map[int64][]repository.ReactionUser
	// ReactionReceivers: chatID -> users
	ReactionReceivers map[int64][]repository.ReactionUser
	// ProfileStats: chatID -> userID -> stats
	ProfileStats map[int64]map[int64]*repository.ProfileStats
	// TopReactorsToUser: chatID -> targetUserID -> interactors
	TopReactorsToUser map[int64]map[int64][]repository.TopInteractor
	// TopReactedToByUser: chatID -> actorUserID -> interactors
	TopReactedToByUser map[int64]map[int64][]repository.TopInteractor
	// TopRepliersToUser: chatID -> targetUserID -> interactors
	TopRepliersToUser map[int64]map[int64][]repository.TopInteractor
	// TopRepliedToByUser: chatID -> actorUserID -> interactors
	TopRepliedToByUser map[int64]map[int64][]repository.TopInteractor
	// ReplySenders: chatID -> users
	ReplySenders map[int64][]repository.ReactionUser
	// ReplyReceivers: chatID -> users
	ReplyReceivers map[int64][]repository.ReactionUser
	// GroupHeatmaps: chatID -> heatmap
	GroupHeatmaps map[int64]*repository.HeatmapData
	// UserHeatmaps: chatID -> userID -> heatmap
	UserHeatmaps map[int64]map[int64]*repository.HeatmapData
	// UserPhotoObjectKeys: userID -> objectKey
	UserPhotoObjectKeys map[int64]string
	// ChatTitles: chatID -> title
	ChatTitles map[int64]string
	// ChatUsers: chatID -> users
	ChatUsers map[int64][]repository.ChatUser
	// Users: userID -> user
	Users map[int64]*repository.ChatUser
	// MediaOverviewStats: chatID -> stats
	MediaOverviewStats map[int64]*repository.MediaOverviewStats
	// MediaTypeDistribution: chatID -> distribution
	MediaTypeDistribution map[int64][]repository.MediaTypeDistribution
	// MediaActivity: chatID -> activity
	MediaActivity map[int64][]repository.MediaActivity
	// TopMediaSenders: chatID -> users
	TopMediaSenders map[int64][]repository.MediaUser
	// ChatTimezones: chatID -> timezone
	ChatTimezones map[int64]string

	// Call tracking
	GetOverviewStatsCalls          int
	GetDailyActivityCalls          int
	GetUserDailyActivityCalls      int
	GetUserRankingsCalls           int
	GetUserRankingsTotalCalls      int
	GetTopReactionsCalls           int
	GetTopReactionGiversCalls      int
	GetTopReactionReceiversCalls   int
	GetUserProfileStatsCalls       int
	GetTopReactorsToUserCalls      int
	GetTopReactedToByUserCalls     int
	GetTopRepliersToUserCalls      int
	GetTopRepliedToByUserCalls     int
	GetTopReplySendersCalls        int
	GetTopReplyReceiversCalls      int
	GetGroupHeatmapCalls           int
	GetUserHeatmapCalls            int
	GetUserPhotoObjectKeyCalls     int
	GetChatTitleCalls              int
	GetChatUsersCalls              int
	GetUserInfoCalls               int
	GetMediaOverviewStatsCalls     int
	GetMediaTypeDistributionCalls  int
	GetMediaActivityCalls          int
	GetTopMediaSendersCalls        int
	GetChatTimezoneCalls           int
	SetChatTimezoneCalls           int

	// Error injection
	GetOverviewStatsError          error
	GetDailyActivityError          error
	GetUserDailyActivityError      error
	GetUserRankingsError           error
	GetUserRankingsTotalError      error
	GetTopReactionsError           error
	GetTopReactionGiversError      error
	GetTopReactionReceiversError   error
	GetUserProfileStatsError       error
	GetTopReactorsToUserError      error
	GetTopReactedToByUserError     error
	GetTopRepliersToUserError      error
	GetTopRepliedToByUserError     error
	GetTopReplySendersError        error
	GetTopReplyReceiversError      error
	GetGroupHeatmapError           error
	GetUserHeatmapError            error
	GetUserPhotoObjectKeyError     error
	GetChatTitleError              error
	GetChatUsersError              error
	GetUserInfoError               error
	GetMediaOverviewStatsError     error
	GetMediaTypeDistributionError  error
	GetMediaActivityError          error
	GetTopMediaSendersError        error
	GetChatTimezoneError           error
	SetChatTimezoneError           error
}

// NewMockMiniAppRepository creates a new MockMiniAppRepository with initialized storage.
func NewMockMiniAppRepository() *MockMiniAppRepository {
	return &MockMiniAppRepository{
		OverviewStats:         make(map[int64]*repository.OverviewStats),
		DailyActivity:         make(map[int64][]repository.DailyActivity),
		UserDailyActivity:     make(map[int64]map[int64][]repository.DailyActivity),
		UserRankings:          make(map[int64]map[string][]repository.UserRanking),
		UserRankingsTotal:     make(map[int64]int),
		TopReactions:          make(map[int64][]repository.TopReaction),
		ReactionGivers:        make(map[int64][]repository.ReactionUser),
		ReactionReceivers:     make(map[int64][]repository.ReactionUser),
		ProfileStats:          make(map[int64]map[int64]*repository.ProfileStats),
		TopReactorsToUser:     make(map[int64]map[int64][]repository.TopInteractor),
		TopReactedToByUser:    make(map[int64]map[int64][]repository.TopInteractor),
		TopRepliersToUser:     make(map[int64]map[int64][]repository.TopInteractor),
		TopRepliedToByUser:    make(map[int64]map[int64][]repository.TopInteractor),
		ReplySenders:          make(map[int64][]repository.ReactionUser),
		ReplyReceivers:        make(map[int64][]repository.ReactionUser),
		GroupHeatmaps:         make(map[int64]*repository.HeatmapData),
		UserHeatmaps:          make(map[int64]map[int64]*repository.HeatmapData),
		UserPhotoObjectKeys:   make(map[int64]string),
		ChatTitles:            make(map[int64]string),
		ChatUsers:             make(map[int64][]repository.ChatUser),
		Users:                 make(map[int64]*repository.ChatUser),
		MediaOverviewStats:    make(map[int64]*repository.MediaOverviewStats),
		MediaTypeDistribution: make(map[int64][]repository.MediaTypeDistribution),
		MediaActivity:         make(map[int64][]repository.MediaActivity),
		TopMediaSenders:       make(map[int64][]repository.MediaUser),
		ChatTimezones:         make(map[int64]string),
	}
}

// Reset clears all storage and resets call counters and errors.
func (m *MockMiniAppRepository) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.OverviewStats = make(map[int64]*repository.OverviewStats)
	m.DailyActivity = make(map[int64][]repository.DailyActivity)
	m.UserDailyActivity = make(map[int64]map[int64][]repository.DailyActivity)
	m.UserRankings = make(map[int64]map[string][]repository.UserRanking)
	m.UserRankingsTotal = make(map[int64]int)
	m.TopReactions = make(map[int64][]repository.TopReaction)
	m.ReactionGivers = make(map[int64][]repository.ReactionUser)
	m.ReactionReceivers = make(map[int64][]repository.ReactionUser)
	m.ProfileStats = make(map[int64]map[int64]*repository.ProfileStats)
	m.TopReactorsToUser = make(map[int64]map[int64][]repository.TopInteractor)
	m.TopReactedToByUser = make(map[int64]map[int64][]repository.TopInteractor)
	m.TopRepliersToUser = make(map[int64]map[int64][]repository.TopInteractor)
	m.TopRepliedToByUser = make(map[int64]map[int64][]repository.TopInteractor)
	m.ReplySenders = make(map[int64][]repository.ReactionUser)
	m.ReplyReceivers = make(map[int64][]repository.ReactionUser)
	m.GroupHeatmaps = make(map[int64]*repository.HeatmapData)
	m.UserHeatmaps = make(map[int64]map[int64]*repository.HeatmapData)
	m.UserPhotoObjectKeys = make(map[int64]string)
	m.ChatTitles = make(map[int64]string)
	m.ChatUsers = make(map[int64][]repository.ChatUser)
	m.Users = make(map[int64]*repository.ChatUser)
	m.MediaOverviewStats = make(map[int64]*repository.MediaOverviewStats)
	m.MediaTypeDistribution = make(map[int64][]repository.MediaTypeDistribution)
	m.MediaActivity = make(map[int64][]repository.MediaActivity)
	m.TopMediaSenders = make(map[int64][]repository.MediaUser)
	m.ChatTimezones = make(map[int64]string)

	// Reset call counters
	m.GetOverviewStatsCalls = 0
	m.GetDailyActivityCalls = 0
	m.GetUserDailyActivityCalls = 0
	m.GetUserRankingsCalls = 0
	m.GetUserRankingsTotalCalls = 0
	m.GetTopReactionsCalls = 0
	m.GetTopReactionGiversCalls = 0
	m.GetTopReactionReceiversCalls = 0
	m.GetUserProfileStatsCalls = 0
	m.GetTopReactorsToUserCalls = 0
	m.GetTopReactedToByUserCalls = 0
	m.GetTopRepliersToUserCalls = 0
	m.GetTopRepliedToByUserCalls = 0
	m.GetTopReplySendersCalls = 0
	m.GetTopReplyReceiversCalls = 0
	m.GetGroupHeatmapCalls = 0
	m.GetUserHeatmapCalls = 0
	m.GetUserPhotoObjectKeyCalls = 0
	m.GetChatTitleCalls = 0
	m.GetChatUsersCalls = 0
	m.GetUserInfoCalls = 0
	m.GetMediaOverviewStatsCalls = 0
	m.GetMediaTypeDistributionCalls = 0
	m.GetMediaActivityCalls = 0
	m.GetTopMediaSendersCalls = 0
	m.GetChatTimezoneCalls = 0
	m.SetChatTimezoneCalls = 0

	// Reset errors
	m.GetOverviewStatsError = nil
	m.GetDailyActivityError = nil
	m.GetUserDailyActivityError = nil
	m.GetUserRankingsError = nil
	m.GetUserRankingsTotalError = nil
	m.GetTopReactionsError = nil
	m.GetTopReactionGiversError = nil
	m.GetTopReactionReceiversError = nil
	m.GetUserProfileStatsError = nil
	m.GetTopReactorsToUserError = nil
	m.GetTopReactedToByUserError = nil
	m.GetTopRepliersToUserError = nil
	m.GetTopRepliedToByUserError = nil
	m.GetTopReplySendersError = nil
	m.GetTopReplyReceiversError = nil
	m.GetGroupHeatmapError = nil
	m.GetUserHeatmapError = nil
	m.GetUserPhotoObjectKeyError = nil
	m.GetChatTitleError = nil
	m.GetChatUsersError = nil
	m.GetUserInfoError = nil
	m.GetMediaOverviewStatsError = nil
	m.GetMediaTypeDistributionError = nil
	m.GetMediaActivityError = nil
	m.GetTopMediaSendersError = nil
	m.GetChatTimezoneError = nil
	m.SetChatTimezoneError = nil
}

// =============================================================================
// Helper functions to populate test data
// =============================================================================

// SetOverviewStats sets overview stats for a chat.
func (m *MockMiniAppRepository) SetOverviewStats(chatID int64, stats *repository.OverviewStats) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.OverviewStats[chatID] = stats
}

// SetDailyActivity sets daily activity for a chat.
func (m *MockMiniAppRepository) SetDailyActivity(chatID int64, activity []repository.DailyActivity) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DailyActivity[chatID] = activity
}

// SetUserDailyActivity sets daily activity for a user in a chat.
func (m *MockMiniAppRepository) SetUserDailyActivity(chatID, userID int64, activity []repository.DailyActivity) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.UserDailyActivity[chatID] == nil {
		m.UserDailyActivity[chatID] = make(map[int64][]repository.DailyActivity)
	}
	m.UserDailyActivity[chatID][userID] = activity
}

// SetUserRankings sets user rankings for a chat and metric.
func (m *MockMiniAppRepository) SetUserRankings(chatID int64, metric string, rankings []repository.UserRanking) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.UserRankings[chatID] == nil {
		m.UserRankings[chatID] = make(map[string][]repository.UserRanking)
	}
	m.UserRankings[chatID][metric] = rankings
}

// SetUserRankingsTotal sets the total user count for a chat.
func (m *MockMiniAppRepository) SetUserRankingsTotal(chatID int64, total int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UserRankingsTotal[chatID] = total
}

// SetTopReactions sets top reactions for a chat.
func (m *MockMiniAppRepository) SetTopReactions(chatID int64, reactions []repository.TopReaction) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TopReactions[chatID] = reactions
}

// SetReactionGivers sets top reaction givers for a chat.
func (m *MockMiniAppRepository) SetReactionGivers(chatID int64, users []repository.ReactionUser) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ReactionGivers[chatID] = users
}

// SetReactionReceivers sets top reaction receivers for a chat.
func (m *MockMiniAppRepository) SetReactionReceivers(chatID int64, users []repository.ReactionUser) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ReactionReceivers[chatID] = users
}

// SetProfileStats sets profile stats for a user in a chat.
func (m *MockMiniAppRepository) SetProfileStats(chatID, userID int64, stats *repository.ProfileStats) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ProfileStats[chatID] == nil {
		m.ProfileStats[chatID] = make(map[int64]*repository.ProfileStats)
	}
	m.ProfileStats[chatID][userID] = stats
}

// SetTopReactorsToUser sets top reactors to a user.
func (m *MockMiniAppRepository) SetTopReactorsToUser(chatID, userID int64, interactors []repository.TopInteractor) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.TopReactorsToUser[chatID] == nil {
		m.TopReactorsToUser[chatID] = make(map[int64][]repository.TopInteractor)
	}
	m.TopReactorsToUser[chatID][userID] = interactors
}

// SetTopReactedToByUser sets users most reacted to by a user.
func (m *MockMiniAppRepository) SetTopReactedToByUser(chatID, userID int64, interactors []repository.TopInteractor) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.TopReactedToByUser[chatID] == nil {
		m.TopReactedToByUser[chatID] = make(map[int64][]repository.TopInteractor)
	}
	m.TopReactedToByUser[chatID][userID] = interactors
}

// SetTopRepliersToUser sets top repliers to a user.
func (m *MockMiniAppRepository) SetTopRepliersToUser(chatID, userID int64, interactors []repository.TopInteractor) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.TopRepliersToUser[chatID] == nil {
		m.TopRepliersToUser[chatID] = make(map[int64][]repository.TopInteractor)
	}
	m.TopRepliersToUser[chatID][userID] = interactors
}

// SetTopRepliedToByUser sets users most replied to by a user.
func (m *MockMiniAppRepository) SetTopRepliedToByUser(chatID, userID int64, interactors []repository.TopInteractor) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.TopRepliedToByUser[chatID] == nil {
		m.TopRepliedToByUser[chatID] = make(map[int64][]repository.TopInteractor)
	}
	m.TopRepliedToByUser[chatID][userID] = interactors
}

// SetReplySenders sets top reply senders for a chat.
func (m *MockMiniAppRepository) SetReplySenders(chatID int64, users []repository.ReactionUser) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ReplySenders[chatID] = users
}

// SetReplyReceivers sets top reply receivers for a chat.
func (m *MockMiniAppRepository) SetReplyReceivers(chatID int64, users []repository.ReactionUser) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ReplyReceivers[chatID] = users
}

// SetGroupHeatmap sets the heatmap for a chat.
func (m *MockMiniAppRepository) SetGroupHeatmap(chatID int64, heatmap *repository.HeatmapData) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GroupHeatmaps[chatID] = heatmap
}

// SetUserHeatmap sets the heatmap for a user in a chat.
func (m *MockMiniAppRepository) SetUserHeatmap(chatID, userID int64, heatmap *repository.HeatmapData) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.UserHeatmaps[chatID] == nil {
		m.UserHeatmaps[chatID] = make(map[int64]*repository.HeatmapData)
	}
	m.UserHeatmaps[chatID][userID] = heatmap
}

// SetUserPhotoObjectKey sets the photo object key for a user.
func (m *MockMiniAppRepository) SetUserPhotoObjectKey(userID int64, objectKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UserPhotoObjectKeys[userID] = objectKey
}

// SetChatTitle sets the title for a chat.
func (m *MockMiniAppRepository) SetChatTitle(chatID int64, title string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ChatTitles[chatID] = title
}

// SetChatUsers sets the users for a chat.
func (m *MockMiniAppRepository) SetChatUsers(chatID int64, users []repository.ChatUser) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ChatUsers[chatID] = users
}

// AddUser adds a user to the mock storage.
func (m *MockMiniAppRepository) AddUser(user *repository.ChatUser) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Users[user.UserID] = user
}

// SetMediaOverviewStats sets media overview stats for a chat.
func (m *MockMiniAppRepository) SetMediaOverviewStats(chatID int64, stats *repository.MediaOverviewStats) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MediaOverviewStats[chatID] = stats
}

// SetMediaTypeDistribution sets media type distribution for a chat.
func (m *MockMiniAppRepository) SetMediaTypeDistribution(chatID int64, distribution []repository.MediaTypeDistribution) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MediaTypeDistribution[chatID] = distribution
}

// SetMediaActivity sets media activity for a chat.
func (m *MockMiniAppRepository) SetMediaActivity(chatID int64, activity []repository.MediaActivity) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MediaActivity[chatID] = activity
}

// SetTopMediaSenders sets top media senders for a chat.
func (m *MockMiniAppRepository) SetTopMediaSenders(chatID int64, users []repository.MediaUser) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TopMediaSenders[chatID] = users
}

// SetChatTimezone sets the timezone for a chat.
func (m *MockMiniAppRepository) SetChatTimezoneValue(chatID int64, timezone string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ChatTimezones[chatID] = timezone
}

// =============================================================================
// Deep copy functions
// =============================================================================

// copyOverviewStats creates a deep copy of OverviewStats.
func copyOverviewStats(s *repository.OverviewStats) *repository.OverviewStats {
	if s == nil {
		return nil
	}
	cp := *s
	if s.TotalMessagesTrend != nil {
		v := *s.TotalMessagesTrend
		cp.TotalMessagesTrend = &v
	}
	if s.TotalUsersTrend != nil {
		v := *s.TotalUsersTrend
		cp.TotalUsersTrend = &v
	}
	if s.TotalReactionsTrend != nil {
		v := *s.TotalReactionsTrend
		cp.TotalReactionsTrend = &v
	}
	if s.TotalMediaTrend != nil {
		v := *s.TotalMediaTrend
		cp.TotalMediaTrend = &v
	}
	if s.MessagesPerDayTrend != nil {
		v := *s.MessagesPerDayTrend
		cp.MessagesPerDayTrend = &v
	}
	return &cp
}

// copyProfileStats creates a deep copy of ProfileStats.
func copyProfileStats(s *repository.ProfileStats) *repository.ProfileStats {
	if s == nil {
		return nil
	}
	cp := *s
	if s.MessageCountTrend != nil {
		v := *s.MessageCountTrend
		cp.MessageCountTrend = &v
	}
	if s.ReactionsSentTrend != nil {
		v := *s.ReactionsSentTrend
		cp.ReactionsSentTrend = &v
	}
	if s.ReactionsReceivedTrend != nil {
		v := *s.ReactionsReceivedTrend
		cp.ReactionsReceivedTrend = &v
	}
	if s.RepliesSentTrend != nil {
		v := *s.RepliesSentTrend
		cp.RepliesSentTrend = &v
	}
	if s.RepliesReceivedTrend != nil {
		v := *s.RepliesReceivedTrend
		cp.RepliesReceivedTrend = &v
	}
	if s.ActiveDaysTrend != nil {
		v := *s.ActiveDaysTrend
		cp.ActiveDaysTrend = &v
	}
	if s.AvgMessagesPerDayTrend != nil {
		v := *s.AvgMessagesPerDayTrend
		cp.AvgMessagesPerDayTrend = &v
	}
	return &cp
}

// copyHeatmapData creates a deep copy of HeatmapData.
func copyHeatmapData(h *repository.HeatmapData) *repository.HeatmapData {
	if h == nil {
		return nil
	}
	cp := *h
	if h.Data != nil {
		cp.Data = make([]repository.HeatmapCell, len(h.Data))
		copy(cp.Data, h.Data)
	}
	return &cp
}

// copyChatUser creates a deep copy of ChatUser.
func copyChatUser(u *repository.ChatUser) *repository.ChatUser {
	if u == nil {
		return nil
	}
	cp := *u
	if u.LastName != nil {
		v := *u.LastName
		cp.LastName = &v
	}
	if u.Username != nil {
		v := *u.Username
		cp.Username = &v
	}
	if u.PhotoObjectKey != nil {
		v := *u.PhotoObjectKey
		cp.PhotoObjectKey = &v
	}
	return &cp
}

// copyMediaOverviewStats creates a deep copy of MediaOverviewStats.
func copyMediaOverviewStats(s *repository.MediaOverviewStats) *repository.MediaOverviewStats {
	if s == nil {
		return nil
	}
	cp := *s
	if s.TotalMediaTrend != nil {
		v := *s.TotalMediaTrend
		cp.TotalMediaTrend = &v
	}
	if s.TotalPhotosTrend != nil {
		v := *s.TotalPhotosTrend
		cp.TotalPhotosTrend = &v
	}
	if s.TotalVideosTrend != nil {
		v := *s.TotalVideosTrend
		cp.TotalVideosTrend = &v
	}
	if s.TotalGifsTrend != nil {
		v := *s.TotalGifsTrend
		cp.TotalGifsTrend = &v
	}
	if s.TotalVoiceTrend != nil {
		v := *s.TotalVoiceTrend
		cp.TotalVoiceTrend = &v
	}
	if s.TotalDocumentsTrend != nil {
		v := *s.TotalDocumentsTrend
		cp.TotalDocumentsTrend = &v
	}
	if s.TotalStickersTrend != nil {
		v := *s.TotalStickersTrend
		cp.TotalStickersTrend = &v
	}
	if s.MediaPerDayTrend != nil {
		v := *s.MediaPerDayTrend
		cp.MediaPerDayTrend = &v
	}
	return &cp
}

// =============================================================================
// Interface implementation
// =============================================================================

// GetOverviewStats returns overview statistics for a chat.
func (m *MockMiniAppRepository) GetOverviewStats(ctx context.Context, chatID int64, startDate, endDate *time.Time) (*repository.OverviewStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetOverviewStatsCalls++
	if m.GetOverviewStatsError != nil {
		return nil, m.GetOverviewStatsError
	}

	return copyOverviewStats(m.OverviewStats[chatID]), nil
}

// GetDailyActivity returns daily message activity for a chat.
func (m *MockMiniAppRepository) GetDailyActivity(ctx context.Context, chatID int64, startDate, endDate *time.Time, tz *time.Location) ([]repository.DailyActivity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetDailyActivityCalls++
	if m.GetDailyActivityError != nil {
		return nil, m.GetDailyActivityError
	}

	activity := m.DailyActivity[chatID]
	if activity == nil {
		return []repository.DailyActivity{}, nil
	}
	// Return a copy
	result := make([]repository.DailyActivity, len(activity))
	copy(result, activity)
	return result, nil
}

// GetUserDailyActivity returns daily message activity for a specific user in a chat.
func (m *MockMiniAppRepository) GetUserDailyActivity(ctx context.Context, chatID, userID int64, startDate, endDate *time.Time, tz *time.Location) ([]repository.DailyActivity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetUserDailyActivityCalls++
	if m.GetUserDailyActivityError != nil {
		return nil, m.GetUserDailyActivityError
	}

	if m.UserDailyActivity[chatID] == nil {
		return []repository.DailyActivity{}, nil
	}
	activity := m.UserDailyActivity[chatID][userID]
	if activity == nil {
		return []repository.DailyActivity{}, nil
	}
	// Return a copy
	result := make([]repository.DailyActivity, len(activity))
	copy(result, activity)
	return result, nil
}

// GetUserRankings returns user rankings for a chat.
func (m *MockMiniAppRepository) GetUserRankings(ctx context.Context, chatID int64, metric string, limit, offset int, startDate, endDate *time.Time, tzName string) ([]repository.UserRanking, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetUserRankingsCalls++
	if m.GetUserRankingsError != nil {
		return nil, m.GetUserRankingsError
	}

	if m.UserRankings[chatID] == nil {
		return []repository.UserRanking{}, nil
	}
	rankings := m.UserRankings[chatID][metric]
	if rankings == nil {
		return []repository.UserRanking{}, nil
	}

	// Apply pagination
	start := offset
	if start >= len(rankings) {
		return []repository.UserRanking{}, nil
	}
	end := start + limit
	if end > len(rankings) {
		end = len(rankings)
	}

	// Return a copy of the slice
	result := make([]repository.UserRanking, end-start)
	copy(result, rankings[start:end])
	return result, nil
}

// GetUserRankingsTotal returns the total count of users for pagination.
func (m *MockMiniAppRepository) GetUserRankingsTotal(ctx context.Context, chatID int64, startDate, endDate *time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetUserRankingsTotalCalls++
	if m.GetUserRankingsTotalError != nil {
		return 0, m.GetUserRankingsTotalError
	}

	return m.UserRankingsTotal[chatID], nil
}

// GetTopReactions returns the top reactions used in a chat.
func (m *MockMiniAppRepository) GetTopReactions(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]repository.TopReaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetTopReactionsCalls++
	if m.GetTopReactionsError != nil {
		return nil, m.GetTopReactionsError
	}

	reactions := m.TopReactions[chatID]
	if reactions == nil {
		return []repository.TopReaction{}, nil
	}

	// Apply limit
	end := limit
	if end > len(reactions) {
		end = len(reactions)
	}

	result := make([]repository.TopReaction, end)
	copy(result, reactions[:end])
	return result, nil
}

// GetTopReactionGivers returns users who give the most reactions.
func (m *MockMiniAppRepository) GetTopReactionGivers(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]repository.ReactionUser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetTopReactionGiversCalls++
	if m.GetTopReactionGiversError != nil {
		return nil, m.GetTopReactionGiversError
	}

	users := m.ReactionGivers[chatID]
	if users == nil {
		return []repository.ReactionUser{}, nil
	}

	// Apply limit
	end := limit
	if end > len(users) {
		end = len(users)
	}

	result := make([]repository.ReactionUser, end)
	copy(result, users[:end])
	return result, nil
}

// GetTopReactionReceivers returns users who receive the most reactions.
func (m *MockMiniAppRepository) GetTopReactionReceivers(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]repository.ReactionUser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetTopReactionReceiversCalls++
	if m.GetTopReactionReceiversError != nil {
		return nil, m.GetTopReactionReceiversError
	}

	users := m.ReactionReceivers[chatID]
	if users == nil {
		return []repository.ReactionUser{}, nil
	}

	// Apply limit
	end := limit
	if end > len(users) {
		end = len(users)
	}

	result := make([]repository.ReactionUser, end)
	copy(result, users[:end])
	return result, nil
}

// GetUserProfileStats returns personal stats for a user including their rankings.
func (m *MockMiniAppRepository) GetUserProfileStats(ctx context.Context, chatID, userID int64, startDate, endDate *time.Time, tzName string) (*repository.ProfileStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetUserProfileStatsCalls++
	if m.GetUserProfileStatsError != nil {
		return nil, m.GetUserProfileStatsError
	}

	if m.ProfileStats[chatID] == nil {
		return nil, nil
	}
	return copyProfileStats(m.ProfileStats[chatID][userID]), nil
}

// GetTopReactorsToUser returns users who react most to a specific user's messages.
func (m *MockMiniAppRepository) GetTopReactorsToUser(ctx context.Context, chatID, userID int64, limit int, startDate, endDate *time.Time) ([]repository.TopInteractor, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetTopReactorsToUserCalls++
	if m.GetTopReactorsToUserError != nil {
		return nil, m.GetTopReactorsToUserError
	}

	if m.TopReactorsToUser[chatID] == nil {
		return []repository.TopInteractor{}, nil
	}
	interactors := m.TopReactorsToUser[chatID][userID]
	if interactors == nil {
		return []repository.TopInteractor{}, nil
	}

	// Apply limit
	end := limit
	if end > len(interactors) {
		end = len(interactors)
	}

	result := make([]repository.TopInteractor, end)
	copy(result, interactors[:end])
	return result, nil
}

// GetTopReactedToByUser returns users whose messages a specific user reacts to most.
func (m *MockMiniAppRepository) GetTopReactedToByUser(ctx context.Context, chatID, userID int64, limit int, startDate, endDate *time.Time) ([]repository.TopInteractor, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetTopReactedToByUserCalls++
	if m.GetTopReactedToByUserError != nil {
		return nil, m.GetTopReactedToByUserError
	}

	if m.TopReactedToByUser[chatID] == nil {
		return []repository.TopInteractor{}, nil
	}
	interactors := m.TopReactedToByUser[chatID][userID]
	if interactors == nil {
		return []repository.TopInteractor{}, nil
	}

	// Apply limit
	end := limit
	if end > len(interactors) {
		end = len(interactors)
	}

	result := make([]repository.TopInteractor, end)
	copy(result, interactors[:end])
	return result, nil
}

// GetTopRepliersToUser returns users who reply most to a specific user's messages.
func (m *MockMiniAppRepository) GetTopRepliersToUser(ctx context.Context, chatID, userID int64, limit int, startDate, endDate *time.Time) ([]repository.TopInteractor, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetTopRepliersToUserCalls++
	if m.GetTopRepliersToUserError != nil {
		return nil, m.GetTopRepliersToUserError
	}

	if m.TopRepliersToUser[chatID] == nil {
		return []repository.TopInteractor{}, nil
	}
	interactors := m.TopRepliersToUser[chatID][userID]
	if interactors == nil {
		return []repository.TopInteractor{}, nil
	}

	// Apply limit
	end := limit
	if end > len(interactors) {
		end = len(interactors)
	}

	result := make([]repository.TopInteractor, end)
	copy(result, interactors[:end])
	return result, nil
}

// GetTopRepliedToByUser returns users that a specific user replies to most.
func (m *MockMiniAppRepository) GetTopRepliedToByUser(ctx context.Context, chatID, userID int64, limit int, startDate, endDate *time.Time) ([]repository.TopInteractor, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetTopRepliedToByUserCalls++
	if m.GetTopRepliedToByUserError != nil {
		return nil, m.GetTopRepliedToByUserError
	}

	if m.TopRepliedToByUser[chatID] == nil {
		return []repository.TopInteractor{}, nil
	}
	interactors := m.TopRepliedToByUser[chatID][userID]
	if interactors == nil {
		return []repository.TopInteractor{}, nil
	}

	// Apply limit
	end := limit
	if end > len(interactors) {
		end = len(interactors)
	}

	result := make([]repository.TopInteractor, end)
	copy(result, interactors[:end])
	return result, nil
}

// GetTopReplySenders returns users who send the most replies in a chat.
func (m *MockMiniAppRepository) GetTopReplySenders(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]repository.ReactionUser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetTopReplySendersCalls++
	if m.GetTopReplySendersError != nil {
		return nil, m.GetTopReplySendersError
	}

	users := m.ReplySenders[chatID]
	if users == nil {
		return []repository.ReactionUser{}, nil
	}

	// Apply limit
	end := limit
	if end > len(users) {
		end = len(users)
	}

	result := make([]repository.ReactionUser, end)
	copy(result, users[:end])
	return result, nil
}

// GetTopReplyReceivers returns users whose messages receive the most replies in a chat.
func (m *MockMiniAppRepository) GetTopReplyReceivers(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]repository.ReactionUser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetTopReplyReceiversCalls++
	if m.GetTopReplyReceiversError != nil {
		return nil, m.GetTopReplyReceiversError
	}

	users := m.ReplyReceivers[chatID]
	if users == nil {
		return []repository.ReactionUser{}, nil
	}

	// Apply limit
	end := limit
	if end > len(users) {
		end = len(users)
	}

	result := make([]repository.ReactionUser, end)
	copy(result, users[:end])
	return result, nil
}

// GetGroupHeatmap returns the activity heatmap for a chat.
func (m *MockMiniAppRepository) GetGroupHeatmap(ctx context.Context, chatID int64, startDate, endDate *time.Time, tz *time.Location) (*repository.HeatmapData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetGroupHeatmapCalls++
	if m.GetGroupHeatmapError != nil {
		return nil, m.GetGroupHeatmapError
	}

	return copyHeatmapData(m.GroupHeatmaps[chatID]), nil
}

// GetUserHeatmap returns the activity heatmap for a specific user.
func (m *MockMiniAppRepository) GetUserHeatmap(ctx context.Context, chatID, userID int64, startDate, endDate *time.Time, tz *time.Location) (*repository.HeatmapData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetUserHeatmapCalls++
	if m.GetUserHeatmapError != nil {
		return nil, m.GetUserHeatmapError
	}

	if m.UserHeatmaps[chatID] == nil {
		return nil, nil
	}
	return copyHeatmapData(m.UserHeatmaps[chatID][userID]), nil
}

// GetUserPhotoObjectKey returns the largest profile photo object key for a user.
func (m *MockMiniAppRepository) GetUserPhotoObjectKey(ctx context.Context, userID int64) (*string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetUserPhotoObjectKeyCalls++
	if m.GetUserPhotoObjectKeyError != nil {
		return nil, m.GetUserPhotoObjectKeyError
	}

	key, exists := m.UserPhotoObjectKeys[userID]
	if !exists {
		return nil, nil
	}
	return &key, nil
}

// GetChatTitle returns the title of a chat.
func (m *MockMiniAppRepository) GetChatTitle(ctx context.Context, chatID int64) (*string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetChatTitleCalls++
	if m.GetChatTitleError != nil {
		return nil, m.GetChatTitleError
	}

	title, exists := m.ChatTitles[chatID]
	if !exists {
		return nil, nil
	}
	return &title, nil
}

// GetChatUsers returns all non-bot users who have sent messages in a chat.
func (m *MockMiniAppRepository) GetChatUsers(ctx context.Context, chatID int64) ([]repository.ChatUser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetChatUsersCalls++
	if m.GetChatUsersError != nil {
		return nil, m.GetChatUsersError
	}

	users := m.ChatUsers[chatID]
	if users == nil {
		return []repository.ChatUser{}, nil
	}

	// Return a copy
	result := make([]repository.ChatUser, len(users))
	copy(result, users)
	return result, nil
}

// GetUserInfo returns basic info for a specific user.
func (m *MockMiniAppRepository) GetUserInfo(ctx context.Context, userID int64) (*repository.ChatUser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetUserInfoCalls++
	if m.GetUserInfoError != nil {
		return nil, m.GetUserInfoError
	}

	return copyChatUser(m.Users[userID]), nil
}

// GetMediaOverviewStats returns aggregate media statistics for a chat.
func (m *MockMiniAppRepository) GetMediaOverviewStats(ctx context.Context, chatID int64, startDate, endDate *time.Time) (*repository.MediaOverviewStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetMediaOverviewStatsCalls++
	if m.GetMediaOverviewStatsError != nil {
		return nil, m.GetMediaOverviewStatsError
	}

	return copyMediaOverviewStats(m.MediaOverviewStats[chatID]), nil
}

// GetMediaTypeDistribution returns the distribution of media types for a chat.
func (m *MockMiniAppRepository) GetMediaTypeDistribution(ctx context.Context, chatID int64, startDate, endDate *time.Time) ([]repository.MediaTypeDistribution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetMediaTypeDistributionCalls++
	if m.GetMediaTypeDistributionError != nil {
		return nil, m.GetMediaTypeDistributionError
	}

	dist := m.MediaTypeDistribution[chatID]
	if dist == nil {
		return []repository.MediaTypeDistribution{}, nil
	}

	// Return a copy
	result := make([]repository.MediaTypeDistribution, len(dist))
	copy(result, dist)
	return result, nil
}

// GetMediaActivity returns daily media upload activity for a chat.
func (m *MockMiniAppRepository) GetMediaActivity(ctx context.Context, chatID int64, startDate, endDate *time.Time, tz *time.Location) ([]repository.MediaActivity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetMediaActivityCalls++
	if m.GetMediaActivityError != nil {
		return nil, m.GetMediaActivityError
	}

	activity := m.MediaActivity[chatID]
	if activity == nil {
		return []repository.MediaActivity{}, nil
	}

	// Return a copy
	result := make([]repository.MediaActivity, len(activity))
	copy(result, activity)
	return result, nil
}

// GetTopMediaSenders returns users who send the most media in a chat.
func (m *MockMiniAppRepository) GetTopMediaSenders(ctx context.Context, chatID int64, limit int, startDate, endDate *time.Time) ([]repository.MediaUser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetTopMediaSendersCalls++
	if m.GetTopMediaSendersError != nil {
		return nil, m.GetTopMediaSendersError
	}

	users := m.TopMediaSenders[chatID]
	if users == nil {
		return []repository.MediaUser{}, nil
	}

	// Apply limit
	end := limit
	if end > len(users) {
		end = len(users)
	}

	result := make([]repository.MediaUser, end)
	copy(result, users[:end])
	return result, nil
}

// GetChatTimezone returns the configured timezone for a chat, or nil if not set.
func (m *MockMiniAppRepository) GetChatTimezone(ctx context.Context, chatID int64) (*string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetChatTimezoneCalls++
	if m.GetChatTimezoneError != nil {
		return nil, m.GetChatTimezoneError
	}

	tz, exists := m.ChatTimezones[chatID]
	if !exists {
		return nil, nil
	}
	return &tz, nil
}

// SetChatTimezone sets the timezone for a chat. Only admins should call this.
func (m *MockMiniAppRepository) SetChatTimezone(ctx context.Context, chatID int64, timezone string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SetChatTimezoneCalls++
	if m.SetChatTimezoneError != nil {
		return m.SetChatTimezoneError
	}

	m.ChatTimezones[chatID] = timezone
	return nil
}

// Ensure MockMiniAppRepository implements the interface at compile time
var _ repository.MiniAppRepositoryInterface = (*MockMiniAppRepository)(nil)

// =============================================================================
// MockMLRepository
// =============================================================================

// MockMLRepository is a mock implementation of MLRepositoryInterface for testing.
// It is thread-safe and can be used in concurrent test scenarios.
type MockMLRepository struct {
	mu sync.RWMutex // protects all fields below

	// Storage
	UnprocessedMessages []repository.UnprocessedMessage
	ProcessingStats     map[string]int64
	SentimentResults    []repository.SentimentResult
	ToxicityResults     []repository.ToxicityResult
	HumorResults        []repository.HumorResult
	QuestionResults     []repository.QuestionResult
	NERResults          []repository.NERResult
	Topics              map[int64]map[int][]string // chatID -> topicID -> keywords
	MessageTopics       []repository.MessageTopicResult
	ProcessedMessages   map[int64]string // messageID -> version

	// Call tracking
	GetUnprocessedMessagesCalls int
	LastGetUnprocessedLimit     int // Track last limit used
	GetProcessingStatsCalls     int
	SaveSentimentResultsCalls   int
	SaveToxicityResultsCalls    int
	SaveHumorResultsCalls       int
	SaveQuestionResultsCalls    int
	SaveNERResultsCalls         int
	SaveTopicsCalls             int
	SaveMessageTopicsCalls      int
	MarkMessagesProcessedCalls  int
	ProcessedMessageIDs         []int64  // Track processed message IDs in order
	ProcessedChatIDs            []int64  // Track processed chat IDs in order
	ProcessedVersion            string   // Track last processor version

	// Error injection
	GetUnprocessedMessagesError error
	GetProcessingStatsError     error
	SaveSentimentResultsError   error
	SaveToxicityResultsError    error
	SaveHumorResultsError       error
	SaveQuestionResultsError    error
	SaveNERResultsError         error
	SaveTopicsError             error
	SaveMessageTopicsError      error
	MarkMessagesProcessedError  error
}

// NewMockMLRepository creates a new MockMLRepository with initialized storage.
func NewMockMLRepository() *MockMLRepository {
	return &MockMLRepository{
		UnprocessedMessages: []repository.UnprocessedMessage{},
		ProcessingStats:     make(map[string]int64),
		SentimentResults:    []repository.SentimentResult{},
		ToxicityResults:     []repository.ToxicityResult{},
		HumorResults:        []repository.HumorResult{},
		QuestionResults:     []repository.QuestionResult{},
		NERResults:          []repository.NERResult{},
		Topics:              make(map[int64]map[int][]string),
		MessageTopics:       []repository.MessageTopicResult{},
		ProcessedMessages:   make(map[int64]string),
	}
}

// Reset clears all storage and resets call counters and errors.
func (m *MockMLRepository) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.UnprocessedMessages = []repository.UnprocessedMessage{}
	m.ProcessingStats = make(map[string]int64)
	m.SentimentResults = []repository.SentimentResult{}
	m.ToxicityResults = []repository.ToxicityResult{}
	m.HumorResults = []repository.HumorResult{}
	m.QuestionResults = []repository.QuestionResult{}
	m.NERResults = []repository.NERResult{}
	m.Topics = make(map[int64]map[int][]string)
	m.MessageTopics = []repository.MessageTopicResult{}
	m.ProcessedMessages = make(map[int64]string)

	m.GetUnprocessedMessagesCalls = 0
	m.LastGetUnprocessedLimit = 0
	m.GetProcessingStatsCalls = 0
	m.SaveSentimentResultsCalls = 0
	m.SaveToxicityResultsCalls = 0
	m.SaveHumorResultsCalls = 0
	m.SaveQuestionResultsCalls = 0
	m.SaveNERResultsCalls = 0
	m.SaveTopicsCalls = 0
	m.SaveMessageTopicsCalls = 0
	m.MarkMessagesProcessedCalls = 0
	m.ProcessedMessageIDs = []int64{}
	m.ProcessedChatIDs = []int64{}
	m.ProcessedVersion = ""

	m.GetUnprocessedMessagesError = nil
	m.GetProcessingStatsError = nil
	m.SaveSentimentResultsError = nil
	m.SaveToxicityResultsError = nil
	m.SaveHumorResultsError = nil
	m.SaveQuestionResultsError = nil
	m.SaveNERResultsError = nil
	m.SaveTopicsError = nil
	m.SaveMessageTopicsError = nil
	m.MarkMessagesProcessedError = nil
}

// =============================================================================
// Helper methods for populating test data
// =============================================================================

// AddUnprocessedMessage adds an unprocessed message to the mock storage.
func (m *MockMLRepository) AddUnprocessedMessage(msg repository.UnprocessedMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UnprocessedMessages = append(m.UnprocessedMessages, msg)
}

// AddUnprocessedMessages adds multiple unprocessed messages to the mock storage.
func (m *MockMLRepository) AddUnprocessedMessages(msgs []repository.UnprocessedMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UnprocessedMessages = append(m.UnprocessedMessages, msgs...)
}

// SetProcessingStats sets the processing statistics.
func (m *MockMLRepository) SetProcessingStats(stats map[string]int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ProcessingStats = stats
}

// SetGetUnprocessedError sets the error for GetUnprocessedMessages.
func (m *MockMLRepository) SetGetUnprocessedError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetUnprocessedMessagesError = err
}

// SetSaveSentimentError sets the error for SaveSentimentResults.
func (m *MockMLRepository) SetSaveSentimentError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SaveSentimentResultsError = err
}

// SetGetStatsError sets the error for GetProcessingStats.
func (m *MockMLRepository) SetGetStatsError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetProcessingStatsError = err
}

// =============================================================================
// Deep copy functions
// =============================================================================

// copyUnprocessedMessage creates a deep copy of an UnprocessedMessage.
func copyUnprocessedMessage(msg *repository.UnprocessedMessage) repository.UnprocessedMessage {
	cp := *msg
	if msg.UserID != nil {
		userID := *msg.UserID
		cp.UserID = &userID
	}
	return cp
}

// copySentimentResult creates a deep copy of a SentimentResult.
func copySentimentResult(r *repository.SentimentResult) repository.SentimentResult {
	return *r
}

// copyToxicityResult creates a deep copy of a ToxicityResult.
func copyToxicityResult(r *repository.ToxicityResult) repository.ToxicityResult {
	return *r
}

// copyHumorResult creates a deep copy of a HumorResult.
func copyHumorResult(r *repository.HumorResult) repository.HumorResult {
	return *r
}

// copyQuestionResult creates a deep copy of a QuestionResult.
func copyQuestionResult(r *repository.QuestionResult) repository.QuestionResult {
	return *r
}

// copyNERResult creates a deep copy of a NERResult.
func copyNERResult(r *repository.NERResult) repository.NERResult {
	cp := *r
	if r.StartPos != nil {
		startPos := *r.StartPos
		cp.StartPos = &startPos
	}
	if r.EndPos != nil {
		endPos := *r.EndPos
		cp.EndPos = &endPos
	}
	return cp
}

// copyMessageTopicResult creates a deep copy of a MessageTopicResult.
func copyMessageTopicResult(r *repository.MessageTopicResult) repository.MessageTopicResult {
	return *r
}

// =============================================================================
// Interface implementation
// =============================================================================

// GetUnprocessedMessages retrieves messages that haven't been processed by ML.
func (m *MockMLRepository) GetUnprocessedMessages(ctx context.Context, limit int) ([]repository.UnprocessedMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetUnprocessedMessagesCalls++
	m.LastGetUnprocessedLimit = limit // Track the limit
	if m.GetUnprocessedMessagesError != nil {
		return nil, m.GetUnprocessedMessagesError
	}

	// Filter out already processed messages
	var unprocessed []repository.UnprocessedMessage
	for _, msg := range m.UnprocessedMessages {
		if _, processed := m.ProcessedMessages[msg.ID]; !processed {
			unprocessed = append(unprocessed, copyUnprocessedMessage(&msg))
		}
	}

	// Apply limit
	if limit > 0 && len(unprocessed) > limit {
		unprocessed = unprocessed[:limit]
	}

	return unprocessed, nil
}

// GetProcessingStats returns statistics about ML processing.
func (m *MockMLRepository) GetProcessingStats(ctx context.Context) (map[string]int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetProcessingStatsCalls++
	if m.GetProcessingStatsError != nil {
		return nil, m.GetProcessingStatsError
	}

	// Return a copy of the stats
	stats := make(map[string]int64)
	for k, v := range m.ProcessingStats {
		stats[k] = v
	}
	return stats, nil
}

// SaveSentimentResults saves sentiment analysis results in batch.
func (m *MockMLRepository) SaveSentimentResults(ctx context.Context, results []repository.SentimentResult, dbtx ...repository.DBTX) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SaveSentimentResultsCalls++
	if m.SaveSentimentResultsError != nil {
		return m.SaveSentimentResultsError
	}

	for _, r := range results {
		m.SentimentResults = append(m.SentimentResults, copySentimentResult(&r))
	}
	return nil
}

// SaveToxicityResults saves toxicity detection results in batch.
func (m *MockMLRepository) SaveToxicityResults(ctx context.Context, results []repository.ToxicityResult, dbtx ...repository.DBTX) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SaveToxicityResultsCalls++
	if m.SaveToxicityResultsError != nil {
		return m.SaveToxicityResultsError
	}

	for _, r := range results {
		m.ToxicityResults = append(m.ToxicityResults, copyToxicityResult(&r))
	}
	return nil
}

// SaveHumorResults saves humor detection results in batch.
func (m *MockMLRepository) SaveHumorResults(ctx context.Context, results []repository.HumorResult, dbtx ...repository.DBTX) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SaveHumorResultsCalls++
	if m.SaveHumorResultsError != nil {
		return m.SaveHumorResultsError
	}

	for _, r := range results {
		m.HumorResults = append(m.HumorResults, copyHumorResult(&r))
	}
	return nil
}

// SaveQuestionResults saves question detection results in batch.
func (m *MockMLRepository) SaveQuestionResults(ctx context.Context, results []repository.QuestionResult, dbtx ...repository.DBTX) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SaveQuestionResultsCalls++
	if m.SaveQuestionResultsError != nil {
		return m.SaveQuestionResultsError
	}

	for _, r := range results {
		m.QuestionResults = append(m.QuestionResults, copyQuestionResult(&r))
	}
	return nil
}

// SaveNERResults saves named entity recognition results in batch.
func (m *MockMLRepository) SaveNERResults(ctx context.Context, results []repository.NERResult, dbtx ...repository.DBTX) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SaveNERResultsCalls++
	if m.SaveNERResultsError != nil {
		return m.SaveNERResultsError
	}

	for _, r := range results {
		m.NERResults = append(m.NERResults, copyNERResult(&r))
	}
	return nil
}

// SaveTopics saves or updates topic clusters for a chat.
func (m *MockMLRepository) SaveTopics(ctx context.Context, chatID int64, topics map[int][]string, dbtx ...repository.DBTX) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SaveTopicsCalls++
	if m.SaveTopicsError != nil {
		return m.SaveTopicsError
	}

	if m.Topics[chatID] == nil {
		m.Topics[chatID] = make(map[int][]string)
	}

	for topicID, keywords := range topics {
		// Deep copy the keywords slice
		keywordsCopy := make([]string, len(keywords))
		copy(keywordsCopy, keywords)
		m.Topics[chatID][topicID] = keywordsCopy
	}
	return nil
}

// SaveMessageTopics saves message-to-topic assignments in batch.
func (m *MockMLRepository) SaveMessageTopics(ctx context.Context, results []repository.MessageTopicResult, dbtx ...repository.DBTX) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SaveMessageTopicsCalls++
	if m.SaveMessageTopicsError != nil {
		return m.SaveMessageTopicsError
	}

	for _, r := range results {
		m.MessageTopics = append(m.MessageTopics, copyMessageTopicResult(&r))
	}
	return nil
}

// MarkMessagesProcessed marks messages as processed by ML.
func (m *MockMLRepository) MarkMessagesProcessed(ctx context.Context, messageIDs []int64, chatIDs []int64, version string, dbtx ...repository.DBTX) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.MarkMessagesProcessedCalls++
	if m.MarkMessagesProcessedError != nil {
		return m.MarkMessagesProcessedError
	}

	// Track message IDs, chat IDs, and version
	m.ProcessedMessageIDs = append(m.ProcessedMessageIDs, messageIDs...)
	m.ProcessedChatIDs = append(m.ProcessedChatIDs, chatIDs...)
	m.ProcessedVersion = version

	for _, msgID := range messageIDs {
		m.ProcessedMessages[msgID] = version
	}
	return nil
}

// Ensure MockMLRepository implements the interface at compile time
var _ repository.MLRepositoryInterface = (*MockMLRepository)(nil)

// =============================================================================
// MockProfilePhotoRepository
// =============================================================================

// MockProfilePhotoRepository is a mock implementation of ProfilePhotoRepositoryInterface for testing.
type MockProfilePhotoRepository struct {
	mu sync.RWMutex

	// Stored data
	UserPhotos []models.DBUserProfilePhoto
	ChatPhotos []models.DBChatProfilePhoto
	AllUserIDs []int64
	AllChatIDs []int64

	// For GetUserPhotoBySize
	UserPhotoBySize *models.DBUserProfilePhoto

	// For GetChatPhotoBySize
	ChatPhotoBySize *models.DBChatProfilePhoto

	// Call counters
	ReplaceUserPhotosCalls int
	ReplaceChatPhotosCalls int
	GetUserPhotosCalls     int
	GetChatPhotosCalls     int
	GetAllUserIDsCalls     int
	GetAllChatIDsCalls     int
	GetUserPhotoBySizeCalls int
	GetChatPhotoBySizeCalls int

	// Errors
	ReplaceUserPhotosError  error
	ReplaceChatPhotosError  error
	GetUserPhotosError      error
	GetChatPhotosError      error
	GetAllUserIDsError      error
	GetAllChatIDsError      error
	GetUserPhotoError       error
	GetChatPhotoError       error
}

// NewMockProfilePhotoRepository creates a new MockProfilePhotoRepository.
func NewMockProfilePhotoRepository() *MockProfilePhotoRepository {
	return &MockProfilePhotoRepository{
		UserPhotos: make([]models.DBUserProfilePhoto, 0),
		ChatPhotos: make([]models.DBChatProfilePhoto, 0),
		AllUserIDs: make([]int64, 0),
		AllChatIDs: make([]int64, 0),
	}
}

// ReplaceUserPhotos replaces user photos.
func (m *MockProfilePhotoRepository) ReplaceUserPhotos(ctx context.Context, tx *sql.Tx, userID int64, photos []models.DBUserProfilePhoto) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ReplaceUserPhotosCalls++
	if m.ReplaceUserPhotosError != nil {
		return m.ReplaceUserPhotosError
	}

	// Remove existing photos for this user
	newPhotos := make([]models.DBUserProfilePhoto, 0)
	for _, photo := range m.UserPhotos {
		if photo.UserID != userID {
			newPhotos = append(newPhotos, photo)
		}
	}
	m.UserPhotos = newPhotos

	// Add new photos
	m.UserPhotos = append(m.UserPhotos, photos...)
	return nil
}

// ReplaceChatPhotos replaces chat photos.
func (m *MockProfilePhotoRepository) ReplaceChatPhotos(ctx context.Context, tx *sql.Tx, chatID int64, photos []models.DBChatProfilePhoto) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ReplaceChatPhotosCalls++
	if m.ReplaceChatPhotosError != nil {
		return m.ReplaceChatPhotosError
	}

	// Remove existing photos for this chat
	newPhotos := make([]models.DBChatProfilePhoto, 0)
	for _, photo := range m.ChatPhotos {
		if photo.ChatID != chatID {
			newPhotos = append(newPhotos, photo)
		}
	}
	m.ChatPhotos = newPhotos

	// Add new photos
	m.ChatPhotos = append(m.ChatPhotos, photos...)
	return nil
}

// GetUserPhotos retrieves user photos.
func (m *MockProfilePhotoRepository) GetUserPhotos(ctx context.Context, userID int64) ([]models.DBUserProfilePhoto, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetUserPhotosCalls++
	if m.GetUserPhotosError != nil {
		return nil, m.GetUserPhotosError
	}

	var photos []models.DBUserProfilePhoto
	for _, photo := range m.UserPhotos {
		if photo.UserID == userID {
			photos = append(photos, photo)
		}
	}
	return photos, nil
}

// GetChatPhotos retrieves chat photos.
func (m *MockProfilePhotoRepository) GetChatPhotos(ctx context.Context, chatID int64) ([]models.DBChatProfilePhoto, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetChatPhotosCalls++
	if m.GetChatPhotosError != nil {
		return nil, m.GetChatPhotosError
	}

	var photos []models.DBChatProfilePhoto
	for _, photo := range m.ChatPhotos {
		if photo.ChatID == chatID {
			photos = append(photos, photo)
		}
	}
	return photos, nil
}

// GetAllUserIDs returns all user IDs.
func (m *MockProfilePhotoRepository) GetAllUserIDs(ctx context.Context) ([]int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetAllUserIDsCalls++
	if m.GetAllUserIDsError != nil {
		return nil, m.GetAllUserIDsError
	}

	return m.AllUserIDs, nil
}

// GetAllChatIDs returns all chat IDs.
func (m *MockProfilePhotoRepository) GetAllChatIDs(ctx context.Context) ([]int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetAllChatIDsCalls++
	if m.GetAllChatIDsError != nil {
		return nil, m.GetAllChatIDsError
	}

	return m.AllChatIDs, nil
}

// GetUserPhotoBySize retrieves user photo by size.
func (m *MockProfilePhotoRepository) GetUserPhotoBySize(ctx context.Context, userID int64, size string) (*models.DBUserProfilePhoto, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetUserPhotoBySizeCalls++
	if m.GetUserPhotoError != nil {
		return nil, m.GetUserPhotoError
	}

	return m.UserPhotoBySize, nil
}

// GetChatPhotoBySize retrieves chat photo by size.
func (m *MockProfilePhotoRepository) GetChatPhotoBySize(ctx context.Context, chatID int64, size string) (*models.DBChatProfilePhoto, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetChatPhotoBySizeCalls++
	if m.GetChatPhotoError != nil {
		return nil, m.GetChatPhotoError
	}

	return m.ChatPhotoBySize, nil
}

// SetAllUserIDs sets the user IDs to return.
func (m *MockProfilePhotoRepository) SetAllUserIDs(ids []int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.AllUserIDs = ids
}

// SetAllChatIDs sets the chat IDs to return.
func (m *MockProfilePhotoRepository) SetAllChatIDs(ids []int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.AllChatIDs = ids
}

// SetUserPhotoBySize sets the user photo to return for GetUserPhotoBySize.
func (m *MockProfilePhotoRepository) SetUserPhotoBySize(photo *models.DBUserProfilePhoto) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UserPhotoBySize = photo
}

// SetChatPhotoBySize sets the chat photo to return for GetChatPhotoBySize.
func (m *MockProfilePhotoRepository) SetChatPhotoBySize(photo *models.DBChatProfilePhoto) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ChatPhotoBySize = photo
}

// SetReplaceUserPhotosError sets the error to return for ReplaceUserPhotos.
func (m *MockProfilePhotoRepository) SetReplaceUserPhotosError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ReplaceUserPhotosError = err
}

// SetGetUserPhotoError sets the error to return for GetUserPhotoBySize.
func (m *MockProfilePhotoRepository) SetGetUserPhotoError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetUserPhotoError = err
}

// Reset clears all stored data and resets call counters.
func (m *MockProfilePhotoRepository) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.UserPhotos = make([]models.DBUserProfilePhoto, 0)
	m.ChatPhotos = make([]models.DBChatProfilePhoto, 0)
	m.AllUserIDs = make([]int64, 0)
	m.AllChatIDs = make([]int64, 0)
	m.UserPhotoBySize = nil
	m.ChatPhotoBySize = nil
	m.ReplaceUserPhotosCalls = 0
	m.ReplaceChatPhotosCalls = 0
	m.GetUserPhotosCalls = 0
	m.GetChatPhotosCalls = 0
	m.GetAllUserIDsCalls = 0
	m.GetAllChatIDsCalls = 0
	m.GetUserPhotoBySizeCalls = 0
	m.GetChatPhotoBySizeCalls = 0
	m.ReplaceUserPhotosError = nil
	m.ReplaceChatPhotosError = nil
	m.GetUserPhotosError = nil
	m.GetChatPhotosError = nil
	m.GetAllUserIDsError = nil
	m.GetAllChatIDsError = nil
	m.GetUserPhotoError = nil
	m.GetChatPhotoError = nil
}

// Ensure MockProfilePhotoRepository implements the interface at compile time
var _ repository.ProfilePhotoRepositoryInterface = (*MockProfilePhotoRepository)(nil)

// =============================================================================
// MockMediaRepository
// =============================================================================

// MockMediaRepository is a mock implementation of MediaRepositoryInterface for testing.
type MockMediaRepository struct {
	mu sync.RWMutex

	// Stored data
	ObjectKeyByHash string

	// Call counters
	GetObjectKeyByHashCalls int

	// Errors
	GetObjectKeyByHashError error
}

// NewMockMediaRepository creates a new MockMediaRepository.
func NewMockMediaRepository() *MockMediaRepository {
	return &MockMediaRepository{}
}

// GetObjectKeyByHash retrieves object key by hash.
func (m *MockMediaRepository) GetObjectKeyByHash(ctx context.Context, tx *sql.Tx, hash string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetObjectKeyByHashCalls++
	if m.GetObjectKeyByHashError != nil {
		return "", m.GetObjectKeyByHashError
	}

	return m.ObjectKeyByHash, nil
}

// SetObjectKeyByHash sets the object key to return.
func (m *MockMediaRepository) SetObjectKeyByHash(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ObjectKeyByHash = key
}

// Reset clears all stored data and resets call counters.
func (m *MockMediaRepository) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ObjectKeyByHash = ""
	m.GetObjectKeyByHashCalls = 0
	m.GetObjectKeyByHashError = nil
}

// Stub implementations for MediaRepositoryInterface methods not used in ProfilePhotoService tests

func (m *MockMediaRepository) InsertMediaFile(ctx context.Context, tx *sql.Tx, messageID int64, mediaType, fileID, fileUniqueID, objectKey, fileHash string, fileSize *int64, mimeType, fileName string, duration, width, height *int, performer, title string) error {
	return nil
}

func (m *MockMediaRepository) InsertMediaFileReturningID(ctx context.Context, tx *sql.Tx, messageID int64, mediaType, fileID, fileUniqueID, objectKey, fileHash string, fileSize *int64, mimeType, fileName string, duration, width, height *int, performer, title string) (int64, error) {
	return 0, nil
}

func (m *MockMediaRepository) InsertPhoto(ctx context.Context, tx *sql.Tx, messageID int64, photo *models.PhotoSize, objectKey, fileHash string) error {
	return nil
}

func (m *MockMediaRepository) InsertLocation(ctx context.Context, tx *sql.Tx, messageID int64, location *models.Location) error {
	return nil
}

func (m *MockMediaRepository) InsertLocationReturningID(ctx context.Context, tx *sql.Tx, messageID int64, location *models.Location) (int64, error) {
	return 0, nil
}

func (m *MockMediaRepository) InsertSticker(ctx context.Context, tx *sql.Tx, messageID, mediaFileID int64, sticker *models.Sticker) error {
	return nil
}

func (m *MockMediaRepository) InsertGame(ctx context.Context, tx *sql.Tx, messageID int64, game *models.Game) (int64, error) {
	return 0, nil
}

func (m *MockMediaRepository) InsertGamePhoto(ctx context.Context, tx *sql.Tx, gameID int64, photo *models.PhotoSize, objectKey, fileHash string) error {
	return nil
}

func (m *MockMediaRepository) InsertPoll(ctx context.Context, tx *sql.Tx, messageID int64, poll *models.Poll) (int64, error) {
	return 0, nil
}

func (m *MockMediaRepository) InsertPollOption(ctx context.Context, tx *sql.Tx, pollID int64, index int, option *models.PollOption) error {
	return nil
}

func (m *MockMediaRepository) InsertContact(ctx context.Context, tx *sql.Tx, messageID int64, contact *models.Contact) error {
	return nil
}

func (m *MockMediaRepository) InsertVenue(ctx context.Context, tx *sql.Tx, messageID, locationID int64, venue *models.Venue) error {
	return nil
}

func (m *MockMediaRepository) InsertDice(ctx context.Context, tx *sql.Tx, messageID int64, dice *models.Dice) error {
	return nil
}

// Ensure MockMediaRepository implements the interface at compile time
var _ repository.MediaRepositoryInterface = (*MockMediaRepository)(nil)

// Test Helper Methods for MockGameRepository

// AddMatch is a helper method to add a match directly to storage for testing.
// This is used to bypass validation logic in CreateMatch.
func (m *MockGameRepository) AddMatch(match *repository.Match) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Matches[match.ID] = match
	if m.Participants[match.ID] == nil {
		m.Participants[match.ID] = make(map[int64]*repository.Participant)
	}
	if m.ParticipantsWithUser[match.ID] == nil {
		m.ParticipantsWithUser[match.ID] = make(map[int64]*repository.ParticipantWithUser)
	}
}

// AddParticipantWithUser is a helper method to add a participant with user info directly to storage.
func (m *MockGameRepository) AddParticipantWithUser(p *repository.ParticipantWithUser) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ParticipantsWithUser[p.MatchID] == nil {
		m.ParticipantsWithUser[p.MatchID] = make(map[int64]*repository.ParticipantWithUser)
	}
	m.ParticipantsWithUser[p.MatchID][p.UserID] = p

	// Also add to Participants map
	if m.Participants[p.MatchID] == nil {
		m.Participants[p.MatchID] = make(map[int64]*repository.Participant)
	}
	m.Participants[p.MatchID][p.UserID] = &p.Participant
}

// GetParticipantsByMatch returns all participants for a match (helper for tests).
func (m *MockGameRepository) GetParticipantsByMatch(matchID string) []*repository.Participant {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var participants []*repository.Participant
	if matchParticipants, ok := m.Participants[matchID]; ok {
		for _, p := range matchParticipants {
			participants = append(participants, p)
		}
	}
	return participants
}
