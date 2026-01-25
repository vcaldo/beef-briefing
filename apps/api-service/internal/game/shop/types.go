package shop

import (
	"encoding/json"

	"beef-briefing/apps/api-service/internal/game/battle"
	"beef-briefing/apps/api-service/internal/jsonutil"
)

// Economy constants
const (
	StartingCoins    = 10
	CardCost         = 3
	RerollCost       = 1
	UpgradeCost      = 1
	ShopSize         = 4
	TeamSize         = 3
	ATKUpgradeAmount = 1
	HPUpgradeAmount  = 3
)

// ShopState represents the current state of a player's shop
type ShopState struct {
	MatchID     string             `json:"match_id"`
	UserID      int64              `json:"user_id"`
	Coins       int                `json:"coins"`
	Cards       []*battle.ShopCard `json:"cards"`      // Available cards in shop
	Team        []*battle.Card     `json:"team"`       // Purchased cards (up to 3)
	TeamOrder   []int              `json:"team_order"` // Order for battle [front, mid, back]
	IsReady     bool               `json:"is_ready"`
	RerollsUsed int                `json:"rerolls_used"`
}

// NewShopState creates a new shop state for a player
func NewShopState(matchID string, userID int64, cards []*battle.ShopCard) *ShopState {
	return &ShopState{
		MatchID:     matchID,
		UserID:      userID,
		Coins:       StartingCoins,
		Cards:       cards,
		Team:        make([]*battle.Card, 0, TeamSize),
		TeamOrder:   []int{0, 1, 2},
		IsReady:     false,
		RerollsUsed: 0,
	}
}

// CanBuy checks if a card can be purchased
func (s *ShopState) CanBuy(cardIndex int) bool {
	if s.Coins < CardCost {
		return false
	}
	if len(s.Team) >= TeamSize {
		return false
	}
	if cardIndex < 0 || cardIndex >= len(s.Cards) {
		return false
	}
	if s.Cards[cardIndex] == nil || s.Cards[cardIndex].IsPurchased {
		return false
	}
	return true
}

// CanReroll checks if a reroll is possible. Rerolls are only allowed before any card is purchased.
func (s *ShopState) CanReroll() bool {
	// Can only reroll if no cards purchased yet
	if len(s.Team) > 0 {
		return false
	}
	return s.Coins >= RerollCost
}

// CanUpgrade checks if an upgrade is possible while ensuring enough coins remain to complete the team
func (s *ShopState) CanUpgrade(teamSlot int) bool {
	remainingCards := TeamSize - len(s.Team)
	coinsNeededForCards := remainingCards * CardCost
	if s.Coins < (UpgradeCost + coinsNeededForCards) {
		return false
	}
	if teamSlot < 0 || teamSlot >= len(s.Team) {
		return false
	}
	return s.Team[teamSlot] != nil
}

// CanSubmit checks if the team can be submitted
func (s *ShopState) CanSubmit() bool {
	return len(s.Team) == TeamSize
}

// Buy purchases a card from the shop
func (s *ShopState) Buy(cardIndex int) (*battle.Card, error) {
	if !s.CanBuy(cardIndex) {
		return nil, ErrCannotBuy
	}

	shopCard := s.Cards[cardIndex]
	shopCard.IsPurchased = true
	s.Coins -= CardCost

	// Convert to battle card and add to team
	card := shopCard.ToCard()
	s.Team = append(s.Team, card)

	return card, nil
}

// Reroll replaces unpurchased cards with new ones
// The caller must provide new cards to replace with
func (s *ShopState) Reroll(newCards []*battle.ShopCard) error {
	if !s.CanReroll() {
		return ErrCannotReroll
	}

	s.Coins -= RerollCost
	s.RerollsUsed++

	// Replace unpurchased cards
	newCardIdx := 0
	for i, card := range s.Cards {
		if card != nil && !card.IsPurchased {
			if newCardIdx < len(newCards) {
				newCards[newCardIdx].Index = i
				s.Cards[i] = newCards[newCardIdx]
				newCardIdx++
			}
		}
	}

	return nil
}

// Upgrade applies an upgrade to a team card
type UpgradeType string

const (
	UpgradeATK UpgradeType = "atk"
	UpgradeHP  UpgradeType = "hp"
)

func (s *ShopState) Upgrade(teamSlot int, upgradeType UpgradeType) error {
	if !s.CanUpgrade(teamSlot) {
		return ErrCannotUpgrade
	}

	card := s.Team[teamSlot]
	s.Coins -= UpgradeCost

	switch upgradeType {
	case UpgradeATK:
		card.ATK += ATKUpgradeAmount
		card.ATKUpgrades++
	case UpgradeHP:
		card.HP += HPUpgradeAmount
		card.MaxHP += HPUpgradeAmount
		card.HPUpgrades++
	default:
		return ErrInvalidUpgradeType
	}

	return nil
}

// SetOrder sets the battle order for the team
func (s *ShopState) SetOrder(order []int) error {
	if len(order) != len(s.Team) {
		return ErrInvalidOrder
	}

	// Validate order contains valid indices
	seen := make(map[int]bool)
	for _, idx := range order {
		if idx < 0 || idx >= len(s.Team) {
			return ErrInvalidOrder
		}
		if seen[idx] {
			return ErrInvalidOrder
		}
		seen[idx] = true
	}

	s.TeamOrder = order
	return nil
}

// GetBattleTeam returns the team in battle order with the given owner name
func (s *ShopState) GetBattleTeam(ownerName string) *battle.Team {
	if len(s.Team) != TeamSize {
		return nil
	}

	cards := make([]*battle.Card, TeamSize)
	for i, orderIdx := range s.TeamOrder {
		if orderIdx >= 0 && orderIdx < len(s.Team) {
			cards[i] = s.Team[orderIdx].Clone()
			cards[i].Position = i
		}
	}

	return battle.NewTeam(s.UserID, ownerName, cards)
}

// Submit marks the team as ready for battle
func (s *ShopState) Submit() error {
	if !s.CanSubmit() {
		return ErrTeamIncomplete
	}
	s.IsReady = true
	return nil
}

// ToJSON serializes the shop state
func (s *ShopState) ToJSON() (json.RawMessage, error) {
	return jsonutil.Marshal(s)
}

// TeamToJSON serializes just the team for database storage
func (s *ShopState) TeamToJSON() (json.RawMessage, error) {
	return jsonutil.Marshal(s.Team)
}

// ShopCardsToJSON serializes the shop cards for database storage
func (s *ShopState) ShopCardsToJSON() (json.RawMessage, error) {
	return jsonutil.Marshal(s.Cards)
}
