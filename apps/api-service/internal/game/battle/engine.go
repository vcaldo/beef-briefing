package battle

import "fmt"

const (
	// MaxRounds prevents infinite loops
	MaxRounds = 100
)

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

	for round < MaxRounds {
		round++

		frontA := a.GetFront()
		frontB := b.GetFront()

		// Both teams empty = compare damage
		if frontA == nil && frontB == nil {
			break
		}

		// One team empty = other wins
		if frontA == nil {
			events = append(events, BattleEvent{
				Type:    EventVictory,
				Round:   round,
				Message: fmt.Sprintf("%s wins - opponent has no cards remaining", getTeamName(b)),
			})
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
			events = append(events, BattleEvent{
				Type:    EventVictory,
				Round:   round,
				Message: fmt.Sprintf("%s wins - opponent has no cards remaining", getTeamName(a)),
			})
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
			Message:             fmt.Sprintf("%s attacks %s for %d damage", frontA.Name, frontB.Name, damageToB),
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
			Message:             fmt.Sprintf("%s attacks %s for %d damage", frontB.Name, frontA.Name, damageToA),
		})

		// Apply damage
		frontB.HP -= damageToB
		frontA.HP -= damageToA
		totalDamageA += damageToB
		totalDamageB += damageToA

		// Check deaths
		aDied := frontA.HP <= 0
		bDied := frontB.HP <= 0

		if aDied {
			events = append(events, BattleEvent{
				Type:                EventDeath,
				Round:               round,
				DefenderCardID:      frontA.CardID,
				DefenderTeamOwnerID: a.OwnerID,
				Message:             fmt.Sprintf("%s has been defeated!", frontA.Name),
			})
		}

		if bDied {
			events = append(events, BattleEvent{
				Type:                EventDeath,
				Round:               round,
				DefenderCardID:      frontB.CardID,
				DefenderTeamOwnerID: b.OwnerID,
				Message:             fmt.Sprintf("%s has been defeated!", frontB.Name),
			})
		}

		// Advance if cards died
		if aDied || bDied {
			events = append(events, BattleEvent{
				Type:    EventAdvance,
				Round:   round,
				Message: "Next cards advance to the front",
			})
		}
	}

	// Determine winner by damage if both teams exhausted
	if totalDamageA > totalDamageB {
		events = append(events, BattleEvent{
			Type:    EventVictory,
			Round:   round,
			Message: fmt.Sprintf("%s wins by dealing more damage (%d vs %d)", getTeamName(a), totalDamageA, totalDamageB),
		})
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
		events = append(events, BattleEvent{
			Type:    EventVictory,
			Round:   round,
			Message: fmt.Sprintf("%s wins by dealing more damage (%d vs %d)", getTeamName(b), totalDamageB, totalDamageA),
		})
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
	events = append(events, BattleEvent{
		Type:    EventVictory,
		Round:   round,
		Message: fmt.Sprintf("Draw! Both teams dealt %d damage", totalDamageA),
	})
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

// getTeamName returns a display name for the team
func getTeamName(t *Team) string {
	if t == nil || len(t.Cards) == 0 {
		return "Team"
	}
	// Use first card's name as team identifier
	for _, card := range t.Cards {
		if card != nil {
			return fmt.Sprintf("%s's team", card.Name)
		}
	}
	return "Team"
}

// SimulateWithSeed runs a battle with a specific random seed (for testing)
// Currently battles are deterministic, so this just calls Simulate
func SimulateWithSeed(teamA, teamB *Team, seed int64) *Result {
	return Simulate(teamA, teamB)
}
