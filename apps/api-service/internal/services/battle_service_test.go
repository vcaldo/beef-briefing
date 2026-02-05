package services

import (
	"context"
	"encoding/json"
	"testing"

	"beef-briefing/apps/api-service/internal/apperror"
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

// TestGetBattle_FreeForAll verifies that GetBattle returns a free_for_all summary
// with standings, ffa_rounds, and empty events/combats for 3-player matches.
func TestGetBattle_FreeForAll(t *testing.T) {
	mockRepo := testutil.NewMockGameRepository()
	svc := NewBattleService(nil, mockRepo, nil, nil)
	ctx := context.Background()

	matchID := "test-ffa-match"
	playerA := int64(5001)
	playerB := int64(5002)
	playerC := int64(5003)
	chatID := int64(-1003280306634)
	format := repository.MatchFormatFreeForAll

	// Setup match with free_for_all format
	winnerID := playerA
	match := &repository.Match{
		ID:           matchID,
		ChatID:       chatID,
		Format:       &format,
		Status:       repository.MatchStatusCompleted,
		WinnerUserID: &winnerID,
	}
	mockRepo.Matches[matchID] = match

	// Add participants
	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		playerA: {MatchID: matchID, UserID: playerA},
		playerB: {MatchID: matchID, UserID: playerB},
		playerC: {MatchID: matchID, UserID: playerC},
	}
	mockRepo.ParticipantsWithUser[matchID] = map[int64]*repository.ParticipantWithUser{
		playerA: {Participant: repository.Participant{MatchID: matchID, UserID: playerA}, FirstName: "Alice"},
		playerB: {Participant: repository.Participant{MatchID: matchID, UserID: playerB}, FirstName: "Bob"},
		playerC: {Participant: repository.Participant{MatchID: matchID, UserID: playerC}, FirstName: "Charlie"},
	}

	// Create 3 rounds (round-robin: A vs B, A vs C, B vs C)
	teamJSON, _ := json.Marshal([]*battle.Card{{CardID: 1, ATK: 10, HP: 10}})
	eventsJSON, _ := json.Marshal([]battle.BattleEvent{{Type: "attack", Message: "test"}})

	mockRepo.Rounds[matchID] = []*repository.MatchRound{
		{ID: 1, MatchID: matchID, RoundNumber: 1, PlayerAID: playerA, PlayerBID: playerB,
			PlayerATeam: teamJSON, PlayerBTeam: teamJSON, BattleLog: eventsJSON,
			WinnerID: &playerA, PlayerADmg: 30, PlayerBDmg: 20, TotalRounds: 5},
		{ID: 2, MatchID: matchID, RoundNumber: 2, PlayerAID: playerA, PlayerBID: playerC,
			PlayerATeam: teamJSON, PlayerBTeam: teamJSON, BattleLog: eventsJSON,
			WinnerID: &playerA, PlayerADmg: 25, PlayerBDmg: 15, TotalRounds: 4},
		{ID: 3, MatchID: matchID, RoundNumber: 3, PlayerAID: playerB, PlayerBID: playerC,
			PlayerATeam: teamJSON, PlayerBTeam: teamJSON, BattleLog: eventsJSON,
			WinnerID: &playerB, PlayerADmg: 20, PlayerBDmg: 10, TotalRounds: 3},
	}

	// Act: GetBattle as playerA
	resp, err := svc.GetBattle(ctx, matchID, playerA)
	if err != nil {
		t.Fatalf("GetBattle failed: %v", err)
	}

	// Assert format
	if resp.Format != "free_for_all" {
		t.Errorf("expected format 'free_for_all', got %q", resp.Format)
	}

	// Assert empty events/combats (individual rounds fetched via GetRoundBattle)
	if len(resp.Events) != 0 {
		t.Errorf("expected empty events for ffa summary, got %d", len(resp.Events))
	}
	if len(resp.Combats) != 0 {
		t.Errorf("expected empty combats for ffa summary, got %d", len(resp.Combats))
	}

	// Assert ffa_rounds
	if len(resp.FfaRounds) != 3 {
		t.Fatalf("expected 3 ffa_rounds, got %d", len(resp.FfaRounds))
	}
	if resp.FfaRounds[0].PlayerAName != "Alice" || resp.FfaRounds[0].PlayerBName != "Bob" {
		t.Errorf("round 1: expected Alice vs Bob, got %s vs %s", resp.FfaRounds[0].PlayerAName, resp.FfaRounds[0].PlayerBName)
	}
	if resp.FfaRounds[0].PlayerADmg != 30 {
		t.Errorf("round 1: expected PlayerADmg=30, got %d", resp.FfaRounds[0].PlayerADmg)
	}

	// Assert standings (sorted by wins desc, then damage desc)
	if len(resp.Standings) != 3 {
		t.Fatalf("expected 3 standings, got %d", len(resp.Standings))
	}
	// Alice: 2 wins, 55 total damage
	if resp.Standings[0].UserID != playerA {
		t.Errorf("expected rank 1 = Alice (5001), got %d", resp.Standings[0].UserID)
	}
	if resp.Standings[0].Wins != 2 {
		t.Errorf("expected Alice wins=2, got %d", resp.Standings[0].Wins)
	}
	if resp.Standings[0].TotalDamageDealt != 55 {
		t.Errorf("expected Alice damage=55, got %d", resp.Standings[0].TotalDamageDealt)
	}
	// Bob: 1 win, 40 total damage
	if resp.Standings[1].UserID != playerB {
		t.Errorf("expected rank 2 = Bob (5002), got %d", resp.Standings[1].UserID)
	}
	if resp.Standings[1].Wins != 1 {
		t.Errorf("expected Bob wins=1, got %d", resp.Standings[1].Wins)
	}

	// Assert winner
	if resp.WinnerID == nil || *resp.WinnerID != playerA {
		t.Errorf("expected winner=5001 (Alice), got %v", resp.WinnerID)
	}

	// Assert NumRounds is total number of rounds
	if resp.NumRounds != 3 {
		t.Errorf("expected NumRounds=3, got %d", resp.NumRounds)
	}
}

// TestGetBattle_Bracket verifies that GetBattle returns a bracket summary
// with bracket_rounds, standings, and empty events/combats for 4-player matches.
func TestGetBattle_Bracket(t *testing.T) {
	mockRepo := testutil.NewMockGameRepository()
	svc := NewBattleService(nil, mockRepo, nil, nil)
	ctx := context.Background()

	matchID := "test-bracket-match"
	p1 := int64(6001)
	p2 := int64(6002)
	p3 := int64(6003)
	p4 := int64(6004)
	chatID := int64(-1003280306634)
	format := repository.MatchFormatBracket

	// Setup: 4-player bracket → 2 semis + 1 final = 3 rounds
	winnerID := p1
	match := &repository.Match{
		ID:           matchID,
		ChatID:       chatID,
		Format:       &format,
		Status:       repository.MatchStatusCompleted,
		WinnerUserID: &winnerID,
	}
	mockRepo.Matches[matchID] = match

	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		p1: {MatchID: matchID, UserID: p1},
		p2: {MatchID: matchID, UserID: p2},
		p3: {MatchID: matchID, UserID: p3},
		p4: {MatchID: matchID, UserID: p4},
	}
	mockRepo.ParticipantsWithUser[matchID] = map[int64]*repository.ParticipantWithUser{
		p1: {Participant: repository.Participant{MatchID: matchID, UserID: p1}, FirstName: "P1"},
		p2: {Participant: repository.Participant{MatchID: matchID, UserID: p2}, FirstName: "P2"},
		p3: {Participant: repository.Participant{MatchID: matchID, UserID: p3}, FirstName: "P3"},
		p4: {Participant: repository.Participant{MatchID: matchID, UserID: p4}, FirstName: "P4"},
	}

	teamJSON, _ := json.Marshal([]*battle.Card{{CardID: 1, ATK: 10, HP: 10}})
	eventsJSON, _ := json.Marshal([]battle.BattleEvent{{Type: "attack", Message: "test"}})

	// Semi 1: P1 vs P2 → P1 wins
	// Semi 2: P3 vs P4 → P3 wins
	// Final: P1 vs P3 → P1 wins
	mockRepo.Rounds[matchID] = []*repository.MatchRound{
		{ID: 1, MatchID: matchID, RoundNumber: 1, PlayerAID: p1, PlayerBID: p2,
			PlayerATeam: teamJSON, PlayerBTeam: teamJSON, BattleLog: eventsJSON,
			WinnerID: &p1, PlayerADmg: 40, PlayerBDmg: 25, TotalRounds: 5},
		{ID: 2, MatchID: matchID, RoundNumber: 2, PlayerAID: p3, PlayerBID: p4,
			PlayerATeam: teamJSON, PlayerBTeam: teamJSON, BattleLog: eventsJSON,
			WinnerID: &p3, PlayerADmg: 35, PlayerBDmg: 20, TotalRounds: 4},
		{ID: 3, MatchID: matchID, RoundNumber: 3, PlayerAID: p1, PlayerBID: p3,
			PlayerATeam: teamJSON, PlayerBTeam: teamJSON, BattleLog: eventsJSON,
			WinnerID: &p1, PlayerADmg: 50, PlayerBDmg: 30, TotalRounds: 6},
	}

	// Act
	resp, err := svc.GetBattle(ctx, matchID, p1)
	if err != nil {
		t.Fatalf("GetBattle failed: %v", err)
	}

	// Assert format
	if resp.Format != "bracket" {
		t.Errorf("expected format 'bracket', got %q", resp.Format)
	}

	// Assert empty events/combats
	if len(resp.Events) != 0 {
		t.Errorf("expected empty events for bracket summary, got %d", len(resp.Events))
	}

	// Assert bracket_rounds: 2 rounds (Semifinals with 2 matches, Final with 1 match)
	if len(resp.BracketRounds) != 2 {
		t.Fatalf("expected 2 bracket rounds, got %d", len(resp.BracketRounds))
	}
	if resp.BracketRounds[0].Name != "Semifinals" {
		t.Errorf("expected first round name 'Semifinals', got %q", resp.BracketRounds[0].Name)
	}
	if len(resp.BracketRounds[0].Matches) != 2 {
		t.Errorf("expected 2 semifinal matches, got %d", len(resp.BracketRounds[0].Matches))
	}
	if resp.BracketRounds[1].Name != "Final" {
		t.Errorf("expected second round name 'Final', got %q", resp.BracketRounds[1].Name)
	}
	if len(resp.BracketRounds[1].Matches) != 1 {
		t.Errorf("expected 1 final match, got %d", len(resp.BracketRounds[1].Matches))
	}

	// Assert semifinal match details
	semi1 := resp.BracketRounds[0].Matches[0]
	if semi1.PlayerAName != "P1" || semi1.PlayerBName != "P2" {
		t.Errorf("semi 1: expected P1 vs P2, got %s vs %s", semi1.PlayerAName, semi1.PlayerBName)
	}
	if semi1.WinnerID == nil || *semi1.WinnerID != p1 {
		t.Errorf("semi 1: expected winner P1, got %v", semi1.WinnerID)
	}

	// Assert final match details
	final := resp.BracketRounds[1].Matches[0]
	if final.PlayerAName != "P1" || final.PlayerBName != "P3" {
		t.Errorf("final: expected P1 vs P3, got %s vs %s", final.PlayerAName, final.PlayerBName)
	}

	// Assert standings
	if len(resp.Standings) != 4 {
		t.Fatalf("expected 4 standings, got %d", len(resp.Standings))
	}
	// P1: 2 wins (semi + final), 90 damage
	if resp.Standings[0].UserID != p1 {
		t.Errorf("expected rank 1 = P1, got %d", resp.Standings[0].UserID)
	}
	if resp.Standings[0].Wins != 2 {
		t.Errorf("expected P1 wins=2, got %d", resp.Standings[0].Wins)
	}

	// Assert winner
	if resp.WinnerID == nil || *resp.WinnerID != p1 {
		t.Errorf("expected winner=P1, got %v", resp.WinnerID)
	}

	// Assert NumRounds is total number of rounds
	if resp.NumRounds != 3 {
		t.Errorf("expected NumRounds=3, got %d", resp.NumRounds)
	}
}

// TestGetBattle_1v1Format verifies that GetBattle includes Format "1v1" for 1v1 matches.
func TestGetBattle_1v1Format(t *testing.T) {
	mockRepo := testutil.NewMockGameRepository()
	svc := NewBattleService(nil, mockRepo, nil, nil)
	ctx := context.Background()

	matchID := "test-1v1-format"
	playerAID := int64(7001)
	playerBID := int64(7002)
	chatID := int64(-1003280306634)
	format := repository.MatchFormat1v1

	match := &repository.Match{
		ID:     matchID,
		ChatID: chatID,
		Format: &format,
		Status: repository.MatchStatusCompleted,
	}
	mockRepo.Matches[matchID] = match

	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		playerAID: {MatchID: matchID, UserID: playerAID},
		playerBID: {MatchID: matchID, UserID: playerBID},
	}
	mockRepo.ParticipantsWithUser[matchID] = map[int64]*repository.ParticipantWithUser{
		playerAID: {Participant: repository.Participant{MatchID: matchID, UserID: playerAID}, FirstName: "Alice"},
		playerBID: {Participant: repository.Participant{MatchID: matchID, UserID: playerBID}, FirstName: "Bob"},
	}

	teamJSON, _ := json.Marshal([]*battle.Card{{CardID: 1, ATK: 10, HP: 10}})
	eventsJSON, _ := json.Marshal([]battle.BattleEvent{{Type: "attack", Message: "test"}})

	mockRepo.Rounds[matchID] = []*repository.MatchRound{
		{ID: 1, MatchID: matchID, RoundNumber: 1, PlayerAID: playerAID, PlayerBID: playerBID,
			PlayerATeam: teamJSON, PlayerBTeam: teamJSON, BattleLog: eventsJSON,
			WinnerID: &playerAID, PlayerADmg: 30, PlayerBDmg: 20, TotalRounds: 5},
	}

	resp, err := svc.GetBattle(ctx, matchID, playerAID)
	if err != nil {
		t.Fatalf("GetBattle failed: %v", err)
	}

	if resp.Format != "1v1" {
		t.Errorf("expected format '1v1', got %q", resp.Format)
	}

	// 1v1 should have events and combats populated
	if len(resp.Events) == 0 {
		t.Error("expected non-empty events for 1v1")
	}

	// Bracket/FFA fields should be empty
	if len(resp.BracketRounds) != 0 {
		t.Errorf("expected no bracket_rounds for 1v1, got %d", len(resp.BracketRounds))
	}
	if len(resp.FfaRounds) != 0 {
		t.Errorf("expected no ffa_rounds for 1v1, got %d", len(resp.FfaRounds))
	}
}

// TestGetRoundBattle verifies that GetRoundBattle returns full events/combats
// for a specific round within a multi-round (free_for_all) match.
func TestGetRoundBattle(t *testing.T) {
	mockRepo := testutil.NewMockGameRepository()
	svc := NewBattleService(nil, mockRepo, nil, nil)
	ctx := context.Background()

	matchID := "test-round-battle"
	playerA := int64(8001)
	playerB := int64(8002)
	playerC := int64(8003)
	chatID := int64(-1003280306634)
	format := repository.MatchFormatFreeForAll

	match := &repository.Match{
		ID:     matchID,
		ChatID: chatID,
		Format: &format,
		Status: repository.MatchStatusCompleted,
	}
	mockRepo.Matches[matchID] = match

	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		playerA: {MatchID: matchID, UserID: playerA},
		playerB: {MatchID: matchID, UserID: playerB},
		playerC: {MatchID: matchID, UserID: playerC},
	}
	mockRepo.ParticipantsWithUser[matchID] = map[int64]*repository.ParticipantWithUser{
		playerA: {Participant: repository.Participant{MatchID: matchID, UserID: playerA}, FirstName: "Alice"},
		playerB: {Participant: repository.Participant{MatchID: matchID, UserID: playerB}, FirstName: "Bob"},
		playerC: {Participant: repository.Participant{MatchID: matchID, UserID: playerC}, FirstName: "Charlie"},
	}

	teamJSON, _ := json.Marshal([]*battle.Card{{CardID: 1, ATK: 10, HP: 10}})
	eventsJSON, _ := json.Marshal([]battle.BattleEvent{
		{Type: "attack", Message: "Alice attacks Bob"},
		{Type: "attack", Message: "Bob attacks Alice"},
	})

	mockRepo.Rounds[matchID] = []*repository.MatchRound{
		{ID: 1, MatchID: matchID, RoundNumber: 1, PlayerAID: playerA, PlayerBID: playerB,
			PlayerATeam: teamJSON, PlayerBTeam: teamJSON, BattleLog: eventsJSON,
			WinnerID: &playerA, PlayerADmg: 30, PlayerBDmg: 20, TotalRounds: 5},
		{ID: 2, MatchID: matchID, RoundNumber: 2, PlayerAID: playerA, PlayerBID: playerC,
			PlayerATeam: teamJSON, PlayerBTeam: teamJSON, BattleLog: eventsJSON,
			WinnerID: &playerA, PlayerADmg: 25, PlayerBDmg: 15, TotalRounds: 4},
		{ID: 3, MatchID: matchID, RoundNumber: 3, PlayerAID: playerB, PlayerBID: playerC,
			PlayerATeam: teamJSON, PlayerBTeam: teamJSON, BattleLog: eventsJSON,
			WinnerID: &playerB, PlayerADmg: 20, PlayerBDmg: 10, TotalRounds: 3},
	}

	// Get round 2 as playerA (who is player A in round 2)
	resp, err := svc.GetRoundBattle(ctx, matchID, 2, playerA)
	if err != nil {
		t.Fatalf("GetRoundBattle failed: %v", err)
	}

	// Should have full events for animated replay
	if len(resp.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(resp.Events))
	}

	// Should have combats
	if len(resp.Combats) == 0 {
		t.Error("expected non-empty combats for round replay")
	}

	// Verify player info from round 2 (A vs C)
	if resp.PlayerAID != playerA {
		t.Errorf("expected PlayerAID=%d, got %d", playerA, resp.PlayerAID)
	}
	if resp.PlayerBID != playerC {
		t.Errorf("expected PlayerBID=%d, got %d", playerC, resp.PlayerBID)
	}
	if resp.PlayerAName != "Alice" {
		t.Errorf("expected PlayerAName='Alice', got %q", resp.PlayerAName)
	}
	if resp.PlayerBName != "Charlie" {
		t.Errorf("expected PlayerBName='Charlie', got %q", resp.PlayerBName)
	}

	// Player-relative damage (playerA is player A in this round)
	if resp.DamageDealt != 25 {
		t.Errorf("expected DamageDealt=25, got %d", resp.DamageDealt)
	}
	if resp.DamageTaken != 15 {
		t.Errorf("expected DamageTaken=15, got %d", resp.DamageTaken)
	}

	// Verify round damage values
	if resp.TeamADamage != 25 {
		t.Errorf("expected TeamADamage=25, got %d", resp.TeamADamage)
	}
	if resp.TeamBDamage != 15 {
		t.Errorf("expected TeamBDamage=15, got %d", resp.TeamBDamage)
	}

	// Winner
	if resp.WinnerID == nil || *resp.WinnerID != playerA {
		t.Errorf("expected winner=%d, got %v", playerA, resp.WinnerID)
	}

	// NumRounds should be the round's internal battle round count
	if resp.NumRounds != 4 {
		t.Errorf("expected NumRounds=4, got %d", resp.NumRounds)
	}
}

// TestGetRoundBattle_PlayerBPerspective verifies that damage is swapped
// when the requesting user is player B in the round.
func TestGetRoundBattle_PlayerBPerspective(t *testing.T) {
	mockRepo := testutil.NewMockGameRepository()
	svc := NewBattleService(nil, mockRepo, nil, nil)
	ctx := context.Background()

	matchID := "test-round-battle-b"
	playerA := int64(8001)
	playerB := int64(8002)
	playerC := int64(8003)
	chatID := int64(-1003280306634)
	format := repository.MatchFormatFreeForAll

	match := &repository.Match{
		ID:     matchID,
		ChatID: chatID,
		Format: &format,
		Status: repository.MatchStatusCompleted,
	}
	mockRepo.Matches[matchID] = match

	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		playerA: {MatchID: matchID, UserID: playerA},
		playerB: {MatchID: matchID, UserID: playerB},
		playerC: {MatchID: matchID, UserID: playerC},
	}
	mockRepo.ParticipantsWithUser[matchID] = map[int64]*repository.ParticipantWithUser{
		playerA: {Participant: repository.Participant{MatchID: matchID, UserID: playerA}, FirstName: "Alice"},
		playerB: {Participant: repository.Participant{MatchID: matchID, UserID: playerB}, FirstName: "Bob"},
		playerC: {Participant: repository.Participant{MatchID: matchID, UserID: playerC}, FirstName: "Charlie"},
	}

	teamJSON, _ := json.Marshal([]*battle.Card{{CardID: 1, ATK: 10, HP: 10}})
	eventsJSON, _ := json.Marshal([]battle.BattleEvent{{Type: "attack", Message: "test"}})

	mockRepo.Rounds[matchID] = []*repository.MatchRound{
		{ID: 1, MatchID: matchID, RoundNumber: 1, PlayerAID: playerA, PlayerBID: playerB,
			PlayerATeam: teamJSON, PlayerBTeam: teamJSON, BattleLog: eventsJSON,
			WinnerID: &playerA, PlayerADmg: 30, PlayerBDmg: 20, TotalRounds: 5},
	}

	// Get round 1 as playerB (who is player B in this round)
	resp, err := svc.GetRoundBattle(ctx, matchID, 1, playerB)
	if err != nil {
		t.Fatalf("GetRoundBattle failed: %v", err)
	}

	// Player-relative damage should be swapped for player B
	if resp.DamageDealt != 20 {
		t.Errorf("expected DamageDealt=20 (player B's damage), got %d", resp.DamageDealt)
	}
	if resp.DamageTaken != 30 {
		t.Errorf("expected DamageTaken=30 (player A's damage), got %d", resp.DamageTaken)
	}
}

// TestGetRoundBattle_RoundNotFound verifies that requesting a non-existent round
// returns ErrRoundNotFound.
func TestGetRoundBattle_RoundNotFound(t *testing.T) {
	mockRepo := testutil.NewMockGameRepository()
	svc := NewBattleService(nil, mockRepo, nil, nil)
	ctx := context.Background()

	matchID := "test-round-notfound"
	playerA := int64(8001)
	chatID := int64(-1003280306634)
	format := repository.MatchFormatFreeForAll

	match := &repository.Match{
		ID:     matchID,
		ChatID: chatID,
		Format: &format,
		Status: repository.MatchStatusCompleted,
	}
	mockRepo.Matches[matchID] = match

	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		playerA: {MatchID: matchID, UserID: playerA},
	}

	mockRepo.Rounds[matchID] = []*repository.MatchRound{} // No rounds

	_, err := svc.GetRoundBattle(ctx, matchID, 99, playerA)
	if err != apperror.ErrRoundNotFound {
		t.Errorf("expected ErrRoundNotFound, got %v", err)
	}
}

// TestGetRoundBattle_NotParticipant verifies that a non-participant
// cannot access round battle data.
func TestGetRoundBattle_NotParticipant(t *testing.T) {
	mockRepo := testutil.NewMockGameRepository()
	svc := NewBattleService(nil, mockRepo, nil, nil)
	ctx := context.Background()

	matchID := "test-round-noparticipant"
	playerA := int64(8001)
	outsider := int64(9999)
	chatID := int64(-1003280306634)
	format := repository.MatchFormatFreeForAll

	match := &repository.Match{
		ID:     matchID,
		ChatID: chatID,
		Format: &format,
		Status: repository.MatchStatusCompleted,
	}
	mockRepo.Matches[matchID] = match

	mockRepo.Participants[matchID] = map[int64]*repository.Participant{
		playerA: {MatchID: matchID, UserID: playerA},
	}

	_, err := svc.GetRoundBattle(ctx, matchID, 1, outsider)
	if err != apperror.ErrNotParticipant {
		t.Errorf("expected ErrNotParticipant, got %v", err)
	}
}
