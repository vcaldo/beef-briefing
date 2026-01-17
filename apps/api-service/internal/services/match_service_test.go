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

func newTestMatchServiceForMatchTests(mockRepo *testutil.MockGameRepository, mockDealer *testutil.MockDealer) *MatchService {
	return NewMatchService(nil, mockRepo, mockDealer, nil)
}

// Test fixtures specific to match_service tests

func newMatchForTest(id string, chatID int64, status repository.MatchStatus, creatorUserID int64) *repository.Match {
	return &repository.Match{
		ID:            id,
		ChatID:        chatID,
		MatchType:     repository.MatchTypeRegular,
		Status:        status,
		CreatorUserID: &creatorUserID,
		CreatedAt:     time.Now(),
	}
}

func newParticipantForMatchTest(matchID string, userID int64) *repository.Participant {
	return &repository.Participant{
		MatchID:        matchID,
		UserID:         userID,
		Status:         repository.ParticipantStatusJoined,
		CoinsRemaining: 10,
		TeamOrder:      []int64{},
		JoinedAt:       time.Now(),
	}
}

func newParticipantWithUserForMatchTest(matchID string, userID int64, firstName, username string) *repository.ParticipantWithUser {
	return &repository.ParticipantWithUser{
		Participant: repository.Participant{
			MatchID:        matchID,
			UserID:         userID,
			Status:         repository.ParticipantStatusJoined,
			CoinsRemaining: 10,
			TeamOrder:      []int64{},
			JoinedAt:       time.Now(),
		},
		FirstName: firstName,
		Username:  username,
	}
}

// CreateMatch Tests

// TestCreateMatch_HappyPath verifies creating a match with no active matches.
func TestCreateMatch_HappyPath(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestMatchServiceForMatchTests(mockRepo, mockDealer)

	chatID := int64(-100123)
	userID := int64(1)

	// Setup: No active matches, sufficient cards
	mockDealer.SetCardCount(15)

	// Execute
	result, err := svc.CreateMatch(ctx, chatID, userID)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Match == nil {
		t.Fatal("expected match, got nil")
	}
	if result.Match.ChatID != chatID {
		t.Errorf("expected chatID %d, got %d", chatID, result.Match.ChatID)
	}
	if result.Match.Status != repository.MatchStatusOpen {
		t.Errorf("expected status Open, got %s", result.Match.Status)
	}
	if result.Match.MatchType != repository.MatchTypeRegular {
		t.Errorf("expected match type Regular, got %s", result.Match.MatchType)
	}

	// Verify creator auto-joined
	if len(result.Participants) != 1 {
		t.Fatalf("expected 1 participant, got %d", len(result.Participants))
	}
	if result.Participants[0].UserID != userID {
		t.Errorf("expected participant userID %d, got %d", userID, result.Participants[0].UserID)
	}

	// Verify card count returned
	if result.CardCount != 15 {
		t.Errorf("expected card count 15, got %d", result.CardCount)
	}
}

// TestCreateMatch_ActiveMatchExistsForMatch verifies error when active match already exists.
func TestCreateMatch_ActiveMatchExistsForMatch(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestMatchServiceForMatchTests(mockRepo, mockDealer)

	chatID := int64(-100123)
	userID := int64(1)

	// Setup: Active match already exists
	activeMatch := newMatchForTest("match-1", chatID, repository.MatchStatusOpen, 1)
	mockRepo.AddMatch(activeMatch)
	mockDealer.SetCardCount(15)

	// Execute
	result, err := svc.CreateMatch(ctx, chatID, userID)

	// Verify
	if err != apperror.ErrActiveMatchExists {
		t.Fatalf("expected ErrActiveMatchExists, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

// TestCreateMatch_NotEnoughCardsForMatch verifies error when insufficient cards.
func TestCreateMatch_NotEnoughCardsForMatch(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestMatchServiceForMatchTests(mockRepo, mockDealer)

	chatID := int64(-100123)
	userID := int64(1)

	// Setup: Only 5 cards available (minimum is 10)
	mockDealer.SetCardCount(5)

	// Execute
	result, err := svc.CreateMatch(ctx, chatID, userID)

	// Verify
	if err != apperror.ErrNotEnoughCards {
		t.Fatalf("expected ErrNotEnoughCards, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

// GetMatch Tests

// TestGetMatch_ExistingMatch verifies retrieving an existing match.
func TestGetMatch_ExistingMatch(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestMatchServiceForMatchTests(mockRepo, mockDealer)

	matchID := "match-1"
	chatID := int64(-100123)
	userID := int64(1)

	// Setup: Create match with participant
	match := newMatchForTest(matchID, chatID, repository.MatchStatusOpen, userID)
	mockRepo.AddMatch(match)
	participant := newParticipantWithUserForMatchTest(matchID, userID, "User", "user1")
	mockRepo.AddParticipantWithUser(participant)
	mockDealer.SetCardCount(15)

	// Execute
	result, err := svc.GetMatch(ctx, matchID)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Match.ID != matchID {
		t.Errorf("expected match ID %s, got %s", matchID, result.Match.ID)
	}
	if len(result.Participants) != 1 {
		t.Fatalf("expected 1 participant, got %d", len(result.Participants))
	}
	if result.Participants[0].UserID != userID {
		t.Errorf("expected participant userID %d, got %d", userID, result.Participants[0].UserID)
	}
}

// TestGetMatch_NonExistentMatch verifies error for non-existent match.
func TestGetMatch_NonExistentMatch(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestMatchServiceForMatchTests(mockRepo, mockDealer)

	matchID := "nonexistent"

	// Execute
	result, err := svc.GetMatch(ctx, matchID)

	// Verify
	if err != apperror.ErrMatchNotFound {
		t.Fatalf("expected ErrMatchNotFound, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

// GetActiveMatches Tests

// TestGetActiveMatches_FilterByStatus verifies filtering active matches.
func TestGetActiveMatches_FilterByStatus(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestMatchServiceForMatchTests(mockRepo, mockDealer)

	chatID := int64(-100123)

	// Setup: Create matches with different statuses
	openMatch := newMatchForTest("match-1", chatID, repository.MatchStatusOpen, 1)
	shopMatch := newMatchForTest("match-2", chatID, repository.MatchStatusShopPhase, 1)
	completedMatch := newMatchForTest("match-3", chatID, repository.MatchStatusCompleted, 1)

	mockRepo.AddMatch(openMatch)
	mockRepo.AddMatch(shopMatch)
	mockRepo.AddMatch(completedMatch)

	// Add participants to each match
	mockRepo.AddParticipantWithUser(newParticipantWithUserForMatchTest("match-1", 1, "User1", "user1"))
	mockRepo.AddParticipantWithUser(newParticipantWithUserForMatchTest("match-2", 2, "User2", "user2"))
	mockRepo.AddParticipantWithUser(newParticipantWithUserForMatchTest("match-3", 3, "User3", "user3"))

	mockDealer.SetCardCount(15)

	// Execute
	results, err := svc.GetActiveMatches(ctx, chatID)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// Only Open and ShopPhase should be returned (not Complete)
	if len(results) != 2 {
		t.Fatalf("expected 2 active matches, got %d", len(results))
	}

	// Verify correct matches returned
	matchIDs := make(map[string]bool)
	for _, r := range results {
		matchIDs[r.Match.ID] = true
	}
	if !matchIDs["match-1"] || !matchIDs["match-2"] {
		t.Error("expected match-1 and match-2 to be returned")
	}
	if matchIDs["match-3"] {
		t.Error("expected match-3 (completed) not to be returned")
	}
}

// TestGetActiveMatches_EmptyResult verifies handling of no active matches.
func TestGetActiveMatches_EmptyResult(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestMatchServiceForMatchTests(mockRepo, mockDealer)

	chatID := int64(-100123)
	mockDealer.SetCardCount(15)

	// Execute (no matches added)
	results, err := svc.GetActiveMatches(ctx, chatID)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 active matches, got %d", len(results))
	}
}

// JoinMatch Tests

// TestJoinMatch_AddParticipant verifies joining an open match.
func TestJoinMatch_AddParticipant(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestMatchServiceForMatchTests(mockRepo, mockDealer)

	matchID := "match-1"
	chatID := int64(-100123)
	creatorID := int64(1)
	joinerID := int64(2)

	// Setup: Create match with creator
	match := newMatchForTest(matchID, chatID, repository.MatchStatusOpen, creatorID)
	mockRepo.AddMatch(match)
	mockRepo.AddParticipantWithUser(newParticipantWithUserForMatchTest(matchID, creatorID, "Creator", "creator"))
	mockDealer.SetCardCount(15)

	// Execute
	result, err := svc.JoinMatch(ctx, matchID, joinerID)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if len(result.Participants) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(result.Participants))
	}

	// Verify both creator and joiner are participants
	userIDs := make(map[int64]bool)
	for _, p := range result.Participants {
		userIDs[p.UserID] = true
	}
	if !userIDs[creatorID] || !userIDs[joinerID] {
		t.Error("expected both creator and joiner to be participants")
	}
}

// TestJoinMatch_MatchNotOpenForJoin verifies error when match is not open.
func TestJoinMatch_MatchNotOpenForJoin(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestMatchServiceForMatchTests(mockRepo, mockDealer)

	matchID := "match-1"
	chatID := int64(-100123)
	userID := int64(2)

	// Setup: Match is in shop phase (not open for joining)
	match := newMatchForTest(matchID, chatID, repository.MatchStatusShopPhase, 1)
	mockRepo.AddMatch(match)

	// Execute
	result, err := svc.JoinMatch(ctx, matchID, userID)

	// Verify
	if err != apperror.ErrMatchNotOpen {
		t.Fatalf("expected ErrMatchNotOpen, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

// TestJoinMatch_AlreadyJoined verifies handling when user already joined.
func TestJoinMatch_AlreadyJoined(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestMatchServiceForMatchTests(mockRepo, mockDealer)

	matchID := "match-1"
	chatID := int64(-100123)
	userID := int64(1)

	// Setup: Match with user already as participant
	match := newMatchForTest(matchID, chatID, repository.MatchStatusOpen, userID)
	mockRepo.AddMatch(match)
	mockRepo.AddParticipantWithUser(newParticipantWithUserForMatchTest(matchID, userID, "User", "user1"))
	mockDealer.SetCardCount(15)

	// Execute (try to join again)
	result, err := svc.JoinMatch(ctx, matchID, userID)

	// Verify (should fail with already joined error)
	if err == nil {
		t.Fatal("expected error when trying to join twice, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result on error, got %+v", result)
	}
}

// LeaveMatch Tests

// TestLeaveMatch_RemoveParticipant verifies removing a participant.
func TestLeaveMatch_RemoveParticipant(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestMatchServiceForMatchTests(mockRepo, mockDealer)

	matchID := "match-1"
	chatID := int64(-100123)
	userID := int64(1)

	// Setup: Match with participant
	match := newMatchForTest(matchID, chatID, repository.MatchStatusOpen, userID)
	mockRepo.AddMatch(match)
	mockRepo.AddParticipantWithUser(newParticipantWithUserForMatchTest(matchID, userID, "User", "user1"))

	// Execute
	err := svc.LeaveMatch(ctx, matchID, userID)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify participant was removed
	participants := mockRepo.GetParticipantsByMatch(matchID)
	if len(participants) != 0 {
		t.Errorf("expected 0 participants after leaving, got %d", len(participants))
	}
}

// TestLeaveMatch_NotInMatchForLeave verifies error when user not in match.
func TestLeaveMatch_NotInMatchForLeave(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestMatchServiceForMatchTests(mockRepo, mockDealer)

	matchID := "match-1"
	chatID := int64(-100123)
	userID := int64(999) // User not in match

	// Setup: Match with different participant
	match := newMatchForTest(matchID, chatID, repository.MatchStatusOpen, userID)
	mockRepo.AddMatch(match)
	mockRepo.AddParticipantWithUser(newParticipantWithUserForMatchTest(matchID, 1, "Other", "other"))

	// Execute
	err := svc.LeaveMatch(ctx, matchID, userID)

	// Verify (should handle gracefully, no error)
	if err != nil {
		t.Fatalf("expected no error when leaving match not in, got %v", err)
	}
}

// StartMatch Tests

// TestStartMatch_TransitionToShopPhaseForMatch verifies starting a match.
func TestStartMatch_TransitionToShopPhaseForMatch(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestMatchServiceForMatchTests(mockRepo, mockDealer)

	matchID := "match-1"
	chatID := int64(-100123)
	userID := int64(1)

	// Setup: Open match with 2 participants
	match := newMatchForTest(matchID, chatID, repository.MatchStatusOpen, userID)
	mockRepo.AddMatch(match)
	mockRepo.AddParticipantWithUser(newParticipantWithUserForMatchTest(matchID, userID, "User1", "user1"))
	mockRepo.AddParticipantWithUser(newParticipantWithUserForMatchTest(matchID, 2, "User2", "user2"))

	// Setup dealer with cards
	mockDealer.SetCardCount(15)

	// Execute
	result, err := svc.StartMatch(ctx, matchID, userID)

	// Verify
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.Match.Status != repository.MatchStatusShopPhase {
		t.Errorf("expected status ShopPhase, got %s", result.Match.Status)
	}

	// Verify format was set
	if result.Match.Format == nil {
		t.Fatal("expected format to be set, got nil")
	}

	// Verify participants count
	if len(result.Participants) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(result.Participants))
	}
}

// TestStartMatch_NotEnoughParticipantsForMatchStart verifies error with insufficient participants.
func TestStartMatch_NotEnoughParticipantsForMatchStart(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestMatchServiceForMatchTests(mockRepo, mockDealer)

	matchID := "match-1"
	chatID := int64(-100123)
	userID := int64(1)

	// Setup: Match with only 1 participant (need at least 2)
	match := newMatchForTest(matchID, chatID, repository.MatchStatusOpen, userID)
	mockRepo.AddMatch(match)
	mockRepo.AddParticipantWithUser(newParticipantWithUserForMatchTest(matchID, userID, "User1", "user1"))

	// Execute
	result, err := svc.StartMatch(ctx, matchID, userID)

	// Verify
	if err == nil {
		t.Fatal("expected error for insufficient participants, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

// TestStartMatch_RepositoryError verifies error propagation from repository.
func TestStartMatch_RepositoryErrorInMatch(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestMatchServiceForMatchTests(mockRepo, mockDealer)

	matchID := "match-1"
	chatID := int64(-100123)
	userID := int64(1)

	// Setup: Match with participants
	match := newMatchForTest(matchID, chatID, repository.MatchStatusOpen, userID)
	mockRepo.AddMatch(match)
	mockRepo.AddParticipantWithUser(newParticipantWithUserForMatchTest(matchID, userID, "User1", "user1"))
	mockRepo.AddParticipantWithUser(newParticipantWithUserForMatchTest(matchID, 2, "User2", "user2"))

	// Inject error in StartShopPhase
	testErr := errors.New("repository error")
	mockRepo.StartShopPhaseError = testErr

	// Execute
	result, err := svc.StartMatch(ctx, matchID, userID)

	// Verify
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, testErr) && err.Error() != "failed to start shop phase: repository error" {
		t.Errorf("expected repository error propagation, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result on error, got %+v", result)
	}
}

// Match Lifecycle Tests

// TestMatchLifecycle_StateTransitions verifies complete match state machine.
func TestMatchLifecycle_StateTransitionsForMatch(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	mockDealer := testutil.NewMockDealer()
	svc := newTestMatchServiceForMatchTests(mockRepo, mockDealer)

	chatID := int64(-100123)
	creatorID := int64(1)
	joinerID := int64(2)

	mockDealer.SetCardCount(15)

	// Step 1: Create match (Open status)
	createResult, err := svc.CreateMatch(ctx, chatID, creatorID)
	if err != nil {
		t.Fatalf("create match failed: %v", err)
	}
	if createResult.Match.Status != repository.MatchStatusOpen {
		t.Errorf("expected status Open, got %s", createResult.Match.Status)
	}
	matchID := createResult.Match.ID

	// Step 2: Second user joins
	_, err = svc.JoinMatch(ctx, matchID, joinerID)
	if err != nil {
		t.Fatalf("join match failed: %v", err)
	}

	// Step 3: Start match (transition to ShopPhase)
	startResult, err := svc.StartMatch(ctx, matchID, creatorID)
	if err != nil {
		t.Fatalf("start match failed: %v", err)
	}
	if startResult.Match.Status != repository.MatchStatusShopPhase {
		t.Errorf("expected status ShopPhase after start, got %s", startResult.Match.Status)
	}

	// Verify format was set
	if startResult.Match.Format == nil {
		t.Error("expected format to be set after starting match")
	}

	// Verify participants count
	if len(startResult.Participants) != 2 {
		t.Errorf("expected 2 participants, got %d", len(startResult.Participants))
	}
}
