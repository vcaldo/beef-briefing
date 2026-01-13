package battle

import (
	"fmt"
	"strings"
)

const (
	// MaxRounds prevents infinite loops
	MaxRounds = 100
)

// duelStats tracks statistics for a card-vs-card duel
type duelStats struct {
	card        *Card  // The card fighting
	team        *Team  // Its team
	opponent    *Card  // The opposing card
	damageDealt int    // Total damage dealt by this card in this duel
	damageTaken int    // Total damage taken by this card in this duel
	roundsStart int    // Round when this card came to front
	roundsCount int    // Number of rounds in duel
}

// captureCardStates captures the current state of all cards from both teams.
// attackerID and defenderID are used to mark which cards are attacking/defending in this event.
func captureCardStates(teamA, teamB *Team, attackerID, defenderID int64) []CardSnapshot {
	states := make([]CardSnapshot, 0, len(teamA.Cards)+len(teamB.Cards))

	// Capture Team A
	for _, card := range teamA.Cards {
		if card != nil {
			states = append(states, CardSnapshot{
				CardID:      card.CardID,
				UserID:      card.UserID,
				Name:        card.Name,
				HP:          card.HP,
				MaxHP:       card.MaxHP,
				ATK:         card.ATK,
				Position:    card.Position,
				IsAlive:     card.HP > 0,
				IsAttacking: card.CardID == attackerID,
				IsDefending: card.CardID == defenderID,
			})
		}
	}

	// Capture Team B
	for _, card := range teamB.Cards {
		if card != nil {
			states = append(states, CardSnapshot{
				CardID:      card.CardID,
				UserID:      card.UserID,
				Name:        card.Name,
				HP:          card.HP,
				MaxHP:       card.MaxHP,
				ATK:         card.ATK,
				Position:    card.Position,
				IsAlive:     card.HP > 0,
				IsAttacking: card.CardID == attackerID,
				IsDefending: card.CardID == defenderID,
			})
		}
	}

	return states
}

// Simulate runs a battle between two teams and returns the result.
// The battle follows SAP-style sequential combat:
// 1. Front cards attack each other simultaneously
// 2. Damage dealt = attacker's ATK
// 3. Card dies when HP <= 0
// 4. Next card advances to front
// 5. Repeat until one team is empty
// 6. If both teams empty simultaneously, winner is determined by total damage dealt
func Simulate(teamA, teamB *Team) *Result {
	// Clone teams to avoid mutating originals
	a := teamA.Clone()
	b := teamB.Clone()

	events := make([]BattleEvent, 0, 50)
	round := 0
	totalDamageA := 0 // Damage dealt BY team A
	totalDamageB := 0 // Damage dealt BY team B

	// Track duel statistics and kill streaks
	var currentDuelA, currentDuelB *duelStats
	killStreaks := make(map[int64]int) // cardID -> consecutive kills

	for round < MaxRounds {
		round++

		frontA := a.GetFront()
		frontB := b.GetFront()

		// Initialize or update duel stats if cards changed
		if frontA != nil && (currentDuelA == nil || currentDuelA.card.CardID != frontA.CardID) {
			currentDuelA = &duelStats{
				card:        frontA,
				team:        a,
				roundsStart: round,
			}
		}
		if frontB != nil && (currentDuelB == nil || currentDuelB.card.CardID != frontB.CardID) {
			currentDuelB = &duelStats{
				card:        frontB,
				team:        b,
				roundsStart: round,
			}
		}

		// Set opponent references
		if currentDuelA != nil && frontB != nil {
			currentDuelA.opponent = frontB
		}
		if currentDuelB != nil && frontA != nil {
			currentDuelB.opponent = frontA
		}

		// Both teams empty = compare damage
		if frontA == nil && frontB == nil {
			break
		}

		// One team empty = other wins
		if frontA == nil {
			victoryEventB := BattleEvent{
				Type:       EventVictory,
				Round:      round,
				Message:    fmt.Sprintf("🏆 %s wins!", getTeamName(b)),
				CardStates: captureCardStates(a, b, 0, 0),
			}
			events = append(events, victoryEventB)
			winnerID := b.OwnerID
			return &Result{
				WinnerID:    &winnerID,
				IsDraw:      false,
				Events:      events,
				NumRounds:   round,
				TeamADamage: totalDamageA,
				TeamBDamage: totalDamageB,
				TeamAFinal:  a,
				TeamBFinal:  b,
			}
		}
		if frontB == nil {
			victoryEventA := BattleEvent{
				Type:       EventVictory,
				Round:      round,
				Message:    fmt.Sprintf("🏆 %s wins!", getTeamName(a)),
				CardStates: captureCardStates(a, b, 0, 0),
			}
			events = append(events, victoryEventA)
			winnerID := a.OwnerID
			return &Result{
				WinnerID:    &winnerID,
				IsDraw:      false,
				Events:      events,
				NumRounds:   round,
				TeamADamage: totalDamageA,
				TeamBDamage: totalDamageB,
				TeamAFinal:  a,
				TeamBFinal:  b,
			}
		}

		// Simultaneous attacks
		damageToB := frontA.ATK
		damageToA := frontB.ATK

		// Record attack from A to B
		events = append(events, BattleEvent{
			Type:                EventAttack,
			Round:               round,
			AttackerCardID:      frontA.CardID,
			DefenderCardID:      frontB.CardID,
			AttackerTeamOwnerID: a.OwnerID,
			DefenderTeamOwnerID: b.OwnerID,
			Damage:              damageToB,
			HPBefore:            frontB.HP,
			HPAfter:             frontB.HP - damageToB,
			Message:             fmt.Sprintf("🗡️ %s attacks %s (%d ATK) → %s %d HP", frontA.Name, frontB.Name, damageToB, generateHealthBar(frontB.HP-damageToB, frontB.MaxHP), frontB.HP-damageToB),
		})

		// Record attack from B to A
		events = append(events, BattleEvent{
			Type:                EventAttack,
			Round:               round,
			AttackerCardID:      frontB.CardID,
			DefenderCardID:      frontA.CardID,
			AttackerTeamOwnerID: b.OwnerID,
			DefenderTeamOwnerID: a.OwnerID,
			Damage:              damageToA,
			HPBefore:            frontA.HP,
			HPAfter:             frontA.HP - damageToA,
			Message:             fmt.Sprintf("🗡️ %s attacks %s (%d ATK) → %s %d HP", frontB.Name, frontA.Name, damageToA, generateHealthBar(frontA.HP-damageToA, frontA.MaxHP), frontA.HP-damageToA),
		})

		// Apply damage and update duel stats
		frontB.HP -= damageToB
		frontA.HP -= damageToA
		totalDamageA += damageToB
		totalDamageB += damageToA

		if currentDuelA != nil {
			currentDuelA.damageDealt += damageToB
			currentDuelA.damageTaken += damageToA
		}
		if currentDuelB != nil {
			currentDuelB.damageDealt += damageToA
			currentDuelB.damageTaken += damageToB
		}

		// Check deaths
		aDied := frontA.HP <= 0
		bDied := frontB.HP <= 0

	// Add card states to attack events (after damage is applied)
	if len(events) >= 2 {
		events[len(events)-2].CardStates = captureCardStates(a, b, frontA.CardID, frontB.CardID)
		events[len(events)-1].CardStates = captureCardStates(a, b, frontB.CardID, frontA.CardID)
	}

	if aDied {
		deathEventA := BattleEvent{
			Type:                EventDeath,
			Round:               round,
			DefenderCardID:      frontA.CardID,
			DefenderTeamOwnerID: a.OwnerID,
			Message:             fmt.Sprintf("💀 %s defeats %s", frontB.Name, frontA.Name),
			CardStates:          captureCardStates(a, b, frontB.CardID, frontA.CardID),
		}
		events = append(events, deathEventA)

		// Generate summary for the winner (frontB defeated frontA)
		if currentDuelB != nil {
			currentDuelB.roundsCount = round - currentDuelB.roundsStart + 1

			// Update kill streak
			killStreaks[frontB.CardID]++
			streak := killStreaks[frontB.CardID]

			// Reset opponent's kill streak
			killStreaks[frontA.CardID] = 0

			streakMsg := ""
			if streak >= 2 {
				streakMsg = fmt.Sprintf(" 🔥 x%d", streak)
			}

			summaryEventB := BattleEvent{
				Type:             EventSummary,
				Round:            round,
				IsSummary:        true,
				KillerCardID:     &frontB.CardID,
				TotalDamageDealt: currentDuelB.damageDealt,
				TotalDamageTaken: currentDuelB.damageTaken,
				RoundsInDuel:     currentDuelB.roundsCount,
				KillStreak:       streak,
				Message: fmt.Sprintf("⚔️ %s defeats %s | %d rounds | %d dealt / %d taken | %d❤️%s",
					frontB.Name, frontA.Name, currentDuelB.roundsCount,
					currentDuelB.damageDealt, currentDuelB.damageTaken,
					frontB.HP, streakMsg),
				CardStates:       captureCardStates(a, b, frontB.CardID, frontA.CardID),
			}
			events = append(events, summaryEventB)
		}
	}

	if bDied {
		deathEventB := BattleEvent{
			Type:                EventDeath,
			Round:               round,
			DefenderCardID:      frontB.CardID,
			DefenderTeamOwnerID: b.OwnerID,
			Message:             fmt.Sprintf("💀 %s defeats %s", frontA.Name, frontB.Name),
			CardStates:          captureCardStates(a, b, frontA.CardID, frontB.CardID),
		}
		events = append(events, deathEventB)

		// Generate summary for the winner (frontA defeated frontB)
		if currentDuelA != nil {
			currentDuelA.roundsCount = round - currentDuelA.roundsStart + 1

			// Update kill streak
			killStreaks[frontA.CardID]++
			streak := killStreaks[frontA.CardID]

			// Reset opponent's kill streak
			killStreaks[frontB.CardID] = 0

			streakMsg := ""
			if streak >= 2 {
				streakMsg = fmt.Sprintf(" 🔥 x%d", streak)
			}

			summaryEventA := BattleEvent{
				Type:             EventSummary,
				Round:            round,
				IsSummary:        true,
				KillerCardID:     &frontA.CardID,
				TotalDamageDealt: currentDuelA.damageDealt,
				TotalDamageTaken: currentDuelA.damageTaken,
				RoundsInDuel:     currentDuelA.roundsCount,
				KillStreak:       streak,
				Message: fmt.Sprintf("⚔️ %s defeats %s | %d rounds | %d dealt / %d taken | %d❤️%s",
					frontA.Name, frontB.Name, currentDuelA.roundsCount,
					currentDuelA.damageDealt, currentDuelA.damageTaken,
					frontA.HP, streakMsg),
				CardStates:       captureCardStates(a, b, frontA.CardID, frontB.CardID),
			}
			events = append(events, summaryEventA)
		}
	}

	// Advance if cards died
	if aDied || bDied {
		advanceEvent := BattleEvent{
			Type:       EventAdvance,
			Round:      round,
			Message:    "➡️ Next cards advance",
			CardStates: captureCardStates(a, b, 0, 0),
		}
		events = append(events, advanceEvent)
	}
	}

	// Determine winner by damage if both teams exhausted
	if totalDamageA > totalDamageB {
		damageVictoryA := BattleEvent{
			Type:       EventVictory,
			Round:      round,
			Message:    fmt.Sprintf("🏆 %s wins!", getTeamName(a)),
			CardStates: captureCardStates(a, b, 0, 0),
		}
		events = append(events, damageVictoryA)
		winnerID := a.OwnerID
		return &Result{
			WinnerID:    &winnerID,
			IsDraw:      false,
			Events:      events,
			NumRounds:   round,
			TeamADamage: totalDamageA,
			TeamBDamage: totalDamageB,
			TeamAFinal:  a,
			TeamBFinal:  b,
		}
	} else if totalDamageB > totalDamageA {
		damageVictoryB := BattleEvent{
			Type:       EventVictory,
			Round:      round,
			Message:    fmt.Sprintf("🏆 %s wins!", getTeamName(b)),
			CardStates: captureCardStates(a, b, 0, 0),
		}
		events = append(events, damageVictoryB)
		winnerID := b.OwnerID
		return &Result{
			WinnerID:    &winnerID,
			IsDraw:      false,
			Events:      events,
			NumRounds:   round,
			TeamADamage: totalDamageA,
			TeamBDamage: totalDamageB,
			TeamAFinal:  a,
			TeamBFinal:  b,
		}
	}

	// True draw - equal damage
	drawEvent := BattleEvent{
		Type:       EventVictory,
		Round:      round,
		Message:    "🤝 Draw!",
		CardStates: captureCardStates(a, b, 0, 0),
	}
	events = append(events, drawEvent)
	return &Result{
		WinnerID:    nil,
		IsDraw:      true,
		Events:      events,
		NumRounds:   round,
		TeamADamage: totalDamageA,
		TeamBDamage: totalDamageB,
		TeamAFinal:  a,
		TeamBFinal:  b,
	}
}

// generateHealthBar creates a visual health bar representation
// Returns a 10-character unicode health bar (e.g., [████████░░])
func generateHealthBar(currentHP, maxHP int) string {
	if maxHP == 0 {
		return "[░░░░░░░░░░]"
	}

	percentage := float64(currentHP) / float64(maxHP)
	filledBlocks := int(percentage * 10)

	if filledBlocks < 0 {
		filledBlocks = 0
	} else if filledBlocks > 10 {
		filledBlocks = 10
	}

	emptyBlocks := 10 - filledBlocks
	var bar strings.Builder
	bar.WriteRune('[')
	for range filledBlocks {
		bar.WriteRune('█')
	}
	for range emptyBlocks {
		bar.WriteRune('░')
	}
	bar.WriteRune(']')

	return bar.String()
}

// getTeamName returns a display name for the team
func getTeamName(t *Team) string {
	if t == nil {
		return "Team"
	}
	if t.OwnerName != "" {
		return t.OwnerName
	}
	return "Team"
}

// SimulateWithSeed runs a battle with a specific random seed (for testing)
// Currently battles are deterministic, so this just calls Simulate
func SimulateWithSeed(teamA, teamB *Team, seed int64) *Result {
	return Simulate(teamA, teamB)
}
