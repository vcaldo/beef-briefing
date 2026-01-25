package battle

// GroupEventsIntoCombats takes a flat list of BattleEvents and groups them
// into Combats. Each combat represents a duel between two cards until one
// or both die.
//
// Algorithm:
// 1. Iterate through events in order
// 2. Add events to current combat
// 3. When an advance event occurs, finalize current combat and start new one
// 4. Track deaths to set combat outcome
// 5. Victory event ends the final combat
func GroupEventsIntoCombats(events []BattleEvent, playerAID, playerBID int64) []Combat {
	if len(events) == 0 {
		return []Combat{}
	}

	var combats []Combat
	var currentCombat *Combat
	combatNumber := 0

	for i, event := range events {
		// Start new combat if needed
		if currentCombat == nil {
			combatNumber++
			currentCombat = &Combat{
				CombatNumber: combatNumber,
				Events:       make([]CombatEvent, 0),
			}
		}

		// Add event to current combat
		currentCombat.Events = append(currentCombat.Events, event.ToCombatEvent())

		// Check if combat ends or track outcome
		switch event.Type {
		case EventDeath:
			// Track who died to set outcome
			if event.DefenderTeamOwnerID == playerAID {
				if currentCombat.Outcome == "" {
					currentCombat.Outcome = "card_a_died"
				} else if currentCombat.Outcome == "card_b_died" {
					currentCombat.Outcome = "both_died"
				}
			} else {
				if currentCombat.Outcome == "" {
					currentCombat.Outcome = "card_b_died"
				} else if currentCombat.Outcome == "card_a_died" {
					currentCombat.Outcome = "both_died"
				}
			}

		case EventAdvance:
			// Advance ends the current combat - finalize and start new
			combats = append(combats, *currentCombat)
			currentCombat = nil

		case EventVictory:
			// Victory ends everything
			if currentCombat.Outcome == "" {
				currentCombat.Outcome = "victory"
			}
			combats = append(combats, *currentCombat)
			currentCombat = nil
		}

		// Handle case where we're at the last event without explicit finalization
		if i == len(events)-1 && currentCombat != nil {
			combats = append(combats, *currentCombat)
		}
	}

	return combats
}
