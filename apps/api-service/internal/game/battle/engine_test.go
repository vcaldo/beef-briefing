package battle

import (
	"testing"
)

// =============================================================================
// Test Fixtures
// =============================================================================

// newTestCard creates a card with the specified stats for testing.
func newTestCard(cardID, userID int64, name string, atk, hp int) *Card {
	return &Card{
		CardID: cardID,
		UserID: userID,
		Name:   name,
		ATK:    atk,
		HP:     hp,
		MaxHP:  hp,
	}
}

// newTestTeam creates a team with the given cards for testing.
func newTestTeam(ownerID int64, ownerName string, cards ...*Card) *Team {
	return NewTeam(ownerID, ownerName, cards)
}

// =============================================================================
// Simulate Tests
// =============================================================================

func TestSimulate_TeamAWins_ByElimination(t *testing.T) {
	// Team A has stronger cards and should win by eliminating Team B
	teamA := newTestTeam(1, "Alice",
		newTestCard(101, 1, "Alice1", 20, 50),
		newTestCard(102, 1, "Alice2", 15, 40),
		newTestCard(103, 1, "Alice3", 10, 30),
	)
	teamB := newTestTeam(2, "Bob",
		newTestCard(201, 2, "Bob1", 5, 20),
		newTestCard(202, 2, "Bob2", 5, 20),
		newTestCard(203, 2, "Bob3", 5, 20),
	)

	result := Simulate(teamA, teamB)

	if result.IsDraw {
		t.Error("expected Team A to win, got draw")
	}
	if result.WinnerID == nil {
		t.Fatal("expected WinnerID to be set")
	}
	if *result.WinnerID != 1 {
		t.Errorf("expected WinnerID=1, got %d", *result.WinnerID)
	}
	// Team A should have cards remaining
	if result.TeamAFinal.RemainingCards() == 0 {
		t.Error("expected Team A to have remaining cards")
	}
	// Team B should be eliminated
	if result.TeamBFinal.RemainingCards() != 0 {
		t.Error("expected Team B to be eliminated")
	}
}

func TestSimulate_TeamBWins_ByElimination(t *testing.T) {
	// Team B has stronger cards and should win by eliminating Team A
	teamA := newTestTeam(1, "Alice",
		newTestCard(101, 1, "Alice1", 5, 20),
		newTestCard(102, 1, "Alice2", 5, 20),
		newTestCard(103, 1, "Alice3", 5, 20),
	)
	teamB := newTestTeam(2, "Bob",
		newTestCard(201, 2, "Bob1", 20, 50),
		newTestCard(202, 2, "Bob2", 15, 40),
		newTestCard(203, 2, "Bob3", 10, 30),
	)

	result := Simulate(teamA, teamB)

	if result.IsDraw {
		t.Error("expected Team B to win, got draw")
	}
	if result.WinnerID == nil {
		t.Fatal("expected WinnerID to be set")
	}
	if *result.WinnerID != 2 {
		t.Errorf("expected WinnerID=2, got %d", *result.WinnerID)
	}
	// Team B should have cards remaining
	if result.TeamBFinal.RemainingCards() == 0 {
		t.Error("expected Team B to have remaining cards")
	}
	// Team A should be eliminated
	if result.TeamAFinal.RemainingCards() != 0 {
		t.Error("expected Team A to be eliminated")
	}
}

func TestSimulate_Draw_EqualDamage(t *testing.T) {
	// Both teams with identical cards should result in a draw
	teamA := newTestTeam(1, "Alice",
		newTestCard(101, 1, "Alice1", 10, 20),
	)
	teamB := newTestTeam(2, "Bob",
		newTestCard(201, 2, "Bob1", 10, 20),
	)

	result := Simulate(teamA, teamB)

	if !result.IsDraw {
		t.Error("expected draw")
	}
	if result.WinnerID != nil {
		t.Errorf("expected WinnerID to be nil for draw, got %d", *result.WinnerID)
	}
	// Both teams should deal equal damage
	if result.TeamADamage != result.TeamBDamage {
		t.Errorf("expected equal damage, got TeamA=%d TeamB=%d", result.TeamADamage, result.TeamBDamage)
	}
}

func TestSimulate_WinByDamage_BothTeamsEliminated(t *testing.T) {
	// Both teams eliminated in same round, winner determined by damage dealt
	// Team A has higher ATK (deals more damage) but same HP
	teamA := newTestTeam(1, "Alice",
		newTestCard(101, 1, "Alice1", 15, 20),
	)
	teamB := newTestTeam(2, "Bob",
		newTestCard(201, 2, "Bob1", 10, 20),
	)

	result := Simulate(teamA, teamB)

	// Team A dealt more damage so should win (both eliminated but A dealt 15, B dealt 10)
	// Actually both will die in round 2:
	// Round 1: Alice1 HP=20-10=10, Bob1 HP=20-15=5
	// Round 2: Alice1 HP=10-10=0, Bob1 HP=5-15=-10
	// Both dead at same time, TeamA dealt 30 damage, TeamB dealt 20 damage
	if result.IsDraw {
		t.Error("expected Team A to win by damage, got draw")
	}
	if result.WinnerID == nil {
		t.Fatal("expected WinnerID to be set")
	}
	if *result.WinnerID != 1 {
		t.Errorf("expected WinnerID=1 (Team A wins by damage), got %d", *result.WinnerID)
	}
	if result.TeamADamage <= result.TeamBDamage {
		t.Errorf("expected TeamA damage > TeamB damage, got A=%d B=%d", result.TeamADamage, result.TeamBDamage)
	}
}

func TestSimulate_KillStreak_MultipleKills(t *testing.T) {
	// Team A's strong card should get multiple kills
	teamA := newTestTeam(1, "Alice",
		newTestCard(101, 1, "Alice1", 25, 100), // Very strong
	)
	teamB := newTestTeam(2, "Bob",
		newTestCard(201, 2, "Bob1", 5, 10),
		newTestCard(202, 2, "Bob2", 5, 10),
		newTestCard(203, 2, "Bob3", 5, 10),
	)

	result := Simulate(teamA, teamB)

	// Alice1 should defeat all 3 opponents
	// Check for kill streak in summary events
	foundStreak := false
	for _, event := range result.Events {
		if event.Type == EventSummary && event.KillStreak >= 2 {
			foundStreak = true
			break
		}
	}
	if !foundStreak {
		t.Error("expected to find kill streak >= 2 in events")
	}

	if result.IsDraw {
		t.Error("expected Team A to win")
	}
	if result.WinnerID == nil || *result.WinnerID != 1 {
		t.Error("expected Team A (ownerID=1) to win")
	}
}

func TestSimulate_SingleRound_OneHitKO(t *testing.T) {
	// Single combat round battle where Team A one-hits Team B's only card
	teamA := newTestTeam(1, "Alice",
		newTestCard(101, 1, "Alice1", 100, 50),
	)
	teamB := newTestTeam(2, "Bob",
		newTestCard(201, 2, "Bob1", 10, 50),
	)

	result := Simulate(teamA, teamB)

	// Alice deals 100 damage, Bob has 50 HP -> dead in round 1
	// Bob deals 10 damage, Alice has 50 HP -> still alive
	// NumRounds captures the round when victory is detected (round 2 checks and finds Team B empty)
	if result.NumRounds != 2 {
		t.Errorf("expected 2 rounds (1 combat + victory detection), got %d", result.NumRounds)
	}
	if result.IsDraw {
		t.Error("expected Team A to win")
	}
	if result.WinnerID == nil || *result.WinnerID != 1 {
		t.Error("expected Team A to win")
	}
	// Verify only 1 round of actual combat happened (damage dealt)
	if result.TeamADamage != 100 {
		t.Errorf("expected Team A to deal exactly 100 damage, got %d", result.TeamADamage)
	}
	if result.TeamBDamage != 10 {
		t.Errorf("expected Team B to deal exactly 10 damage, got %d", result.TeamBDamage)
	}
}

func TestSimulate_MaxRoundsLimit(t *testing.T) {
	// Test that battle doesn't exceed MaxRounds (unlikely scenario but should be handled)
	// Create cards with very high HP and low ATK to potentially trigger max rounds
	teamA := newTestTeam(1, "Alice",
		newTestCard(101, 1, "Alice1", 1, 100),
	)
	teamB := newTestTeam(2, "Bob",
		newTestCard(201, 2, "Bob1", 1, 100),
	)

	result := Simulate(teamA, teamB)

	// With ATK=1 and HP=100, each card dies after 100 rounds
	// Both should die at same round, equal damage -> draw
	if result.NumRounds > MaxRounds {
		t.Errorf("battle exceeded MaxRounds: got %d rounds", result.NumRounds)
	}
}

func TestSimulate_DoesNotMutateInputTeams(t *testing.T) {
	// Verify that Simulate doesn't mutate the original teams
	teamA := newTestTeam(1, "Alice",
		newTestCard(101, 1, "Alice1", 20, 50),
	)
	teamB := newTestTeam(2, "Bob",
		newTestCard(201, 2, "Bob1", 10, 30),
	)

	// Store original HP values
	originalAHP := teamA.Cards[0].HP
	originalBHP := teamB.Cards[0].HP

	_ = Simulate(teamA, teamB)

	// Verify original teams unchanged
	if teamA.Cards[0].HP != originalAHP {
		t.Errorf("Team A card HP mutated: expected %d, got %d", originalAHP, teamA.Cards[0].HP)
	}
	if teamB.Cards[0].HP != originalBHP {
		t.Errorf("Team B card HP mutated: expected %d, got %d", originalBHP, teamB.Cards[0].HP)
	}
}

// =============================================================================
// GenerateHealthBar Tests
// =============================================================================

func TestGenerateHealthBar_FullHealth(t *testing.T) {
	bar := generateHealthBar(100, 100)
	expected := "[██████████]"
	if bar != expected {
		t.Errorf("expected %q, got %q", expected, bar)
	}
}

func TestGenerateHealthBar_Empty(t *testing.T) {
	bar := generateHealthBar(0, 100)
	expected := "[░░░░░░░░░░]"
	if bar != expected {
		t.Errorf("expected %q, got %q", expected, bar)
	}
}

func TestGenerateHealthBar_ZeroMaxHP(t *testing.T) {
	bar := generateHealthBar(50, 0)
	expected := "[░░░░░░░░░░]"
	if bar != expected {
		t.Errorf("expected %q for zero maxHP, got %q", expected, bar)
	}
}

func TestGenerateHealthBar_Half(t *testing.T) {
	bar := generateHealthBar(50, 100)
	expected := "[█████░░░░░]"
	if bar != expected {
		t.Errorf("expected %q, got %q", expected, bar)
	}
}

func TestGenerateHealthBar_NegativeHP(t *testing.T) {
	bar := generateHealthBar(-10, 100)
	expected := "[░░░░░░░░░░]"
	if bar != expected {
		t.Errorf("expected %q for negative HP, got %q", expected, bar)
	}
}

func TestGenerateHealthBar_OverMaxHP(t *testing.T) {
	// Edge case: current > max (shouldn't happen but handle gracefully)
	bar := generateHealthBar(150, 100)
	expected := "[██████████]"
	if bar != expected {
		t.Errorf("expected %q for over-max HP, got %q", expected, bar)
	}
}

// =============================================================================
// Team Tests
// =============================================================================

func TestTeam_Clone_DoesNotMutateOriginal(t *testing.T) {
	original := newTestTeam(1, "Alice",
		newTestCard(101, 1, "Alice1", 10, 50),
		newTestCard(102, 1, "Alice2", 15, 40),
	)

	clone := original.Clone()

	// Modify clone
	clone.OwnerID = 999
	clone.OwnerName = "Modified"
	clone.Cards[0].HP = 0
	clone.Cards[0].Name = "Modified"

	// Verify original unchanged
	if original.OwnerID != 1 {
		t.Errorf("original OwnerID mutated: expected 1, got %d", original.OwnerID)
	}
	if original.OwnerName != "Alice" {
		t.Errorf("original OwnerName mutated: expected Alice, got %s", original.OwnerName)
	}
	if original.Cards[0].HP != 50 {
		t.Errorf("original card HP mutated: expected 50, got %d", original.Cards[0].HP)
	}
	if original.Cards[0].Name != "Alice1" {
		t.Errorf("original card Name mutated: expected Alice1, got %s", original.Cards[0].Name)
	}
}

func TestTeam_GetFront_SkipsDeadCards(t *testing.T) {
	team := newTestTeam(1, "Alice",
		newTestCard(101, 1, "Alice1", 10, 0),  // Dead (HP=0)
		newTestCard(102, 1, "Alice2", 15, 0),  // Dead (HP=0)
		newTestCard(103, 1, "Alice3", 20, 30), // Alive
	)

	front := team.GetFront()

	if front == nil {
		t.Fatal("expected to get front card")
	}
	if front.CardID != 103 {
		t.Errorf("expected front card ID=103 (first alive), got %d", front.CardID)
	}
	if front.Name != "Alice3" {
		t.Errorf("expected front card Name=Alice3, got %s", front.Name)
	}
}

func TestTeam_GetFront_ReturnsNilWhenAllDead(t *testing.T) {
	team := newTestTeam(1, "Alice",
		newTestCard(101, 1, "Alice1", 10, 0), // Dead
		newTestCard(102, 1, "Alice2", 15, 0), // Dead
		newTestCard(103, 1, "Alice3", 20, 0), // Dead
	)

	front := team.GetFront()

	if front != nil {
		t.Errorf("expected nil when all cards dead, got card %d", front.CardID)
	}
}

func TestTeam_RemainingCards(t *testing.T) {
	team := newTestTeam(1, "Alice",
		newTestCard(101, 1, "Alice1", 10, 50), // Alive
		newTestCard(102, 1, "Alice2", 15, 0),  // Dead
		newTestCard(103, 1, "Alice3", 20, 30), // Alive
	)

	remaining := team.RemainingCards()

	if remaining != 2 {
		t.Errorf("expected 2 remaining cards, got %d", remaining)
	}
}

func TestTeam_IsEmpty(t *testing.T) {
	aliveTeam := newTestTeam(1, "Alice",
		newTestCard(101, 1, "Alice1", 10, 50),
	)
	deadTeam := newTestTeam(2, "Bob",
		newTestCard(201, 2, "Bob1", 10, 0),
	)

	if aliveTeam.IsEmpty() {
		t.Error("expected alive team to not be empty")
	}
	if !deadTeam.IsEmpty() {
		t.Error("expected dead team to be empty")
	}
}

// =============================================================================
// Card Tests
// =============================================================================

func TestCard_Clone(t *testing.T) {
	original := newTestCard(101, 1, "Alice", 20, 50)

	clone := original.Clone()

	// Modify clone
	clone.HP = 0
	clone.Name = "Modified"

	// Verify original unchanged
	if original.HP != 50 {
		t.Errorf("original HP mutated: expected 50, got %d", original.HP)
	}
	if original.Name != "Alice" {
		t.Errorf("original Name mutated: expected Alice, got %s", original.Name)
	}
}

func TestCard_Clone_NilCard(t *testing.T) {
	var nilCard *Card
	clone := nilCard.Clone()

	if clone != nil {
		t.Error("expected nil clone for nil card")
	}
}

// =============================================================================
// Event Generation Tests
// =============================================================================

func TestSimulate_GeneratesAttackEvents(t *testing.T) {
	teamA := newTestTeam(1, "Alice",
		newTestCard(101, 1, "Alice1", 10, 50),
	)
	teamB := newTestTeam(2, "Bob",
		newTestCard(201, 2, "Bob1", 10, 50),
	)

	result := Simulate(teamA, teamB)

	// Should have attack events
	foundAttack := false
	for _, event := range result.Events {
		if event.Type == EventAttack {
			foundAttack = true
			if event.Damage <= 0 {
				t.Error("attack event should have positive damage")
			}
			break
		}
	}
	if !foundAttack {
		t.Error("expected attack events in battle")
	}
}

func TestSimulate_GeneratesDeathEvents(t *testing.T) {
	teamA := newTestTeam(1, "Alice",
		newTestCard(101, 1, "Alice1", 100, 50), // Will kill Bob quickly
	)
	teamB := newTestTeam(2, "Bob",
		newTestCard(201, 2, "Bob1", 10, 20), // Low HP, will die
	)

	result := Simulate(teamA, teamB)

	foundDeath := false
	for _, event := range result.Events {
		if event.Type == EventDeath {
			foundDeath = true
			break
		}
	}
	if !foundDeath {
		t.Error("expected death event when card is defeated")
	}
}

func TestSimulate_GeneratesVictoryEvent(t *testing.T) {
	teamA := newTestTeam(1, "Alice",
		newTestCard(101, 1, "Alice1", 100, 50),
	)
	teamB := newTestTeam(2, "Bob",
		newTestCard(201, 2, "Bob1", 10, 20),
	)

	result := Simulate(teamA, teamB)

	// Last event should be victory
	if len(result.Events) == 0 {
		t.Fatal("expected at least one event")
	}
	lastEvent := result.Events[len(result.Events)-1]
	if lastEvent.Type != EventVictory {
		t.Errorf("expected last event to be victory, got %s", lastEvent.Type)
	}
}

func TestSimulate_CardStatesIncluded(t *testing.T) {
	teamA := newTestTeam(1, "Alice",
		newTestCard(101, 1, "Alice1", 10, 50),
	)
	teamB := newTestTeam(2, "Bob",
		newTestCard(201, 2, "Bob1", 10, 50),
	)

	result := Simulate(teamA, teamB)

	// Attack events should include card states
	for _, event := range result.Events {
		if event.Type == EventAttack && len(event.CardStates) == 0 {
			t.Error("attack events should include card states")
			break
		}
	}
}
