package shop

import (
	"testing"

	"beef-briefing/apps/api-service/internal/game/battle"
)

// =============================================================================
// Test Fixtures
// =============================================================================

// newTestShopCard creates a ShopCard with the specified stats for testing.
func newTestShopCard(cardID, userID int64, name string, atk, hp int, index int) *battle.ShopCard {
	return &battle.ShopCard{
		CardID:      cardID,
		UserID:      userID,
		Name:        name,
		ATK:         atk,
		HP:          hp,
		Index:       index,
		IsPurchased: false,
	}
}

// newTestShopCards creates a slice of shop cards for testing.
func newTestShopCards() []*battle.ShopCard {
	return []*battle.ShopCard{
		newTestShopCard(101, 1, "Card1", 10, 30, 0),
		newTestShopCard(102, 2, "Card2", 12, 25, 1),
		newTestShopCard(103, 3, "Card3", 8, 35, 2),
		newTestShopCard(104, 4, "Card4", 15, 20, 3),
	}
}

// =============================================================================
// NewShopState Tests
// =============================================================================

func TestNewShopState_InitialValues(t *testing.T) {
	matchID := "test-match-123"
	userID := int64(456)
	cards := newTestShopCards()

	state := NewShopState(matchID, userID, cards)

	if state.MatchID != matchID {
		t.Errorf("expected MatchID=%q, got %q", matchID, state.MatchID)
	}
	if state.UserID != userID {
		t.Errorf("expected UserID=%d, got %d", userID, state.UserID)
	}
	if state.Coins != StartingCoins {
		t.Errorf("expected Coins=%d (StartingCoins), got %d", StartingCoins, state.Coins)
	}
	if len(state.Cards) != len(cards) {
		t.Errorf("expected %d cards, got %d", len(cards), len(state.Cards))
	}
	if len(state.Team) != 0 {
		t.Errorf("expected empty team, got %d cards", len(state.Team))
	}
	if cap(state.Team) != TeamSize {
		t.Errorf("expected team capacity=%d, got %d", TeamSize, cap(state.Team))
	}
	if len(state.TeamOrder) != 3 || state.TeamOrder[0] != 0 || state.TeamOrder[1] != 1 || state.TeamOrder[2] != 2 {
		t.Errorf("expected TeamOrder=[0,1,2], got %v", state.TeamOrder)
	}
	if state.IsReady {
		t.Error("expected IsReady=false")
	}
	if state.RerollsUsed != 0 {
		t.Errorf("expected RerollsUsed=0, got %d", state.RerollsUsed)
	}
}

// =============================================================================
// CanBuy Tests
// =============================================================================

func TestCanBuy_Success(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())

	if !state.CanBuy(0) {
		t.Error("expected CanBuy(0) to return true with full coins and empty team")
	}
	if !state.CanBuy(3) {
		t.Error("expected CanBuy(3) to return true for last card index")
	}
}

func TestCanBuy_InsufficientCoins(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	state.Coins = CardCost - 1 // Not enough coins

	if state.CanBuy(0) {
		t.Errorf("expected CanBuy to return false with %d coins (need %d)", state.Coins, CardCost)
	}
}

func TestCanBuy_TeamFull(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	// Fill the team
	state.Team = []*battle.Card{
		{CardID: 1, HP: 10, MaxHP: 10},
		{CardID: 2, HP: 10, MaxHP: 10},
		{CardID: 3, HP: 10, MaxHP: 10},
	}

	if state.CanBuy(0) {
		t.Error("expected CanBuy to return false when team is full")
	}
}

func TestCanBuy_AlreadyPurchased(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	state.Cards[0].IsPurchased = true

	if state.CanBuy(0) {
		t.Error("expected CanBuy to return false for already purchased card")
	}
}

func TestCanBuy_InvalidIndex(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())

	if state.CanBuy(-1) {
		t.Error("expected CanBuy to return false for negative index")
	}
	if state.CanBuy(len(state.Cards)) {
		t.Error("expected CanBuy to return false for out of bounds index")
	}
	if state.CanBuy(100) {
		t.Error("expected CanBuy to return false for very large index")
	}
}

func TestCanBuy_NilCard(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	state.Cards[0] = nil

	if state.CanBuy(0) {
		t.Error("expected CanBuy to return false for nil card")
	}
}

// =============================================================================
// Buy Tests
// =============================================================================

func TestBuy_DeductsCoinsAndAddsToTeam(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	initialCoins := state.Coins
	cardIndex := 1

	card, err := state.Buy(cardIndex)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card == nil {
		t.Fatal("expected card to be returned")
	}
	if state.Coins != initialCoins-CardCost {
		t.Errorf("expected coins=%d, got %d", initialCoins-CardCost, state.Coins)
	}
	if len(state.Team) != 1 {
		t.Errorf("expected team size=1, got %d", len(state.Team))
	}
	if state.Team[0].CardID != state.Cards[cardIndex].CardID {
		t.Errorf("expected team card ID=%d, got %d", state.Cards[cardIndex].CardID, state.Team[0].CardID)
	}
	if !state.Cards[cardIndex].IsPurchased {
		t.Error("expected card to be marked as purchased")
	}
}

func TestBuy_ReturnsErrorWhenCannotBuy(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	state.Coins = 0

	card, err := state.Buy(0)

	if err != ErrCannotBuy {
		t.Errorf("expected ErrCannotBuy, got %v", err)
	}
	if card != nil {
		t.Error("expected nil card on error")
	}
}

func TestBuy_ConvertsToBattleCard(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	shopCard := state.Cards[0]

	card, err := state.Buy(0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if card.ATK != shopCard.ATK {
		t.Errorf("expected ATK=%d, got %d", shopCard.ATK, card.ATK)
	}
	if card.HP != shopCard.HP {
		t.Errorf("expected HP=%d, got %d", shopCard.HP, card.HP)
	}
	if card.MaxHP != shopCard.HP {
		t.Errorf("expected MaxHP=%d, got %d", shopCard.HP, card.MaxHP)
	}
}

// =============================================================================
// CanReroll Tests
// =============================================================================

func TestCanReroll_BeforePurchase(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())

	if !state.CanReroll() {
		t.Error("expected CanReroll to return true before any purchase")
	}
}

func TestCanReroll_DisabledAfterPurchase(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	_, _ = state.Buy(0) // Purchase a card

	if state.CanReroll() {
		t.Error("expected CanReroll to return false after purchase")
	}
}

func TestCanReroll_InsufficientCoins(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	state.Coins = RerollCost - 1

	if state.CanReroll() {
		t.Errorf("expected CanReroll to return false with %d coins (need %d)", state.Coins, RerollCost)
	}
}

// =============================================================================
// Reroll Tests
// =============================================================================

func TestReroll_ReplacesUnpurchasedCards(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	initialCoins := state.Coins
	oldCardIDs := make([]int64, len(state.Cards))
	for i, c := range state.Cards {
		oldCardIDs[i] = c.CardID
	}

	newCards := []*battle.ShopCard{
		newTestShopCard(201, 10, "NewCard1", 20, 40, 0),
		newTestShopCard(202, 11, "NewCard2", 22, 35, 1),
		newTestShopCard(203, 12, "NewCard3", 18, 45, 2),
		newTestShopCard(204, 13, "NewCard4", 25, 30, 3),
	}

	err := state.Reroll(newCards)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Coins != initialCoins-RerollCost {
		t.Errorf("expected coins=%d, got %d", initialCoins-RerollCost, state.Coins)
	}
	if state.RerollsUsed != 1 {
		t.Errorf("expected RerollsUsed=1, got %d", state.RerollsUsed)
	}

	// Verify cards were replaced
	for i, c := range state.Cards {
		if c.CardID == oldCardIDs[i] {
			t.Errorf("expected card at index %d to be replaced", i)
		}
	}
}

func TestReroll_PreservesPurchasedCards(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	// Mark some cards as purchased (simulating a different scenario)
	// Note: In actual gameplay, you can't reroll after purchasing,
	// but testing the replacement logic if Cards were pre-marked
	state.Cards[1].IsPurchased = true
	purchasedCardID := state.Cards[1].CardID

	// For this test, allow reroll by ensuring Team is empty
	newCards := []*battle.ShopCard{
		newTestShopCard(201, 10, "NewCard1", 20, 40, 0),
		newTestShopCard(202, 11, "NewCard2", 22, 35, 1),
		newTestShopCard(203, 12, "NewCard3", 18, 45, 2),
	}

	err := state.Reroll(newCards)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Purchased card should remain
	if state.Cards[1].CardID != purchasedCardID {
		t.Errorf("expected purchased card to remain, but CardID changed from %d to %d", purchasedCardID, state.Cards[1].CardID)
	}
}

func TestReroll_ReturnsErrorWhenCannotReroll(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	_, _ = state.Buy(0) // Purchase disables reroll

	newCards := []*battle.ShopCard{
		newTestShopCard(201, 10, "NewCard1", 20, 40, 0),
	}

	err := state.Reroll(newCards)

	if err != ErrCannotReroll {
		t.Errorf("expected ErrCannotReroll, got %v", err)
	}
}

// =============================================================================
// CanUpgrade Tests
// =============================================================================

func TestCanUpgrade_ReservesCoinsForTeam(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	// Buy one card, leaving 2 more to buy
	_, _ = state.Buy(0)

	// With 1 card bought (cost 3), we have 7 coins left
	// Need 6 coins for 2 more cards (2*3=6), plus 1 for upgrade
	// 7 >= 6+1, so upgrade should be allowed
	if !state.CanUpgrade(0) {
		t.Error("expected CanUpgrade to return true when coins cover remaining cards + upgrade")
	}

	// Set coins so we can buy remaining cards but not upgrade
	state.Coins = 6 // Exactly enough for 2 cards
	if state.CanUpgrade(0) {
		t.Error("expected CanUpgrade to return false when coins only cover remaining cards")
	}
}

func TestCanUpgrade_InvalidSlot(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	_, _ = state.Buy(0) // Have one card in team at slot 0

	if state.CanUpgrade(-1) {
		t.Error("expected CanUpgrade to return false for negative slot")
	}
	if state.CanUpgrade(1) {
		t.Error("expected CanUpgrade to return false for slot beyond team size")
	}
}

func TestCanUpgrade_NilCardInSlot(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	state.Team = []*battle.Card{nil}

	if state.CanUpgrade(0) {
		t.Error("expected CanUpgrade to return false for nil card in slot")
	}
}

// =============================================================================
// Upgrade Tests
// =============================================================================

func TestUpgrade_ATK_Increases(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	_, _ = state.Buy(0)
	initialATK := state.Team[0].ATK
	initialCoins := state.Coins

	err := state.Upgrade(0, UpgradeATK)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Team[0].ATK != initialATK+ATKUpgradeAmount {
		t.Errorf("expected ATK=%d, got %d", initialATK+ATKUpgradeAmount, state.Team[0].ATK)
	}
	if state.Team[0].ATKUpgrades != 1 {
		t.Errorf("expected ATKUpgrades=1, got %d", state.Team[0].ATKUpgrades)
	}
	if state.Coins != initialCoins-UpgradeCost {
		t.Errorf("expected coins=%d, got %d", initialCoins-UpgradeCost, state.Coins)
	}
}

func TestUpgrade_HP_IncreasesHPAndMaxHP(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	_, _ = state.Buy(0)
	initialHP := state.Team[0].HP
	initialMaxHP := state.Team[0].MaxHP
	initialCoins := state.Coins

	err := state.Upgrade(0, UpgradeHP)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Team[0].HP != initialHP+HPUpgradeAmount {
		t.Errorf("expected HP=%d, got %d", initialHP+HPUpgradeAmount, state.Team[0].HP)
	}
	if state.Team[0].MaxHP != initialMaxHP+HPUpgradeAmount {
		t.Errorf("expected MaxHP=%d, got %d", initialMaxHP+HPUpgradeAmount, state.Team[0].MaxHP)
	}
	if state.Team[0].HPUpgrades != 1 {
		t.Errorf("expected HPUpgrades=1, got %d", state.Team[0].HPUpgrades)
	}
	if state.Coins != initialCoins-UpgradeCost {
		t.Errorf("expected coins=%d, got %d", initialCoins-UpgradeCost, state.Coins)
	}
}

func TestUpgrade_InvalidType(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	_, _ = state.Buy(0)

	err := state.Upgrade(0, "invalid")

	if err != ErrInvalidUpgradeType {
		t.Errorf("expected ErrInvalidUpgradeType, got %v", err)
	}
}

func TestUpgrade_ReturnsErrorWhenCannotUpgrade(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	_, _ = state.Buy(0)
	state.Coins = 0 // No coins for upgrade

	err := state.Upgrade(0, UpgradeATK)

	if err != ErrCannotUpgrade {
		t.Errorf("expected ErrCannotUpgrade, got %v", err)
	}
}

// =============================================================================
// SetOrder Tests
// =============================================================================

func TestSetOrder_ValidOrder(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	// Buy 3 cards to have a full team
	_, _ = state.Buy(0)
	_, _ = state.Buy(1)
	_, _ = state.Buy(2)

	err := state.SetOrder([]int{2, 0, 1})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.TeamOrder[0] != 2 || state.TeamOrder[1] != 0 || state.TeamOrder[2] != 1 {
		t.Errorf("expected TeamOrder=[2,0,1], got %v", state.TeamOrder)
	}
}

func TestSetOrder_DuplicateIndices_Error(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	_, _ = state.Buy(0)
	_, _ = state.Buy(1)
	_, _ = state.Buy(2)

	err := state.SetOrder([]int{0, 0, 1})

	if err != ErrInvalidOrder {
		t.Errorf("expected ErrInvalidOrder for duplicate indices, got %v", err)
	}
}

func TestSetOrder_WrongLength(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	_, _ = state.Buy(0)
	_, _ = state.Buy(1)

	err := state.SetOrder([]int{0, 1, 2}) // Team has 2 cards, order has 3

	if err != ErrInvalidOrder {
		t.Errorf("expected ErrInvalidOrder for wrong length, got %v", err)
	}
}

func TestSetOrder_InvalidIndex(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	_, _ = state.Buy(0)
	_, _ = state.Buy(1)
	_, _ = state.Buy(2)

	err := state.SetOrder([]int{0, 1, 5}) // 5 is out of bounds

	if err != ErrInvalidOrder {
		t.Errorf("expected ErrInvalidOrder for out of bounds index, got %v", err)
	}
}

func TestSetOrder_NegativeIndex(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	_, _ = state.Buy(0)
	_, _ = state.Buy(1)
	_, _ = state.Buy(2)

	err := state.SetOrder([]int{0, -1, 2})

	if err != ErrInvalidOrder {
		t.Errorf("expected ErrInvalidOrder for negative index, got %v", err)
	}
}

// =============================================================================
// GetBattleTeam Tests
// =============================================================================

func TestGetBattleTeam_RespectsOrder(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	_, _ = state.Buy(0)
	_, _ = state.Buy(1)
	_, _ = state.Buy(2)

	// Set order: slot 2 first, then 0, then 1
	_ = state.SetOrder([]int{2, 0, 1})

	team := state.GetBattleTeam("TestOwner")

	if team == nil {
		t.Fatal("expected team to be returned")
	}
	if team.OwnerID != state.UserID {
		t.Errorf("expected OwnerID=%d, got %d", state.UserID, team.OwnerID)
	}
	if team.OwnerName != "TestOwner" {
		t.Errorf("expected OwnerName=TestOwner, got %s", team.OwnerName)
	}
	if len(team.Cards) != TeamSize {
		t.Errorf("expected %d cards, got %d", TeamSize, len(team.Cards))
	}
	// Verify order: position 0 should have the card from team slot 2
	if team.Cards[0].CardID != state.Team[2].CardID {
		t.Errorf("expected front card ID=%d (from slot 2), got %d", state.Team[2].CardID, team.Cards[0].CardID)
	}
	if team.Cards[1].CardID != state.Team[0].CardID {
		t.Errorf("expected mid card ID=%d (from slot 0), got %d", state.Team[0].CardID, team.Cards[1].CardID)
	}
	if team.Cards[2].CardID != state.Team[1].CardID {
		t.Errorf("expected back card ID=%d (from slot 1), got %d", state.Team[1].CardID, team.Cards[2].CardID)
	}
}

func TestGetBattleTeam_SetsPositions(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	_, _ = state.Buy(0)
	_, _ = state.Buy(1)
	_, _ = state.Buy(2)

	team := state.GetBattleTeam("TestOwner")

	for i, card := range team.Cards {
		if card.Position != i {
			t.Errorf("expected card at index %d to have Position=%d, got %d", i, i, card.Position)
		}
	}
}

func TestGetBattleTeam_ReturnsNilIfIncomplete(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	_, _ = state.Buy(0)
	_, _ = state.Buy(1)
	// Only 2 cards, not 3

	team := state.GetBattleTeam("TestOwner")

	if team != nil {
		t.Error("expected nil team for incomplete lineup")
	}
}

func TestGetBattleTeam_ClonesCards(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	_, _ = state.Buy(0)
	_, _ = state.Buy(1)
	_, _ = state.Buy(2)

	team := state.GetBattleTeam("TestOwner")
	originalHP := state.Team[0].HP
	team.Cards[1].HP = 0 // Modify the cloned card (position 1 maps to slot 0 by default)

	// Original should be unchanged
	if state.Team[0].HP != originalHP {
		t.Errorf("expected original card HP to remain %d, got %d", originalHP, state.Team[0].HP)
	}
}

// =============================================================================
// CanSubmit Tests
// =============================================================================

func TestCanSubmit_TeamComplete(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	_, _ = state.Buy(0)
	_, _ = state.Buy(1)
	_, _ = state.Buy(2)

	if !state.CanSubmit() {
		t.Error("expected CanSubmit to return true with full team")
	}
}

func TestCanSubmit_TeamIncomplete(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	_, _ = state.Buy(0)
	_, _ = state.Buy(1)

	if state.CanSubmit() {
		t.Error("expected CanSubmit to return false with incomplete team")
	}
}

// =============================================================================
// Submit Tests
// =============================================================================

func TestSubmit_MarksReady(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	_, _ = state.Buy(0)
	_, _ = state.Buy(1)
	_, _ = state.Buy(2)

	err := state.Submit()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !state.IsReady {
		t.Error("expected IsReady=true after submit")
	}
}

func TestSubmit_ReturnsErrorIfIncomplete(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	_, _ = state.Buy(0)

	err := state.Submit()

	if err != ErrTeamIncomplete {
		t.Errorf("expected ErrTeamIncomplete, got %v", err)
	}
	if state.IsReady {
		t.Error("expected IsReady=false when submit fails")
	}
}

// =============================================================================
// JSON Serialization Tests
// =============================================================================

func TestToJSON_ReturnsValidJSON(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	_, _ = state.Buy(0)

	data, err := state.ToJSON()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON data")
	}
}

func TestTeamToJSON_ReturnsValidJSON(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())
	_, _ = state.Buy(0)

	data, err := state.TeamToJSON()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON data")
	}
}

func TestShopCardsToJSON_ReturnsValidJSON(t *testing.T) {
	state := NewShopState("match-1", 1, newTestShopCards())

	data, err := state.ShopCardsToJSON()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON data")
	}
}
