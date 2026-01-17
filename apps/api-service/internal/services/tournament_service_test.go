package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"beef-briefing/apps/api-service/internal/apperror"
	"beef-briefing/apps/api-service/internal/repository"
	"beef-briefing/apps/api-service/internal/testutil"
)

// Test helpers

func newTestTournamentService(mockRepo *testutil.MockGameRepository, mockDealer *testutil.MockDealer) *TournamentService {
	return NewTournamentService(nil, mockRepo, mockDealer, nil)
}

// Test fixtures for tournament_service tests

func newOpenTournament(id int64, chatID int64, date string) *repository.RankedTournament {
	return &repository.RankedTournament{
		ID:               id,
		ChatID:           chatID,
		TournamentDate:   date,
		Status:           repository.TournamentStatusOpen,
		ParticipantCount: 0,
		CreatedAt:        time.Now(),
	}
}

func newScheduledTournament(id int64, chatID int64, date string) *repository.RankedTournament {
	return &repository.RankedTournament{
		ID:               id,
		ChatID:           chatID,
		TournamentDate:   date,
		Status:           repository.TournamentStatusScheduled,
		ParticipantCount: 0,
		CreatedAt:        time.Now(),
	}
}

func newInProgressTournament(id int64, chatID int64, date string, matchID string) *repository.RankedTournament {
	return &repository.RankedTournament{
		ID:               id,
		ChatID:           chatID,
		TournamentDate:   date,
		Status:           repository.TournamentStatusInProgress,
		MatchID:          &matchID,
		ParticipantCount: 0,
		CreatedAt:        time.Now(),
	}
}

// =============================================================================
// GetOrCreateTodayTournament Tests
// =============================================================================

// TestGetOrCreateTodayTournament_CreatesNewTournament verifies creating a new tournament.
func TestGetOrCreateTodayTournament_CreatesNewTournament(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	chatID := int64(-100123)
	date := "2025-01-17"
	mockDealer.SetCardCount(15)

	// Execute
	result, err := svc.GetOrCreateTodayTournament(ctx, chatID, date)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.ChatID != chatID {
		t.Errorf("expected chatID %d, got %d", chatID, result.ChatID)
	}
	if result.TournamentDate != date {
		t.Errorf("expected date %s, got %s", date, result.TournamentDate)
	}
	if result.Status != repository.TournamentStatusScheduled {
		t.Errorf("expected status Scheduled, got %s", result.Status)
	}
	if result.CardCount != 15 {
		t.Errorf("expected card count 15, got %d", result.CardCount)
	}
}

// TestGetOrCreateTodayTournament_GetsExistingTournament verifies returning existing tournament.
func TestGetOrCreateTodayTournament_GetsExistingTournament(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	chatID := int64(-100123)
	date := "2025-01-17"
	mockDealer.SetCardCount(15)

	// Setup existing tournament
	existingTournament := newOpenTournament(1, chatID, date)
	mockRepo.AddTournament(existingTournament)

	// Execute
	result, err := svc.GetOrCreateTodayTournament(ctx, chatID, date)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.ID != existingTournament.ID {
		t.Errorf("expected tournament ID %d, got %d", existingTournament.ID, result.ID)
	}
	// Should only have called GetOrCreateTournament once (no duplicate creation)
	if mockRepo.GetOrCreateTournamentCalls != 1 {
		t.Errorf("expected 1 GetOrCreateTournament call, got %d", mockRepo.GetOrCreateTournamentCalls)
	}
}

// TestGetOrCreateTodayTournament_RepositoryError verifies error propagation.
func TestGetOrCreateTodayTournament_RepositoryError(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	chatID := int64(-100123)
	date := "2025-01-17"

	// Inject error
	mockRepo.GetOrCreateTournamentError = errors.New("database error")

	// Execute
	result, err := svc.GetOrCreateTodayTournament(ctx, chatID, date)

	// Verify
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

// =============================================================================
// GetTodayTournament Tests
// =============================================================================

// TestGetTodayTournament_ExistingTournament verifies returning an existing tournament.
func TestGetTodayTournament_ExistingTournament(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	chatID := int64(-100123)
	date := "2025-01-17"
	mockDealer.SetCardCount(15)

	// Setup existing tournament
	existingTournament := newOpenTournament(1, chatID, date)
	mockRepo.AddTournament(existingTournament)

	// Execute
	result, err := svc.GetTodayTournament(ctx, chatID, date)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.ID != existingTournament.ID {
		t.Errorf("expected tournament ID %d, got %d", existingTournament.ID, result.ID)
	}
}

// TestGetTodayTournament_NoTournamentExists verifies nil when no tournament exists.
func TestGetTodayTournament_NoTournamentExists(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	chatID := int64(-100123)
	date := "2025-01-17"

	// Execute (no tournament added)
	result, err := svc.GetTodayTournament(ctx, chatID, date)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for non-existent tournament, got %+v", result)
	}
}

// =============================================================================
// GetTournamentByID Tests
// =============================================================================

// TestGetTournamentByID_ExistingTournament verifies retrieving an existing tournament.
func TestGetTournamentByID_ExistingTournament(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	mockDealer.SetCardCount(15)

	// Setup existing tournament
	tournament := newOpenTournament(1, -100123, "2025-01-17")
	mockRepo.AddTournament(tournament)

	// Execute
	result, err := svc.GetTournamentByID(ctx, 1)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.ID != tournament.ID {
		t.Errorf("expected tournament ID %d, got %d", tournament.ID, result.ID)
	}
}

// TestGetTournamentByID_NonExistentTournament verifies error for non-existent tournament.
func TestGetTournamentByID_NonExistentTournament(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	// Execute (no tournament added)
	result, err := svc.GetTournamentByID(ctx, 999)

	// Verify
	if err != apperror.ErrTournamentNotFound {
		t.Errorf("expected ErrTournamentNotFound, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

// =============================================================================
// JoinTournament Tests
// =============================================================================

// TestJoinTournament_HappyPath verifies joining an open tournament.
func TestJoinTournament_HappyPath(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	chatID := int64(-100123)
	tournamentID := int64(1)
	userID := int64(456)
	mockDealer.SetCardCount(15)

	// Setup open tournament
	tournament := newOpenTournament(tournamentID, chatID, "2025-01-17")
	mockRepo.AddTournament(tournament)

	// Execute
	result, err := svc.JoinTournament(ctx, tournamentID, userID)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if len(result.Participants) != 1 {
		t.Fatalf("expected 1 participant, got %d", len(result.Participants))
	}
	if result.Participants[0].UserID != userID {
		t.Errorf("expected participant userID %d, got %d", userID, result.Participants[0].UserID)
	}
}

// TestJoinTournament_TournamentNotFound verifies error for non-existent tournament.
func TestJoinTournament_TournamentNotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	// Execute (no tournament added)
	result, err := svc.JoinTournament(ctx, 999, 456)

	// Verify
	if err != apperror.ErrTournamentNotFound {
		t.Errorf("expected ErrTournamentNotFound, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

// TestJoinTournament_TournamentNotOpen verifies error when tournament is scheduled.
func TestJoinTournament_TournamentNotOpen(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	// Setup scheduled (not open) tournament
	tournament := newScheduledTournament(1, -100123, "2025-01-17")
	mockRepo.AddTournament(tournament)

	// Execute
	result, err := svc.JoinTournament(ctx, 1, 456)

	// Verify
	if err != apperror.ErrTournamentNotOpen {
		t.Errorf("expected ErrTournamentNotOpen, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

// TestJoinTournament_RegistrationClosed verifies error when tournament is in progress.
func TestJoinTournament_RegistrationClosed(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	// Setup in-progress tournament
	tournament := newInProgressTournament(1, -100123, "2025-01-17", "match-1")
	mockRepo.AddTournament(tournament)

	// Execute
	result, err := svc.JoinTournament(ctx, 1, 456)

	// Verify
	if err != apperror.ErrTournamentRegistrationClosed {
		t.Errorf("expected ErrTournamentRegistrationClosed, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

// TestJoinTournament_AlreadyRegistered verifies error when user already registered.
func TestJoinTournament_AlreadyRegistered(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	chatID := int64(-100123)
	tournamentID := int64(1)
	userID := int64(456)
	mockDealer.SetCardCount(15)

	// Setup open tournament
	tournament := newOpenTournament(tournamentID, chatID, "2025-01-17")
	mockRepo.AddTournament(tournament)

	// Add user as participant
	_ = mockRepo.AddTournamentParticipant(ctx, tournamentID, userID)

	// Execute (try to join again)
	result, err := svc.JoinTournament(ctx, tournamentID, userID)

	// Verify
	if err != apperror.ErrAlreadyRegistered {
		t.Errorf("expected ErrAlreadyRegistered, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

// =============================================================================
// LeaveTournament Tests
// =============================================================================

// TestLeaveTournament_HappyPath verifies leaving an open tournament.
func TestLeaveTournament_HappyPath(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	chatID := int64(-100123)
	tournamentID := int64(1)
	userID := int64(456)
	mockDealer.SetCardCount(15)

	// Setup open tournament with user as participant
	tournament := newOpenTournament(tournamentID, chatID, "2025-01-17")
	mockRepo.AddTournament(tournament)
	_ = mockRepo.AddTournamentParticipant(ctx, tournamentID, userID)

	// Execute
	result, err := svc.LeaveTournament(ctx, tournamentID, userID)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if len(result.Participants) != 0 {
		t.Errorf("expected 0 participants after leaving, got %d", len(result.Participants))
	}
}

// TestLeaveTournament_TournamentNotFound verifies error for non-existent tournament.
func TestLeaveTournament_TournamentNotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	// Execute (no tournament added)
	result, err := svc.LeaveTournament(ctx, 999, 456)

	// Verify
	if err != apperror.ErrTournamentNotFound {
		t.Errorf("expected ErrTournamentNotFound, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

// TestLeaveTournament_RegistrationClosed verifies error when tournament not open.
func TestLeaveTournament_RegistrationClosed(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	// Setup in-progress tournament
	tournament := newInProgressTournament(1, -100123, "2025-01-17", "match-1")
	mockRepo.AddTournament(tournament)

	// Execute
	result, err := svc.LeaveTournament(ctx, 1, 456)

	// Verify
	if err != apperror.ErrTournamentRegistrationClosed {
		t.Errorf("expected ErrTournamentRegistrationClosed, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

// TestLeaveTournament_NotRegistered verifies error when user not registered.
func TestLeaveTournament_NotRegistered(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	chatID := int64(-100123)
	tournamentID := int64(1)
	mockDealer.SetCardCount(15)

	// Setup open tournament (user not registered)
	tournament := newOpenTournament(tournamentID, chatID, "2025-01-17")
	mockRepo.AddTournament(tournament)

	// Execute
	result, err := svc.LeaveTournament(ctx, tournamentID, 456)

	// Verify
	if err != apperror.ErrNotRegistered {
		t.Errorf("expected ErrNotRegistered, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

// =============================================================================
// CloseAndStartTournament Tests
// =============================================================================

// TestCloseAndStartTournament_TwoParticipants_1v1Format verifies starting with 2 participants.
func TestCloseAndStartTournament_TwoParticipants_1v1Format(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	chatID := int64(-100123)
	tournamentID := int64(1)
	mockDealer.SetCardCount(15)

	// Setup open tournament with 2 participants
	tournament := newOpenTournament(tournamentID, chatID, "2025-01-17")
	mockRepo.AddTournament(tournament)
	_ = mockRepo.AddTournamentParticipant(ctx, tournamentID, 1)
	_ = mockRepo.AddTournamentParticipant(ctx, tournamentID, 2)

	// Execute
	result, err := svc.CloseAndStartTournament(ctx, tournamentID)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Skipped {
		t.Error("expected tournament not skipped")
	}
	if result.Format != repository.MatchFormat1v1 {
		t.Errorf("expected format 1v1, got %s", result.Format)
	}
	if result.ParticipantCount != 2 {
		t.Errorf("expected 2 participants, got %d", result.ParticipantCount)
	}
	if result.Match == nil {
		t.Error("expected match to be created")
	}
}

// TestCloseAndStartTournament_ThreeOrMoreParticipants_ArenaFormat verifies arena format.
func TestCloseAndStartTournament_ThreeOrMoreParticipants_ArenaFormat(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	chatID := int64(-100123)
	tournamentID := int64(1)
	mockDealer.SetCardCount(15)

	// Setup open tournament with 3 participants
	tournament := newOpenTournament(tournamentID, chatID, "2025-01-17")
	mockRepo.AddTournament(tournament)
	_ = mockRepo.AddTournamentParticipant(ctx, tournamentID, 1)
	_ = mockRepo.AddTournamentParticipant(ctx, tournamentID, 2)
	_ = mockRepo.AddTournamentParticipant(ctx, tournamentID, 3)

	// Execute
	result, err := svc.CloseAndStartTournament(ctx, tournamentID)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Skipped {
		t.Error("expected tournament not skipped")
	}
	if result.Format != repository.MatchFormatArena {
		t.Errorf("expected format Arena, got %s", result.Format)
	}
	if result.ParticipantCount != 3 {
		t.Errorf("expected 3 participants, got %d", result.ParticipantCount)
	}
}

// TestCloseAndStartTournament_NoParticipants_Skipped verifies skipping with no participants.
func TestCloseAndStartTournament_NoParticipants_Skipped(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	chatID := int64(-100123)
	tournamentID := int64(1)
	mockDealer.SetCardCount(15)

	// Setup open tournament with no participants
	tournament := newOpenTournament(tournamentID, chatID, "2025-01-17")
	mockRepo.AddTournament(tournament)

	// Execute
	result, err := svc.CloseAndStartTournament(ctx, tournamentID)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if !result.Skipped {
		t.Error("expected tournament to be skipped")
	}
	if result.Reason != "no participants" {
		t.Errorf("expected reason 'no participants', got '%s'", result.Reason)
	}
	if result.Tournament.Status != repository.TournamentStatusSkipped {
		t.Errorf("expected status Skipped, got %s", result.Tournament.Status)
	}
}

// TestCloseAndStartTournament_OneParticipant_Skipped verifies skipping with 1 participant.
func TestCloseAndStartTournament_OneParticipant_Skipped(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	chatID := int64(-100123)
	tournamentID := int64(1)
	mockDealer.SetCardCount(15)

	// Setup open tournament with 1 participant
	tournament := newOpenTournament(tournamentID, chatID, "2025-01-17")
	mockRepo.AddTournament(tournament)
	_ = mockRepo.AddTournamentParticipant(ctx, tournamentID, 1)

	// Execute
	result, err := svc.CloseAndStartTournament(ctx, tournamentID)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.Skipped {
		t.Error("expected tournament to be skipped")
	}
	if result.Reason != "only 1 participant" {
		t.Errorf("expected reason 'only 1 participant', got '%s'", result.Reason)
	}
}

// TestCloseAndStartTournament_TournamentNotFound verifies error for non-existent tournament.
func TestCloseAndStartTournament_TournamentNotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	// Execute (no tournament added)
	result, err := svc.CloseAndStartTournament(ctx, 999)

	// Verify
	if err != apperror.ErrTournamentNotFound {
		t.Errorf("expected ErrTournamentNotFound, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

// TestCloseAndStartTournament_TournamentNotOpen verifies error when tournament not open.
func TestCloseAndStartTournament_TournamentNotOpen(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	// Setup in-progress tournament
	tournament := newInProgressTournament(1, -100123, "2025-01-17", "match-1")
	mockRepo.AddTournament(tournament)

	// Execute
	result, err := svc.CloseAndStartTournament(ctx, 1)

	// Verify
	if err == nil {
		t.Fatal("expected error for non-open tournament, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

// =============================================================================
// SetTournamentAnnounced Tests
// =============================================================================

// TestSetTournamentAnnounced_HappyPath verifies marking tournament as announced.
func TestSetTournamentAnnounced_HappyPath(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	// Setup scheduled tournament
	tournament := newScheduledTournament(1, -100123, "2025-01-17")
	mockRepo.AddTournament(tournament)

	// Execute
	err := svc.SetTournamentAnnounced(ctx, 1, 12345)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if mockRepo.SetTournamentAnnouncedCalls != 1 {
		t.Errorf("expected 1 SetTournamentAnnounced call, got %d", mockRepo.SetTournamentAnnouncedCalls)
	}
}

// =============================================================================
// GetTournamentsNeedingAnnouncement Tests
// =============================================================================

// TestGetTournamentsNeedingAnnouncement_ReturnsScheduledTournaments verifies scheduled filter.
func TestGetTournamentsNeedingAnnouncement_ReturnsScheduledTournaments(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	// Setup tournaments with different statuses
	scheduled := newScheduledTournament(1, -100123, "2025-01-17")
	open := newOpenTournament(2, -100124, "2025-01-17")
	mockRepo.AddTournament(scheduled)
	mockRepo.AddTournament(open)

	// Execute
	result, err := svc.GetTournamentsNeedingAnnouncement(ctx, time.Now())

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 tournament needing announcement, got %d", len(result))
	}
	if result[0].TournamentID != scheduled.ID {
		t.Errorf("expected tournament ID %d, got %d", scheduled.ID, result[0].TournamentID)
	}
}

// =============================================================================
// GetTournamentsNeedingClose Tests
// =============================================================================

// TestGetTournamentsNeedingClose_ReturnsOpenTournaments verifies open filter.
func TestGetTournamentsNeedingClose_ReturnsOpenTournaments(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	// Setup tournaments with different statuses
	scheduled := newScheduledTournament(1, -100123, "2025-01-17")
	open := newOpenTournament(2, -100124, "2025-01-17")
	mockRepo.AddTournament(scheduled)
	mockRepo.AddTournament(open)

	// Execute
	result, err := svc.GetTournamentsNeedingClose(ctx, time.Now())

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 tournament needing close, got %d", len(result))
	}
	if result[0].TournamentID != open.ID {
		t.Errorf("expected tournament ID %d, got %d", open.ID, result[0].TournamentID)
	}
}

// =============================================================================
// CompleteTournament Tests
// =============================================================================

// TestCompleteTournament_HappyPath verifies completing a tournament with winner.
func TestCompleteTournament_HappyPath(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	// Setup in-progress tournament
	tournament := newInProgressTournament(1, -100123, "2025-01-17", "match-1")
	mockRepo.AddTournament(tournament)

	winnerID := int64(456)

	// Execute
	err := svc.CompleteTournament(ctx, 1, &winnerID)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if mockRepo.CompleteTournamentCalls != 1 {
		t.Errorf("expected 1 CompleteTournament call, got %d", mockRepo.CompleteTournamentCalls)
	}
}

// =============================================================================
// GetTournamentsWithPendingRounds Tests
// =============================================================================

// TestGetTournamentsWithPendingRounds_ReturnsInProgressTournaments verifies filter.
func TestGetTournamentsWithPendingRounds_ReturnsInProgressTournaments(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	// Setup tournaments with different statuses
	open := newOpenTournament(1, -100123, "2025-01-17")
	inProgress := newInProgressTournament(2, -100124, "2025-01-17", "match-1")
	mockRepo.AddTournament(open)
	mockRepo.AddTournament(inProgress)

	// Execute
	result, err := svc.GetTournamentsWithPendingRounds(ctx)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 tournament with pending rounds, got %d", len(result))
	}
	if result[0].ID != inProgress.ID {
		t.Errorf("expected tournament ID %d, got %d", inProgress.ID, result[0].ID)
	}
}

// =============================================================================
// Tournament Lifecycle Tests
// =============================================================================

// TestTournamentLifecycle_FullFlow verifies complete tournament lifecycle.
func TestTournamentLifecycle_FullFlow(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestTournamentService(mockRepo, mockDealer)

	chatID := int64(-100123)
	date := "2025-01-17"
	mockDealer.SetCardCount(15)

	// Step 1: Create tournament (scheduled)
	tourney, err := svc.GetOrCreateTodayTournament(ctx, chatID, date)
	if err != nil {
		t.Fatalf("create tournament failed: %v", err)
	}
	if tourney.Status != repository.TournamentStatusScheduled {
		t.Errorf("expected status Scheduled, got %s", tourney.Status)
	}
	tournamentID := tourney.ID

	// Step 2: Announce tournament (scheduled -> open)
	err = svc.SetTournamentAnnounced(ctx, tournamentID, 12345)
	if err != nil {
		t.Fatalf("announce tournament failed: %v", err)
	}

	// Verify status changed to open
	tourney, err = svc.GetTournamentByID(ctx, tournamentID)
	if err != nil {
		t.Fatalf("get tournament failed: %v", err)
	}
	if tourney.Status != repository.TournamentStatusOpen {
		t.Errorf("expected status Open after announcement, got %s", tourney.Status)
	}

	// Step 3: Users join
	_, err = svc.JoinTournament(ctx, tournamentID, 1)
	if err != nil {
		t.Fatalf("join tournament (user 1) failed: %v", err)
	}
	_, err = svc.JoinTournament(ctx, tournamentID, 2)
	if err != nil {
		t.Fatalf("join tournament (user 2) failed: %v", err)
	}

	// Step 4: Close and start tournament
	startResult, err := svc.CloseAndStartTournament(ctx, tournamentID)
	if err != nil {
		t.Fatalf("close and start tournament failed: %v", err)
	}
	if startResult.Skipped {
		t.Error("expected tournament not skipped")
	}
	if startResult.Format != repository.MatchFormat1v1 {
		t.Errorf("expected 1v1 format, got %s", startResult.Format)
	}

	// Verify status changed to in_progress
	tourney, err = svc.GetTournamentByID(ctx, tournamentID)
	if err != nil {
		t.Fatalf("get tournament failed: %v", err)
	}
	if tourney.Status != repository.TournamentStatusInProgress {
		t.Errorf("expected status InProgress, got %s", tourney.Status)
	}

	// Step 5: Complete tournament
	winnerID := int64(1)
	err = svc.CompleteTournament(ctx, tournamentID, &winnerID)
	if err != nil {
		t.Fatalf("complete tournament failed: %v", err)
	}

	// Verify final status
	tourney, err = svc.GetTournamentByID(ctx, tournamentID)
	if err != nil {
		t.Fatalf("get tournament failed: %v", err)
	}
	if tourney.Status != repository.TournamentStatusCompleted {
		t.Errorf("expected status Completed, got %s", tourney.Status)
	}
	if tourney.WinnerUserID == nil || *tourney.WinnerUserID != winnerID {
		t.Errorf("expected winner %d, got %v", winnerID, tourney.WinnerUserID)
	}
}
