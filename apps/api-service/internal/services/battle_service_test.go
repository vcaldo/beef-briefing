package services

import (
	"context"
	"encoding/json"
	"testing"

	"beef-briefing/apps/api-service/internal/game/battle"
	"beef-briefing/apps/api-service/internal/repository"
	"beef-briefing/apps/api-service/internal/testutil"
)

// TestGetBattle_DamageSummary_PlayerA verifies that damage summary is calculated
// correctly when the requesting user is Player A.
func TestGetBattle_DamageSummary_PlayerA(t *testing.T) {
	mockRepo := testutil.NewMockGameRepository()

	svc := NewBattleService(nil, mockRepo, nil, nil)
	ctx := context.Background()

	matchID := "test-match-123"
	playerAID := int64(1001)
	playerBID := int64(1002)
	chatID := int64(-1003280306634)
	format := repository.MatchFormat1v1

	// Setup: Create a match with Player A and Player B
	match := &repository.Match{
		ID:     matchID,
		ChatID: chatID,
		Format: &format,
		Status: repository.MatchStatusCompleted,
	}
	mockRepo.Matches[matchID] = match

	// Add participants
	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		playerAID: {
			MatchID: matchID,
			UserID:  playerAID,
		},
		playerBID: {
			MatchID: matchID,
			UserID:  playerBID,
		},
	}

	// Mock GetMatchParticipants to return ParticipantWithUser
	mockRepo.ParticipantsWithUser[matchID] = map[int64]*repository.ParticipantWithUser{
		playerAID: {
			Participant: repository.Participant{
				MatchID: matchID,
				UserID:  playerAID,
			},
			FirstName: "PlayerA",
			Username:  "playerA",
		},
		playerBID: {
			Participant: repository.Participant{
				MatchID: matchID,
				UserID:  playerBID,
			},
			FirstName: "PlayerB",
			Username:  "playerB",
		},
	}

	// Create battle round with specific damage values
	// Player A dealt 50 damage, Player B dealt 30 damage
	teamACards := []*battle.Card{
		{CardID: 1, UserID: playerAID, Name: "PlayerA", ATK: 10, HP: 15},
		{CardID: 2, UserID: playerAID, Name: "PlayerA", ATK: 15, HP: 20},
		{CardID: 3, UserID: playerAID, Name: "PlayerA", ATK: 25, HP: 10},
	}
	teamBCards := []*battle.Card{
		{CardID: 4, UserID: playerBID, Name: "PlayerB", ATK: 10, HP: 15},
		{CardID: 5, UserID: playerBID, Name: "PlayerB", ATK: 10, HP: 20},
		{CardID: 6, UserID: playerBID, Name: "PlayerB", ATK: 10, HP: 10},
	}

	// Sample battle events
	events := []battle.BattleEvent{
		{Type: "attack", Message: "Card1 attacks Card4"},
		{Type: "attack", Message: "Card4 attacks Card1"},
	}

	teamAJSON, _ := json.Marshal(teamACards)
	teamBJSON, _ := json.Marshal(teamBCards)
	eventsJSON, _ := json.Marshal(events)

	round := &repository.MatchRound{
		ID:          1,
		MatchID:     matchID,
		RoundNumber: 1,
		PlayerAID:   playerAID,
		PlayerBID:   playerBID,
		PlayerATeam: teamAJSON,
		PlayerBTeam: teamBJSON,
		PlayerADmg:  50, // Player A dealt 50 damage
		PlayerBDmg:  30, // Player B dealt 30 damage
		WinnerID:    &playerAID,
		IsDraw:      false,
		TotalRounds: 5,
		BattleLog:   eventsJSON,
	}
	mockRepo.Rounds[matchID] = []*repository.MatchRound{round}

	// Act: Request battle as Player A
	battleResp, err := svc.GetBattle(ctx, matchID, playerAID)

	// Assert
	if err != nil {
		t.Fatalf("GetBattle failed: %v", err)
	}

	if battleResp == nil {
		t.Fatal("Expected non-nil BattleResponse")
	}

	// When user is Player A:
	// - DamageDealt should be Player A's damage (50)
	// - DamageTaken should be Player B's damage (30)
	if battleResp.DamageDealt != 50 {
		t.Errorf("Expected DamageDealt=50 for Player A, got %d", battleResp.DamageDealt)
	}

	if battleResp.DamageTaken != 30 {
		t.Errorf("Expected DamageTaken=30 for Player A, got %d", battleResp.DamageTaken)
	}

	// Verify absolute damage values are still present
	if battleResp.TeamADamage != 50 {
		t.Errorf("Expected TeamADamage=50, got %d", battleResp.TeamADamage)
	}

	if battleResp.TeamBDamage != 30 {
		t.Errorf("Expected TeamBDamage=30, got %d", battleResp.TeamBDamage)
	}
}

// TestGetBattle_DamageSummary_PlayerB verifies that damage summary is calculated
// correctly when the requesting user is Player B.
func TestGetBattle_DamageSummary_PlayerB(t *testing.T) {
	mockRepo := testutil.NewMockGameRepository()

	svc := NewBattleService(nil, mockRepo, nil, nil)
	ctx := context.Background()

	matchID := "test-match-456"
	playerAID := int64(2001)
	playerBID := int64(2002)
	chatID := int64(-1003280306634)
	format := repository.MatchFormat1v1

	// Setup: Create a match with Player A and Player B
	match := &repository.Match{
		ID:     matchID,
		ChatID: chatID,
		Format: &format,
		Status: repository.MatchStatusCompleted,
	}
	mockRepo.Matches[matchID] = match

	// Add participants
	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		playerAID: {
			MatchID: matchID,
			UserID:  playerAID,
		},
		playerBID: {
			MatchID: matchID,
			UserID:  playerBID,
		},
	}

	// Mock GetMatchParticipants to return ParticipantWithUser
	mockRepo.ParticipantsWithUser[matchID] = map[int64]*repository.ParticipantWithUser{
		playerAID: {
			Participant: repository.Participant{
				MatchID: matchID,
				UserID:  playerAID,
			},
			FirstName: "PlayerA",
			Username:  "playerA",
		},
		playerBID: {
			Participant: repository.Participant{
				MatchID: matchID,
				UserID:  playerBID,
			},
			FirstName: "PlayerB",
			Username:  "playerB",
		},
	}

	// Create battle round with specific damage values
	// Player A dealt 40 damage, Player B dealt 60 damage
	teamACards := []*battle.Card{
		{CardID: 1, UserID: playerAID, Name: "PlayerA", ATK: 10, HP: 15},
		{CardID: 2, UserID: playerAID, Name: "PlayerA", ATK: 15, HP: 20},
		{CardID: 3, UserID: playerAID, Name: "PlayerA", ATK: 15, HP: 10},
	}
	teamBCards := []*battle.Card{
		{CardID: 4, UserID: playerBID, Name: "PlayerB", ATK: 20, HP: 15},
		{CardID: 5, UserID: playerBID, Name: "PlayerB", ATK: 20, HP: 20},
		{CardID: 6, UserID: playerBID, Name: "PlayerB", ATK: 20, HP: 10},
	}

	// Sample battle events
	events := []battle.BattleEvent{
		{Type: "attack", Message: "Card4 attacks Card1"},
		{Type: "attack", Message: "Card1 attacks Card4"},
	}

	teamAJSON, _ := json.Marshal(teamACards)
	teamBJSON, _ := json.Marshal(teamBCards)
	eventsJSON, _ := json.Marshal(events)

	round := &repository.MatchRound{
		ID:          1,
		MatchID:     matchID,
		RoundNumber: 1,
		PlayerAID:   playerAID,
		PlayerBID:   playerBID,
		PlayerATeam: teamAJSON,
		PlayerBTeam: teamBJSON,
		PlayerADmg:  40, // Player A dealt 40 damage
		PlayerBDmg:  60, // Player B dealt 60 damage
		WinnerID:    &playerBID,
		IsDraw:      false,
		TotalRounds: 4,
		BattleLog:   eventsJSON,
	}
	mockRepo.Rounds[matchID] = []*repository.MatchRound{round}

	// Act: Request battle as Player B
	battleResp, err := svc.GetBattle(ctx, matchID, playerBID)

	// Assert
	if err != nil {
		t.Fatalf("GetBattle failed: %v", err)
	}

	if battleResp == nil {
		t.Fatal("Expected non-nil BattleResponse")
	}

	// When user is Player B:
	// - DamageDealt should be Player B's damage (60)
	// - DamageTaken should be Player A's damage (40)
	if battleResp.DamageDealt != 60 {
		t.Errorf("Expected DamageDealt=60 for Player B, got %d", battleResp.DamageDealt)
	}

	if battleResp.DamageTaken != 40 {
		t.Errorf("Expected DamageTaken=40 for Player B, got %d", battleResp.DamageTaken)
	}

	// Verify absolute damage values are still present
	if battleResp.TeamADamage != 40 {
		t.Errorf("Expected TeamADamage=40, got %d", battleResp.TeamADamage)
	}

	if battleResp.TeamBDamage != 60 {
		t.Errorf("Expected TeamBDamage=60, got %d", battleResp.TeamBDamage)
	}
}

// TestGetBattle_DamageSummary_NoRounds verifies that damage summary is set to zero
// when the match has no battle rounds (edge case).
func TestGetBattle_DamageSummary_NoRounds(t *testing.T) {
	mockRepo := testutil.NewMockGameRepository()

	svc := NewBattleService(nil, mockRepo, nil, nil)
	ctx := context.Background()

	matchID := "test-match-no-rounds"
	playerAID := int64(3001)
	playerBID := int64(3002)
	chatID := int64(-1003280306634)
	format := repository.MatchFormat1v1

	// Setup: Create a match with no rounds
	match := &repository.Match{
		ID:     matchID,
		ChatID: chatID,
		Format: &format,
		Status: repository.MatchStatusCompleted,
	}
	mockRepo.Matches[matchID] = match

	// Add participants
	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		playerAID: {
			MatchID: matchID,
			UserID:  playerAID,
		},
		playerBID: {
			MatchID: matchID,
			UserID:  playerBID,
		},
	}

	// Mock GetMatchParticipants to return ParticipantWithUser
	mockRepo.ParticipantsWithUser[matchID] = map[int64]*repository.ParticipantWithUser{
		playerAID: {
			Participant: repository.Participant{
				MatchID: matchID,
				UserID:  playerAID,
			},
			FirstName: "PlayerA",
			Username:  "playerA",
		},
		playerBID: {
			Participant: repository.Participant{
				MatchID: matchID,
				UserID:  playerBID,
			},
			FirstName: "PlayerB",
			Username:  "playerB",
		},
	}

	// NO ROUNDS - leave mockRepo.Rounds[matchID] empty/nil

	// Act: Request battle as Player A (no rounds exist)
	battleResp, err := svc.GetBattle(ctx, matchID, playerAID)

	// Assert
	if err != nil {
		t.Fatalf("GetBattle failed: %v", err)
	}

	if battleResp == nil {
		t.Fatal("Expected non-nil BattleResponse")
	}

	// When no rounds exist:
	// - DamageDealt should be 0
	// - DamageTaken should be 0
	if battleResp.DamageDealt != 0 {
		t.Errorf("Expected DamageDealt=0 when no rounds exist, got %d", battleResp.DamageDealt)
	}

	if battleResp.DamageTaken != 0 {
		t.Errorf("Expected DamageTaken=0 when no rounds exist, got %d", battleResp.DamageTaken)
	}

	// Verify absolute damage values are also 0
	if battleResp.TeamADamage != 0 {
		t.Errorf("Expected TeamADamage=0 when no rounds exist, got %d", battleResp.TeamADamage)
	}

	if battleResp.TeamBDamage != 0 {
		t.Errorf("Expected TeamBDamage=0 when no rounds exist, got %d", battleResp.TeamBDamage)
	}
}

// TestGetBattle_DamageSummary_Draw verifies that damage summary is calculated
// correctly when both teams deal equal damage (draw scenario).
func TestGetBattle_DamageSummary_Draw(t *testing.T) {
	mockRepo := testutil.NewMockGameRepository()

	svc := NewBattleService(nil, mockRepo, nil, nil)
	ctx := context.Background()

	matchID := "test-match-draw"
	playerAID := int64(4001)
	playerBID := int64(4002)
	chatID := int64(-1003280306634)
	format := repository.MatchFormat1v1

	// Setup: Create a match with Player A and Player B
	match := &repository.Match{
		ID:     matchID,
		ChatID: chatID,
		Format: &format,
		Status: repository.MatchStatusCompleted,
	}
	mockRepo.Matches[matchID] = match

	// Add participants
	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		playerAID: {
			MatchID: matchID,
			UserID:  playerAID,
		},
		playerBID: {
			MatchID: matchID,
			UserID:  playerBID,
		},
	}

	// Mock GetMatchParticipants to return ParticipantWithUser
	mockRepo.ParticipantsWithUser[matchID] = map[int64]*repository.ParticipantWithUser{
		playerAID: {
			Participant: repository.Participant{
				MatchID: matchID,
				UserID:  playerAID,
			},
			FirstName: "PlayerA",
			Username:  "playerA",
		},
		playerBID: {
			Participant: repository.Participant{
				MatchID: matchID,
				UserID:  playerBID,
			},
			FirstName: "PlayerB",
			Username:  "playerB",
		},
	}

	// Create battle round with EQUAL damage values (draw)
	// Player A dealt 45 damage, Player B dealt 45 damage
	teamACards := []*battle.Card{
		{CardID: 1, UserID: playerAID, Name: "PlayerA", ATK: 15, HP: 15},
		{CardID: 2, UserID: playerAID, Name: "PlayerA", ATK: 15, HP: 20},
		{CardID: 3, UserID: playerAID, Name: "PlayerA", ATK: 15, HP: 10},
	}
	teamBCards := []*battle.Card{
		{CardID: 4, UserID: playerBID, Name: "PlayerB", ATK: 15, HP: 15},
		{CardID: 5, UserID: playerBID, Name: "PlayerB", ATK: 15, HP: 20},
		{CardID: 6, UserID: playerBID, Name: "PlayerB", ATK: 15, HP: 10},
	}

	// Sample battle events
	events := []battle.BattleEvent{
		{Type: "attack", Message: "Card1 attacks Card4"},
		{Type: "attack", Message: "Card4 attacks Card1"},
		{Type: "draw", Message: "Battle ended in a draw"},
	}

	teamAJSON, _ := json.Marshal(teamACards)
	teamBJSON, _ := json.Marshal(teamBCards)
	eventsJSON, _ := json.Marshal(events)

	round := &repository.MatchRound{
		ID:          1,
		MatchID:     matchID,
		RoundNumber: 1,
		PlayerAID:   playerAID,
		PlayerBID:   playerBID,
		PlayerATeam: teamAJSON,
		PlayerBTeam: teamBJSON,
		PlayerADmg:  45, // Player A dealt 45 damage
		PlayerBDmg:  45, // Player B dealt 45 damage (EQUAL)
		WinnerID:    nil,
		IsDraw:      true,
		TotalRounds: 6,
		BattleLog:   eventsJSON,
	}
	mockRepo.Rounds[matchID] = []*repository.MatchRound{round}

	// Act: Request battle as Player A
	battleResp, err := svc.GetBattle(ctx, matchID, playerAID)

	// Assert
	if err != nil {
		t.Fatalf("GetBattle failed: %v", err)
	}

	if battleResp == nil {
		t.Fatal("Expected non-nil BattleResponse")
	}

	// When user is Player A in a draw:
	// - DamageDealt should be Player A's damage (45)
	// - DamageTaken should be Player B's damage (45)
	// Both should be EQUAL
	if battleResp.DamageDealt != 45 {
		t.Errorf("Expected DamageDealt=45 for Player A in draw, got %d", battleResp.DamageDealt)
	}

	if battleResp.DamageTaken != 45 {
		t.Errorf("Expected DamageTaken=45 for Player A in draw, got %d", battleResp.DamageTaken)
	}

	// Verify absolute damage values are equal
	if battleResp.TeamADamage != 45 {
		t.Errorf("Expected TeamADamage=45, got %d", battleResp.TeamADamage)
	}

	if battleResp.TeamBDamage != 45 {
		t.Errorf("Expected TeamBDamage=45, got %d", battleResp.TeamBDamage)
	}

	// Verify draw status
	if battleResp.IsDraw != true {
		t.Errorf("Expected IsDraw=true for draw scenario, got %v", battleResp.IsDraw)
	}
}
