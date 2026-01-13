package battle

import "encoding/json"

// EventType represents the type of battle event for replay
type EventType string

const (
	EventAttack   EventType = "attack"
	EventDamage   EventType = "damage"
	EventDeath    EventType = "death"
	EventAdvance  EventType = "advance"
	EventVictory  EventType = "victory"
	EventSummary  EventType = "summary"
)

// Card represents a card in battle with current state
type Card struct {
	CardID   int64  `json:"card_id"`
	UserID   int64  `json:"user_id"`
	Name     string `json:"name"`      // First name of the user
	Username string `json:"username"`  // Optional username
	PhotoURL string `json:"photo_url"` // Profile photo URL

	// Combat stats (base + upgrades)
	ATK   int `json:"atk"`
	DEF   int `json:"def"` // Used for display, not combat
	HP    int `json:"hp"`
	MaxHP int `json:"max_hp"`

	// Upgrades applied
	ATKUpgrades int `json:"atk_upgrades"`
	HPUpgrades  int `json:"hp_upgrades"`

	// Position in team (0=front, 1=mid, 2=back)
	Position int `json:"position"`
}

// Clone creates a deep copy of the card
func (c *Card) Clone() *Card {
	if c == nil {
		return nil
	}
	clone := *c
	return &clone
}

// Team represents a player's 3-card lineup
type Team struct {
	OwnerID   int64   `json:"owner_id"`
	OwnerName string  `json:"owner_name"` // Player name (first_name or username)
	Cards     []*Card `json:"cards"`      // Ordered: [0]=front, [1]=mid, [2]=back
}

// NewTeam creates a team from cards
func NewTeam(ownerID int64, ownerName string, cards []*Card) *Team {
	team := &Team{
		OwnerID:   ownerID,
		OwnerName: ownerName,
		Cards:     make([]*Card, len(cards)),
	}
	for i, c := range cards {
		if c != nil {
			clone := c.Clone()
			clone.Position = i
			team.Cards[i] = clone
		}
	}
	return team
}

// Clone creates a deep copy of the team
func (t *Team) Clone() *Team {
	clone := &Team{
		OwnerID:   t.OwnerID,
		OwnerName: t.OwnerName,
		Cards:     make([]*Card, len(t.Cards)),
	}
	for i, c := range t.Cards {
		clone.Cards[i] = c.Clone()
	}
	return clone
}

// GetFront returns the frontmost living card (nil if all dead)
func (t *Team) GetFront() *Card {
	for _, card := range t.Cards {
		if card != nil && card.HP > 0 {
			return card
		}
	}
	return nil
}

// RemoveFront marks the front card as dead and shifts positions
func (t *Team) RemoveFront() {
	for i, card := range t.Cards {
		if card != nil && card.HP > 0 {
			t.Cards[i].HP = 0 // Mark as dead
			return
		}
	}
}

// RemainingCards returns count of cards with HP > 0
func (t *Team) RemainingCards() int {
	count := 0
	for _, card := range t.Cards {
		if card != nil && card.HP > 0 {
			count++
		}
	}
	return count
}

// IsEmpty returns true if no cards remain alive
func (t *Team) IsEmpty() bool {
	return t.RemainingCards() == 0
}

// CardSnapshot represents the state of a card at a point in time during battle
type CardSnapshot struct {
	CardID      int64  `json:"card_id"`
	UserID      int64  `json:"user_id"`
	Name        string `json:"name"`
	HP          int    `json:"hp"`
	MaxHP       int    `json:"max_hp"`
	ATK         int    `json:"atk"`
	Position    int    `json:"position"` // 0=front, 1=mid, 2=back
	IsAlive     bool   `json:"is_alive"`
	IsAttacking bool   `json:"is_attacking"`
	IsDefending bool   `json:"is_defending"`
}

// BattleEvent represents a single action in battle replay
type BattleEvent struct {
	Type    EventType `json:"type"`
	Round   int       `json:"round"`
	Message string    `json:"message,omitempty"`

	// Card-specific identifiers - uniquely identify which card instance participated
	AttackerCardID      int64 `json:"attacker_card_id,omitempty"`
	DefenderCardID      int64 `json:"defender_card_id,omitempty"`
	AttackerTeamOwnerID int64 `json:"attacker_team_owner_id,omitempty"`
	DefenderTeamOwnerID int64 `json:"defender_team_owner_id,omitempty"`

	// Combat stats
	Damage   int `json:"damage,omitempty"`
	HPBefore int `json:"hp_before,omitempty"`
	HPAfter  int `json:"hp_after,omitempty"`

	// Summary event fields (for consolidated card duel outcomes)
	IsSummary         bool  `json:"is_summary,omitempty"`
	KillerCardID      *int64 `json:"killer_card_id,omitempty"`
	TotalDamageDealt  int   `json:"total_damage_dealt,omitempty"`
	TotalDamageTaken  int   `json:"total_damage_taken,omitempty"`
	RoundsInDuel      int   `json:"rounds_in_duel,omitempty"`
	KillStreak        int   `json:"kill_streak,omitempty"`

	// Card states snapshot - shows all cards' states after this event
	CardStates []CardSnapshot `json:"card_states,omitempty"`
}

// Result represents the outcome of a battle
type Result struct {
	WinnerID  *int64        `json:"winner_id"` // nil for draw
	IsDraw    bool          `json:"is_draw"`
	Events    []BattleEvent `json:"events"`
	NumRounds int           `json:"num_rounds"`

	// Damage stats
	TeamADamage int `json:"team_a_damage"`
	TeamBDamage int `json:"team_b_damage"`

	// Final team states (for display)
	TeamAFinal *Team `json:"team_a_final"`
	TeamBFinal *Team `json:"team_b_final"`
}

// ToJSON serializes the result to JSON
func (r *Result) ToJSON() (json.RawMessage, error) {
	return json.Marshal(r)
}

// EventsToJSON serializes just the events for storage
func (r *Result) EventsToJSON() (json.RawMessage, error) {
	return json.Marshal(r.Events)
}

// ShopCard represents a card available in the shop
type ShopCard struct {
	CardID   int64  `json:"card_id"`
	UserID   int64  `json:"user_id"`
	Name     string `json:"name"`
	Username string `json:"username,omitempty"`
	PhotoURL string `json:"photo_url,omitempty"`

	// Card image URL for shop display (overrides PhotoURL for shop phase)
	CardImageURL string `json:"card_image_url,omitempty"`

	// Base stats from ml_user_cards
	ATK int `json:"atk"`
	DEF int `json:"def"`
	HP  int `json:"hp"`

	// Full stats for display (optional)
	Stats json.RawMessage `json:"stats,omitempty"`

	// Shop state
	IsPurchased bool `json:"is_purchased"`
	Index       int  `json:"index"` // Position in shop (0-5)
}

// ToCard converts a ShopCard to a battle Card
func (sc *ShopCard) ToCard() *Card {
	return &Card{
		CardID:      sc.CardID,
		UserID:      sc.UserID,
		Name:        sc.Name,
		Username:    sc.Username,
		PhotoURL:    sc.PhotoURL,
		ATK:         sc.ATK,
		DEF:         sc.DEF,
		HP:          sc.HP,
		MaxHP:       sc.HP,
		ATKUpgrades: 0,
		HPUpgrades:  0,
	}
}

// TeamCard represents a card in the player's team (with upgrades)
type TeamCard struct {
	Card
	SlotIndex int `json:"slot_index"` // 0, 1, or 2
}
