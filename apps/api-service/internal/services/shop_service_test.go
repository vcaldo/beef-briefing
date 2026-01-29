package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"beef-briefing/apps/api-service/internal/apperror"
	"beef-briefing/apps/api-service/internal/game/battle"
	"beef-briefing/apps/api-service/internal/game/shop"
	"beef-briefing/apps/api-service/internal/repository"
	"beef-briefing/apps/api-service/internal/testutil"
)

// Test helpers

func newTestShopService(mockRepo *testutil.MockGameRepository) *ShopService {
	return &ShopService{
		db:       nil, // Not needed for tests without dealer dependency
		gameRepo: mockRepo,
		dealer:   nil, // Not needed for tests without dealer dependency
		nrApp:    nil, // Not needed for these tests
	}
}

// Test fixtures

func newTestMatch(id string, status repository.MatchStatus) *repository.Match {
	now := time.Now()
	deadline := now.Add(5 * time.Minute)
	return &repository.Match{
		ID:                 id,
		ChatID:             -100123,
		Status:             status,
		MatchType:          repository.MatchTypeRegular,
		ShopPhaseDeadline:  &deadline,
		ShopPhaseStartedAt: &now,
		CreatedAt:          now,
	}
}

func newTestParticipant(matchID string, userID int64, status repository.ParticipantStatus, coins int) *repository.Participant {
	now := time.Now()
	return &repository.Participant{
		MatchID:        matchID,
		UserID:         userID,
		Status:         status,
		CoinsRemaining: coins,
		TeamOrder:      []int64{},
		JoinedAt:       now,
	}
}

func newTestParticipantWithShop(matchID string, userID int64, cards []*battle.ShopCard, team []*battle.Card, coins int) *repository.Participant {
	p := newTestParticipant(matchID, userID, repository.ParticipantStatusJoined, coins)

	if cards != nil {
		cardsJSON, _ := json.Marshal(cards)
		p.ShopCards = (*json.RawMessage)(&cardsJSON)
	}

	if team != nil {
		teamJSON, _ := json.Marshal(team)
		p.Team = (*json.RawMessage)(&teamJSON)

		// Set team order to match positions
		order := make([]int64, len(team))
		for i := range team {
			order[i] = int64(i)
		}
		p.TeamOrder = order
	}

	return p
}

// TestValidateShopPhaseAccess_Success verifies that validation passes when all conditions are met.
func TestValidateShopPhaseAccess_Success(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	svc := newTestShopService(mockRepo)

	matchID := "match-1"
	userID := int64(1)

	match := newTestMatch(matchID, repository.MatchStatusShopPhase)
	participant := newTestParticipant(matchID, userID, repository.ParticipantStatusJoined, 10)

	mockRepo.Matches[matchID] = match
	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		userID: participant,
	}

	resultMatch, resultParticipant, err := svc.validateShopPhaseAccess(ctx, matchID, userID, true)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resultMatch == nil {
		t.Fatal("expected match, got nil")
	}
	if resultParticipant == nil {
		t.Fatal("expected participant, got nil")
	}
	if resultMatch.ID != matchID {
		t.Errorf("expected match ID %s, got %s", matchID, resultMatch.ID)
	}
	if resultParticipant.UserID != userID {
		t.Errorf("expected user ID %d, got %d", userID, resultParticipant.UserID)
	}
}

// TestValidateShopPhaseAccess_MatchNotFound verifies that validation fails when match doesn't exist.
func TestValidateShopPhaseAccess_MatchNotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	svc := newTestShopService(mockRepo)

	matchID := "nonexistent"
	userID := int64(1)

	_, _, err := svc.validateShopPhaseAccess(ctx, matchID, userID, true)

	if err != apperror.ErrMatchNotFound {
		t.Errorf("expected ErrMatchNotFound, got %v", err)
	}
}

// TestValidateShopPhaseAccess_WrongPhase verifies that validation fails when match is not in shop phase.
func TestValidateShopPhaseAccess_WrongPhase(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	svc := newTestShopService(mockRepo)

	matchID := "match-1"
	userID := int64(1)

	match := newTestMatch(matchID, repository.MatchStatusBattlePhase) // Wrong phase
	participant := newTestParticipant(matchID, userID, repository.ParticipantStatusJoined, 10)

	mockRepo.Matches[matchID] = match
	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		userID: participant,
	}

	_, _, err := svc.validateShopPhaseAccess(ctx, matchID, userID, true)

	if err != apperror.ErrMatchNotInShopPhase {
		t.Errorf("expected ErrMatchNotInShopPhase, got %v", err)
	}
}

// TestValidateShopPhaseAccess_DeadlinePassed verifies that validation fails when deadline has expired.
func TestValidateShopPhaseAccess_DeadlinePassed(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	svc := newTestShopService(mockRepo)

	matchID := "match-1"
	userID := int64(1)

	match := newTestMatch(matchID, repository.MatchStatusShopPhase)
	pastDeadline := time.Now().Add(-1 * time.Hour) // Deadline in the past
	match.ShopPhaseDeadline = &pastDeadline

	participant := newTestParticipant(matchID, userID, repository.ParticipantStatusJoined, 10)

	mockRepo.Matches[matchID] = match
	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		userID: participant,
	}

	_, _, err := svc.validateShopPhaseAccess(ctx, matchID, userID, true)

	if err != apperror.ErrShopPhaseExpired {
		t.Errorf("expected ErrShopPhaseExpired, got %v", err)
	}
}

// TestValidateShopPhaseAccess_NotParticipant verifies that validation fails when user is not a participant.
func TestValidateShopPhaseAccess_NotParticipant(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	svc := newTestShopService(mockRepo)

	matchID := "match-1"
	userID := int64(1)

	match := newTestMatch(matchID, repository.MatchStatusShopPhase)
	mockRepo.Matches[matchID] = match
	mockRepo.Participants[matchID] = map[int64]*repository.Participant{} // No participants

	_, _, err := svc.validateShopPhaseAccess(ctx, matchID, userID, true)

	if err != apperror.ErrNotParticipant {
		t.Errorf("expected ErrNotParticipant, got %v", err)
	}
}

// TestGetShop_InitializesState verifies that GetShop returns shop state with enhanced cards.
func TestGetShop_InitializesState(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	svc := newTestShopService(mockRepo)

	matchID := "match-1"
	userID := int64(1)

	match := newTestMatch(matchID, repository.MatchStatusShopPhase)

	// Create participant with shop cards
	cards := []*battle.ShopCard{
		{Index: 0, CardID: 1, UserID: 101, Name: "Alice", ATK: 10, HP: 30},
		{Index: 1, CardID: 2, UserID: 102, Name: "Bob", ATK: 12, HP: 28},
	}
	team := []*battle.Card{}
	participant := newTestParticipantWithShop(matchID, userID, cards, team, 10)

	mockRepo.Matches[matchID] = match
	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		userID: participant,
	}

	resp, err := svc.GetShop(ctx, matchID, userID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if resp.MatchID != matchID {
		t.Errorf("expected match ID %s, got %s", matchID, resp.MatchID)
	}
	if resp.Coins != 10 {
		t.Errorf("expected 10 coins, got %d", resp.Coins)
	}
	if len(resp.Cards) != 2 {
		t.Errorf("expected 2 cards, got %d", len(resp.Cards))
	}
	if len(resp.Team) != 0 {
		t.Errorf("expected 0 team cards, got %d", len(resp.Team))
	}
	if !resp.Affordability.CanBuy {
		t.Error("expected CanBuy to be true")
	}
	if !resp.Affordability.CanReroll {
		t.Error("expected CanReroll to be true (team is empty)")
	}
}

// TestShopBuyCard_Success verifies that buying a card works correctly.
func TestShopBuyCard_Success(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	svc := newTestShopService(mockRepo)

	matchID := "match-1"
	userID := int64(1)

	match := newTestMatch(matchID, repository.MatchStatusShopPhase)

	cards := []*battle.ShopCard{
		{Index: 0, CardID: 1, UserID: 101, Name: "Alice", ATK: 10, HP: 30, IsPurchased: false},
		{Index: 1, CardID: 2, UserID: 102, Name: "Bob", ATK: 12, HP: 28, IsPurchased: false},
	}
	team := []*battle.Card{}
	participant := newTestParticipantWithShop(matchID, userID, cards, team, 10)

	mockRepo.Matches[matchID] = match
	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		userID: participant,
	}

	resp, err := svc.BuyCard(ctx, matchID, userID, 0)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp == nil {
		t.Fatal("expected response, got nil")
	}

	// Verify coin deduction
	if resp.Coins != 7 { // 10 - 3 = 7
		t.Errorf("expected 7 coins, got %d", resp.Coins)
	}

	// Verify team size increased
	if len(resp.Team) != 1 {
		t.Errorf("expected 1 team card, got %d", len(resp.Team))
	}

	// Verify repository was called
	if mockRepo.UpdateParticipantShopCalls != 1 {
		t.Errorf("expected 1 UpdateParticipantShop call, got %d", mockRepo.UpdateParticipantShopCalls)
	}
}

// TestShopBuyCard_InsufficientCoins verifies that buying fails when player has insufficient coins.
func TestShopBuyCard_InsufficientCoins(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	svc := newTestShopService(mockRepo)

	matchID := "match-1"
	userID := int64(1)

	match := newTestMatch(matchID, repository.MatchStatusShopPhase)

	cards := []*battle.ShopCard{
		{Index: 0, CardID: 1, UserID: 101, Name: "Alice", ATK: 10, HP: 30, IsPurchased: false},
	}
	team := []*battle.Card{}
	participant := newTestParticipantWithShop(matchID, userID, cards, team, 2) // Only 2 coins, need 3

	mockRepo.Matches[matchID] = match
	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		userID: participant,
	}

	_, err := svc.BuyCard(ctx, matchID, userID, 0)

	if err != apperror.ErrNotEnoughCoins {
		t.Errorf("expected ErrNotEnoughCoins, got %v", err)
	}
}

// Note: Reroll tests (TestReroll_Success, TestReroll_AfterPurchase) already exist
// in arena_service_test.go as integration tests.

// TestUpgradeCard_ATK verifies that upgrading a card's ATK works correctly.
func TestUpgradeCard_ATK(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	svc := newTestShopService(mockRepo)

	matchID := "match-1"
	userID := int64(1)

	match := newTestMatch(matchID, repository.MatchStatusShopPhase)

	// Create team with 3 cards (team is full)
	cards := []*battle.ShopCard{
		{Index: 0, CardID: 1, UserID: 101, Name: "Alice", ATK: 10, HP: 30, IsPurchased: true},
		{Index: 1, CardID: 2, UserID: 102, Name: "Bob", ATK: 12, HP: 28, IsPurchased: true},
		{Index: 2, CardID: 3, UserID: 103, Name: "Charlie", ATK: 8, HP: 35, IsPurchased: true},
	}
	team := []*battle.Card{
		{CardID: 1, UserID: 101, Name: "Alice", ATK: 10, HP: 30, MaxHP: 30, Position: 0},
		{CardID: 2, UserID: 102, Name: "Bob", ATK: 12, HP: 28, MaxHP: 28, Position: 1},
		{CardID: 3, UserID: 103, Name: "Charlie", ATK: 8, HP: 35, MaxHP: 35, Position: 2},
	}
	// Team is full (3 cards), so only need 1 coin for upgrade
	participant := newTestParticipantWithShop(matchID, userID, cards, team, 1)

	mockRepo.Matches[matchID] = match
	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		userID: participant,
	}

	resp, err := svc.UpgradeCard(ctx, matchID, userID, 0, shop.UpgradeATK)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp == nil {
		t.Fatal("expected response, got nil")
	}

	// Verify coin deduction (1 - 1 = 0)
	if resp.Coins != 0 {
		t.Errorf("expected 0 coins, got %d", resp.Coins)
	}

	// Verify team card was upgraded
	if len(resp.Team) != 3 {
		t.Fatalf("expected 3 team cards, got %d", len(resp.Team))
	}

	// Check that ATK increased by 1 (10 + 1 = 11)
	if resp.Team[0].ATK != 11 {
		t.Errorf("expected ATK 11, got %d", resp.Team[0].ATK)
	}

	// Check that HP and MaxHP remain unchanged
	if resp.Team[0].HP != 30 {
		t.Errorf("expected HP 30, got %d", resp.Team[0].HP)
	}
	if resp.Team[0].MaxHP != 30 {
		t.Errorf("expected MaxHP 30, got %d", resp.Team[0].MaxHP)
	}

	// Verify repository was called
	if mockRepo.UpdateParticipantShopCalls != 1 {
		t.Errorf("expected 1 UpdateParticipantShop call, got %d", mockRepo.UpdateParticipantShopCalls)
	}
}

// TestUpgradeCard_HP verifies that upgrading a card's HP works correctly.
func TestUpgradeCard_HP(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	svc := newTestShopService(mockRepo)

	matchID := "match-1"
	userID := int64(1)

	match := newTestMatch(matchID, repository.MatchStatusShopPhase)

	// Create team with 3 cards (team is full)
	cards := []*battle.ShopCard{
		{Index: 0, CardID: 1, UserID: 101, Name: "Alice", ATK: 10, HP: 30, IsPurchased: true},
		{Index: 1, CardID: 2, UserID: 102, Name: "Bob", ATK: 12, HP: 28, IsPurchased: true},
		{Index: 2, CardID: 3, UserID: 103, Name: "Charlie", ATK: 8, HP: 35, IsPurchased: true},
	}
	team := []*battle.Card{
		{CardID: 1, UserID: 101, Name: "Alice", ATK: 10, HP: 30, MaxHP: 30, Position: 0},
		{CardID: 2, UserID: 102, Name: "Bob", ATK: 12, HP: 28, MaxHP: 28, Position: 1},
		{CardID: 3, UserID: 103, Name: "Charlie", ATK: 8, HP: 35, MaxHP: 35, Position: 2},
	}
	// Team is full (3 cards), so only need 1 coin for upgrade
	participant := newTestParticipantWithShop(matchID, userID, cards, team, 1)

	mockRepo.Matches[matchID] = match
	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		userID: participant,
	}

	resp, err := svc.UpgradeCard(ctx, matchID, userID, 0, shop.UpgradeHP)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp == nil {
		t.Fatal("expected response, got nil")
	}

	// Verify coin deduction (1 - 1 = 0)
	if resp.Coins != 0 {
		t.Errorf("expected 0 coins, got %d", resp.Coins)
	}

	// Verify team card was upgraded
	if len(resp.Team) != 3 {
		t.Fatalf("expected 3 team cards, got %d", len(resp.Team))
	}

	// Check that HP and MaxHP increased by 3 (30 + 3 = 33)
	if resp.Team[0].HP != 33 {
		t.Errorf("expected HP 33, got %d", resp.Team[0].HP)
	}
	if resp.Team[0].MaxHP != 33 {
		t.Errorf("expected MaxHP 33, got %d", resp.Team[0].MaxHP)
	}

	// Check that ATK remains unchanged
	if resp.Team[0].ATK != 10 {
		t.Errorf("expected ATK 10, got %d", resp.Team[0].ATK)
	}

	// Verify repository was called
	if mockRepo.UpdateParticipantShopCalls != 1 {
		t.Errorf("expected 1 UpdateParticipantShop call, got %d", mockRepo.UpdateParticipantShopCalls)
	}
}

// TestSubmitTeam_AllReady_TriggersBattle verifies that SubmitTeam marks team as ready.
// Note: This is a unit test for ShopService.SubmitTeam. Full integration tests that verify
// the battle trigger mechanism exist in arena_service_test.go (TestSubmitTeam_BothPlayers,
// TestExecuteBattle_1v1).
func TestSubmitTeam_AllReady_TriggersBattle(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	svc := newTestShopService(mockRepo)

	matchID := "match-1"
	userID := int64(1)

	match := newTestMatch(matchID, repository.MatchStatusShopPhase)

	// Create team with 3 cards (team is complete)
	cards := []*battle.ShopCard{
		{Index: 0, CardID: 1, UserID: 101, Name: "Alice", ATK: 10, HP: 30, IsPurchased: true},
		{Index: 1, CardID: 2, UserID: 102, Name: "Bob", ATK: 12, HP: 28, IsPurchased: true},
		{Index: 2, CardID: 3, UserID: 103, Name: "Charlie", ATK: 8, HP: 35, IsPurchased: true},
	}
	team := []*battle.Card{
		{CardID: 1, UserID: 101, Name: "Alice", ATK: 10, HP: 30, MaxHP: 30, Position: 0},
		{CardID: 2, UserID: 102, Name: "Bob", ATK: 12, HP: 28, MaxHP: 28, Position: 1},
		{CardID: 3, UserID: 103, Name: "Charlie", ATK: 8, HP: 35, MaxHP: 35, Position: 2},
	}
	participant := newTestParticipantWithShop(matchID, userID, cards, team, 1)

	mockRepo.Matches[matchID] = match
	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		userID: participant,
	}

	// Submit team
	err := svc.SubmitTeam(ctx, matchID, userID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify SubmitTeam was called in the repository
	if mockRepo.SubmitTeamCalls != 1 {
		t.Errorf("expected 1 SubmitTeam call, got %d", mockRepo.SubmitTeamCalls)
	}

	// Verify the team was submitted for the correct user
	submittedParticipant, err := mockRepo.GetParticipant(ctx, matchID, userID)
	if err != nil {
		t.Fatalf("GetParticipant failed: %v", err)
	}
	if submittedParticipant.Status != repository.ParticipantStatusReady {
		t.Errorf("expected participant status to be 'ready', got %s", submittedParticipant.Status)
	}
}

// TestReroll_DisabledAfterFirstReroll verifies that reroll fails after using the single reroll.
func TestReroll_DisabledAfterFirstReroll(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()

	// Dealer not needed for this validation test
	svc := &ShopService{
		db:       nil,
		gameRepo: mockRepo,
		dealer:   nil, // Not needed - test only validates hasRerolled check
		nrApp:    nil,
	}

	matchID := "match-1"
	userID := int64(1)

	match := newTestMatch(matchID, repository.MatchStatusShopPhase)

	// Create participant that has already used their reroll
	cards := []*battle.ShopCard{
		{Index: 0, CardID: 1, UserID: 101, Name: "Alice", ATK: 10, HP: 30, IsPurchased: false},
		{Index: 1, CardID: 2, UserID: 102, Name: "Bob", ATK: 12, HP: 28, IsPurchased: false},
	}
	team := []*battle.Card{} // Empty team (reroll doesn't depend on team now)
	participant := newTestParticipantWithShop(matchID, userID, cards, team, 10) // Enough coins
	participant.HasRerolled = true                                              // Already used reroll

	mockRepo.Matches[matchID] = match
	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		userID: participant,
	}

	// Attempt reroll (should fail because already rerolled)
	_, err := svc.Reroll(ctx, matchID, userID)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	expectedMsg := "reroll already used"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}

// TestReroll_InsufficientCoins verifies that reroll fails when player has insufficient coins.
func TestReroll_InsufficientCoins(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()

	// Dealer not needed for this validation test
	svc := &ShopService{
		db:       nil,
		gameRepo: mockRepo,
		dealer:   nil, // Not needed - test only validates coins check
		nrApp:    nil,
	}

	matchID := "match-1"
	userID := int64(1)

	match := newTestMatch(matchID, repository.MatchStatusShopPhase)

	// Create participant with no purchases but insufficient coins
	// Reroll costs 1, but also need 9 coins to complete team (3 cards * 3 coins)
	cards := []*battle.ShopCard{
		{Index: 0, CardID: 1, UserID: 101, Name: "Alice", ATK: 10, HP: 30, IsPurchased: false},
		{Index: 1, CardID: 2, UserID: 102, Name: "Bob", ATK: 12, HP: 28, IsPurchased: false},
	}
	team := []*battle.Card{} // Empty team
	participant := newTestParticipantWithShop(matchID, userID, cards, team, 5) // Only 5 coins (need 10 total)

	mockRepo.Matches[matchID] = match
	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		userID: participant,
	}

	// Attempt reroll (should fail due to insufficient coins)
	_, err := svc.Reroll(ctx, matchID, userID)

	if err != apperror.ErrNotEnoughCoins {
		t.Errorf("expected ErrNotEnoughCoins, got %v", err)
	}
}

// TestBuyCard_CardNotInShop verifies that buying an invalid card index fails.
func TestBuyCard_CardNotInShop(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	svc := newTestShopService(mockRepo)

	matchID := "match-1"
	userID := int64(1)

	match := newTestMatch(matchID, repository.MatchStatusShopPhase)

	cards := []*battle.ShopCard{
		{Index: 0, CardID: 1, UserID: 101, Name: "Alice", ATK: 10, HP: 30, IsPurchased: false},
		{Index: 1, CardID: 2, UserID: 102, Name: "Bob", ATK: 12, HP: 28, IsPurchased: false},
	}
	team := []*battle.Card{}
	participant := newTestParticipantWithShop(matchID, userID, cards, team, 10)

	mockRepo.Matches[matchID] = match
	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		userID: participant,
	}

	// Attempt to buy card at invalid index (out of bounds)
	_, err := svc.BuyCard(ctx, matchID, userID, 5) // Only 2 cards, index 5 is invalid

	if err != apperror.ErrInvalidCardIndex {
		t.Errorf("expected ErrInvalidCardIndex, got %v", err)
	}
}

// TestBuyCard_CardAlreadyPurchased verifies that buying the same card twice fails.
func TestBuyCard_CardAlreadyPurchased(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	svc := newTestShopService(mockRepo)

	matchID := "match-1"
	userID := int64(1)

	match := newTestMatch(matchID, repository.MatchStatusShopPhase)

	// Card at index 0 is already purchased
	cards := []*battle.ShopCard{
		{Index: 0, CardID: 1, UserID: 101, Name: "Alice", ATK: 10, HP: 30, IsPurchased: true},
		{Index: 1, CardID: 2, UserID: 102, Name: "Bob", ATK: 12, HP: 28, IsPurchased: false},
	}
	team := []*battle.Card{
		{CardID: 1, UserID: 101, Name: "Alice", ATK: 10, HP: 30, MaxHP: 30, Position: 0},
	}
	participant := newTestParticipantWithShop(matchID, userID, cards, team, 10)

	mockRepo.Matches[matchID] = match
	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		userID: participant,
	}

	// Attempt to buy already-purchased card
	_, err := svc.BuyCard(ctx, matchID, userID, 0)

	if err != apperror.ErrCardAlreadyPurchased {
		t.Errorf("expected ErrCardAlreadyPurchased, got %v", err)
	}
}

// Note: TestBuyCard_TeamFull already exists in arena_service_test.go

// TestUpgradeCard_InsufficientCoins verifies that upgrade fails with insufficient coins.
func TestUpgradeCard_InsufficientCoins(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	svc := newTestShopService(mockRepo)

	matchID := "match-1"
	userID := int64(1)

	match := newTestMatch(matchID, repository.MatchStatusShopPhase)

	// Create team with 1 card (need 6 more coins to complete: 2 cards * 3 coins)
	// Upgrade costs 1, so need 7 total, but only have 5
	cards := []*battle.ShopCard{
		{Index: 0, CardID: 1, UserID: 101, Name: "Alice", ATK: 10, HP: 30, IsPurchased: true},
	}
	team := []*battle.Card{
		{CardID: 1, UserID: 101, Name: "Alice", ATK: 10, HP: 30, MaxHP: 30, Position: 0},
	}
	participant := newTestParticipantWithShop(matchID, userID, cards, team, 5) // Only 5 coins

	mockRepo.Matches[matchID] = match
	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		userID: participant,
	}

	// Attempt upgrade (should fail - insufficient coins)
	_, err := svc.UpgradeCard(ctx, matchID, userID, 0, shop.UpgradeATK)

	if err != apperror.ErrNotEnoughCoins {
		t.Errorf("expected ErrNotEnoughCoins, got %v", err)
	}
}

// TestSubmitTeam_WrongSize verifies that submitting a team with wrong size fails.
func TestSubmitTeam_WrongSize(t *testing.T) {
	ctx := context.Background()
	mockRepo := testutil.NewMockGameRepository()
	svc := newTestShopService(mockRepo)

	matchID := "match-1"
	userID := int64(1)

	match := newTestMatch(matchID, repository.MatchStatusShopPhase)

	// Create team with only 2 cards (incomplete - needs 3)
	cards := []*battle.ShopCard{
		{Index: 0, CardID: 1, UserID: 101, Name: "Alice", ATK: 10, HP: 30, IsPurchased: true},
		{Index: 1, CardID: 2, UserID: 102, Name: "Bob", ATK: 12, HP: 28, IsPurchased: true},
	}
	team := []*battle.Card{
		{CardID: 1, UserID: 101, Name: "Alice", ATK: 10, HP: 30, MaxHP: 30, Position: 0},
		{CardID: 2, UserID: 102, Name: "Bob", ATK: 12, HP: 28, MaxHP: 28, Position: 1},
	}
	participant := newTestParticipantWithShop(matchID, userID, cards, team, 1)

	mockRepo.Matches[matchID] = match
	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		userID: participant,
	}

	// Attempt to submit incomplete team
	err := svc.SubmitTeam(ctx, matchID, userID)

	if err != shop.ErrTeamIncomplete {
		t.Errorf("expected ErrTeamIncomplete, got %v", err)
	}
}
