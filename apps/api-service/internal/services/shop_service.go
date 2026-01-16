package services

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"beef-briefing/apps/api-service/internal/apperror"
	"beef-briefing/apps/api-service/internal/game/battle"
	"beef-briefing/apps/api-service/internal/game/shop"
	"beef-briefing/apps/api-service/internal/jsonutil"
	"beef-briefing/apps/api-service/internal/nrutil"
	"beef-briefing/apps/api-service/internal/repository"

	"github.com/newrelic/go-agent/v3/newrelic"
)

// ShopService handles arena shop phase operations (buying cards, rerolling, upgrading, submitting teams).
// Extracted from ArenaService to improve maintainability and reduce file size.
type ShopService struct {
	db       *sql.DB
	gameRepo repository.GameRepositoryInterface
	dealer   *shop.Dealer
	nrApp    *newrelic.Application
}

// NewShopService creates a new ShopService instance.
func NewShopService(db *sql.DB, gameRepo repository.GameRepositoryInterface, dealer *shop.Dealer, nrApp *newrelic.Application) *ShopService {
	return &ShopService{
		db:       db,
		gameRepo: gameRepo,
		dealer:   dealer,
		nrApp:    nrApp,
	}
}

// validateShopPhaseAccess validates that a user can access the shop for a match.
// It checks: match exists, match is in shop phase, deadline not expired, user is participant, not already submitted.
// Set checkDeadline to false to skip deadline validation (for SetTeamOrder which allows after deadline).
func (s *ShopService) validateShopPhaseAccess(ctx context.Context, matchID string, userID int64, checkDeadline bool) (*repository.Match, *repository.Participant, error) {
	match, err := s.gameRepo.GetMatch(ctx, matchID)
	if err != nil {
		if txn := newrelic.FromContext(ctx); txn != nil {
			txn.NoticeError(err)
		}
		return nil, nil, fmt.Errorf("failed to get match: %w", err)
	}
	if match == nil {
		err := apperror.ErrMatchNotFound
		if txn := newrelic.FromContext(ctx); txn != nil {
			txn.NoticeError(err)
		}
		return nil, nil, err
	}
	if match.Status != repository.MatchStatusShopPhase {
		err := apperror.ErrMatchNotInShopPhase
		if txn := newrelic.FromContext(ctx); txn != nil {
			txn.NoticeError(err)
		}
		return nil, nil, err
	}
	if checkDeadline && match.ShopPhaseDeadline != nil && time.Now().After(*match.ShopPhaseDeadline) {
		err := apperror.ErrShopPhaseExpired
		if txn := newrelic.FromContext(ctx); txn != nil {
			txn.NoticeError(err)
		}
		return nil, nil, err
	}

	participant, err := s.gameRepo.GetParticipant(ctx, matchID, userID)
	if err != nil {
		if txn := newrelic.FromContext(ctx); txn != nil {
			txn.NoticeError(err)
		}
		return nil, nil, fmt.Errorf("failed to get participant: %w", err)
	}
	if participant == nil {
		err := apperror.ErrNotParticipant
		if txn := newrelic.FromContext(ctx); txn != nil {
			txn.NoticeError(err)
		}
		return nil, nil, err
	}
	if participant.Status == repository.ParticipantStatusReady {
		err := apperror.ErrTeamAlreadySubmitted
		if txn := newrelic.FromContext(ctx); txn != nil {
			txn.NoticeError(err)
		}
		return nil, nil, err
	}

	return match, participant, nil
}

// parseParticipantShopState extracts cards and team from participant JSON fields.
// Returns empty slices if fields are nil (not an error condition).
func parseParticipantShopState(participant *repository.Participant) (cards []*battle.ShopCard, team []*battle.Card, err error) {
	if participant.ShopCards != nil {
		if err := jsonutil.Unmarshal(*participant.ShopCards, &cards); err != nil {
			return nil, nil, fmt.Errorf("failed to parse shop cards: %w", err)
		}
	}
	if participant.Team != nil {
		if err := jsonutil.Unmarshal(*participant.Team, &team); err != nil {
			return nil, nil, fmt.Errorf("failed to parse team: %w", err)
		}
	}
	return cards, team, nil
}

// recordCardTransaction records a custom event for card purchases, rerolls, and upgrades.
func recordCardTransaction(nrApp *newrelic.Application, eventType string, matchID string, userID int64, coinsSpent int, cardATK, cardHP int) {
	if nrApp == nil {
		return
	}

	params := map[string]interface{}{
		"event_type":  eventType,
		"match_id":    matchID,
		"user_id":     userID,
		"coins_spent": coinsSpent,
	}

	if cardATK > 0 {
		params["card_atk"] = cardATK
	}
	if cardHP > 0 {
		params["card_hp"] = cardHP
	}

	nrApp.RecordCustomEvent("arena.card.transaction", params)
}

// teamOrderToInt converts int64 team order to int for JSON response.
func teamOrderToInt(order []int64) []int {
	result := make([]int, len(order))
	for i, v := range order {
		result[i] = int(v)
	}
	return result
}

// computeShopAffordability calculates which actions are affordable (buy, reroll, upgrade, submit).
func (s *ShopService) computeShopAffordability(coins int, teamSize int, isReady bool) ShopAffordability {
	remainingCards := shop.TeamSize - teamSize
	coinsNeededForCards := remainingCards * shop.CardCost

	canBuy := coins >= shop.CardCost && teamSize < shop.TeamSize
	canReroll := teamSize == 0 && coins >= shop.RerollCost
	canUpgrade := coins >= (shop.UpgradeCost + coinsNeededForCards)
	canSubmit := teamSize == shop.TeamSize && !isReady

	aff := ShopAffordability{
		CanBuy:     canBuy,
		CanReroll:  canReroll,
		CanUpgrade: canUpgrade,
		CanSubmit:  canSubmit,
	}

	if !canBuy {
		if teamSize >= shop.TeamSize {
			reason := "team is full"
			aff.BuyDisabledReason = &reason
		} else {
			reason := fmt.Sprintf("need %d coins", shop.CardCost)
			aff.BuyDisabledReason = &reason
		}
	}

	if !canReroll {
		var reason string
		if teamSize > 0 {
			reason = "cannot reroll after purchasing cards"
		} else {
			reason = fmt.Sprintf("need %d coins", shop.RerollCost)
		}
		aff.RerollDisabledReason = &reason
	}

	if !canUpgrade {
		reason := fmt.Sprintf("need %d coins (%d upgrade + %d to complete team)",
			shop.UpgradeCost+coinsNeededForCards, shop.UpgradeCost, coinsNeededForCards)
		aff.UpgradeDisabledReason = &reason
	}

	if !canSubmit {
		if teamSize < shop.TeamSize {
			reason := fmt.Sprintf("team incomplete (%d/%d cards)", teamSize, shop.TeamSize)
			aff.SubmitDisabledReason = &reason
		} else if isReady {
			reason := "already submitted"
			aff.SubmitDisabledReason = &reason
		}
	}

	return aff
}

// enhanceShopCards wraps each shop card with affordability info for client display.
// Each card shows whether it can be purchased and disabled reason if not affordable.
func (s *ShopService) enhanceShopCards(cards []*battle.ShopCard, coins int, teamSize int) []*EnhancedShopCard {
	enhanced := make([]*EnhancedShopCard, len(cards))
	for i, card := range cards {
		canAfford := !card.IsPurchased && coins >= shop.CardCost && teamSize < shop.TeamSize
		enhanced[i] = &EnhancedShopCard{
			ShopCard:  card,
			CanAfford: canAfford,
		}
		if !canAfford {
			if card.IsPurchased {
				reason := "already purchased"
				enhanced[i].BuyDisabledReason = &reason
			} else if teamSize >= shop.TeamSize {
				reason := "team is full"
				enhanced[i].BuyDisabledReason = &reason
			} else {
				reason := fmt.Sprintf("need %d coins", shop.CardCost)
				enhanced[i].BuyDisabledReason = &reason
			}
		}
	}
	return enhanced
}

// enhanceTeamCards wraps each team card with upgrade preview information.
// Shows whether upgrades (ATK/HP) are affordable and projected stats after upgrade.
func (s *ShopService) enhanceTeamCards(team []*battle.Card, coins int, teamSize int) []*EnhancedTeamCard {
	remainingCards := shop.TeamSize - teamSize
	coinsNeededForCards := remainingCards * shop.CardCost
	canAffordUpgrade := coins >= (shop.UpgradeCost + coinsNeededForCards)

	enhanced := make([]*EnhancedTeamCard, len(team))
	for i, card := range team {
		enhanced[i] = &EnhancedTeamCard{
			Card:            card,
			CanUpgradeATK:   canAffordUpgrade,
			CanUpgradeHP:    canAffordUpgrade,
			ATKIfUpgraded:   card.ATK + shop.ATKUpgradeAmount,
			HPIfUpgraded:    card.HP + shop.HPUpgradeAmount,
			MaxHPIfUpgraded: card.MaxHP + shop.HPUpgradeAmount,
		}
		if !canAffordUpgrade {
			reason := fmt.Sprintf("need %d coins (%d upgrade + %d to complete team)",
				shop.UpgradeCost+coinsNeededForCards, shop.UpgradeCost, coinsNeededForCards)
			enhanced[i].UpgradeATKDisabledReason = &reason
			enhanced[i].UpgradeHPDisabledReason = &reason
		}
	}
	return enhanced
}

// buildReadOnlyShopState builds a read-only shop state for a user who already submitted their team.
// Returns the user's team and order but no shop cards, with all affordability flags set to false.
func (s *ShopService) buildReadOnlyShopState(ctx context.Context, match *repository.Match, participant *repository.Participant) (*EnhancedShopResponse, error) {
	// Parse team (cards not needed for submitted state)
	_, team, err := parseParticipantShopState(participant)
	if err != nil {
		return nil, err
	}

	// Build read-only response with submitted team
	msg := "team already submitted"
	return &EnhancedShopResponse{
		MatchID:       match.ID,
		Status:        string(match.Status),
		Coins:         participant.CoinsRemaining,
		Cards:         nil, // No shop cards for submitted players
		Team:          s.enhanceTeamCards(team, participant.CoinsRemaining, len(team)),
		TeamOrder:     teamOrderToInt(participant.TeamOrder),
		IsReady:       true,
		TeamSubmitted: true,
		Deadline:      match.ShopPhaseDeadline,
		TimeRemaining: 0, // Submitted players don't need timer
		CanReroll:     false,
		Affordability: ShopAffordability{
			CanBuy:                false,
			CanReroll:             false,
			CanUpgrade:            false,
			CanSubmit:             false,
			BuyDisabledReason:     &msg,
			RerollDisabledReason:  &msg,
			UpgradeDisabledReason: &msg,
			SubmitDisabledReason:  &msg,
		},
	}, nil
}

// GetShop retrieves the enhanced shop state for a player (includes affordability and upgrade previews).
// Returns gracefully after team submission or phase transition instead of errors.
func (s *ShopService) GetShop(ctx context.Context, matchID string, userID int64) (*EnhancedShopResponse, error) {
	defer nrutil.StartSegment(ctx, "service:shop:get-shop")()

	// Get match and participant without strict phase validation
	match, err := s.gameRepo.GetMatch(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get match: %w", err)
	}
	if match == nil {
		return nil, apperror.ErrMatchNotFound
	}

	participant, err := s.gameRepo.GetParticipant(ctx, matchID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participant: %w", err)
	}
	if participant == nil {
		return nil, apperror.ErrNotParticipant
	}

	isReady := participant.Status == repository.ParticipantStatusReady

	// If match moved to battle or completed phase, return status-only response
	if match.Status == repository.MatchStatusBattlePhase || match.Status == repository.MatchStatusCompleted {
		// Return read-only state showing battle/completed phase
		msg := "match has moved to battle phase"
		return &EnhancedShopResponse{
			MatchID:       matchID,
			Status:        string(match.Status),
			Coins:         participant.CoinsRemaining,
			Cards:         nil, // No shop cards when not in shop phase
			Team:          nil, // Will be populated in battle view
			TeamOrder:     teamOrderToInt(participant.TeamOrder),
			IsReady:       isReady,
			TeamSubmitted: isReady,
			Deadline:      match.ShopPhaseDeadline,
			TimeRemaining: 0,
			Affordability: ShopAffordability{
				CanBuy:                false,
				CanReroll:             false,
				CanUpgrade:            false,
				CanSubmit:             false,
				BuyDisabledReason:     &msg,
				RerollDisabledReason:  &msg,
				UpgradeDisabledReason: &msg,
				SubmitDisabledReason:  &msg,
			},
		}, nil
	}

	// Check if we're in shop phase
	if match.Status != repository.MatchStatusShopPhase {
		return nil, fmt.Errorf("failed to get shop: %w", apperror.ErrMatchNotInShopPhase)
	}

	// If user already submitted, return read-only state
	if isReady {
		return s.buildReadOnlyShopState(ctx, match, participant)
	}

	// Parse shop cards and team
	cards, team, err := parseParticipantShopState(participant)
	if err != nil {
		return nil, err
	}

	// Calculate time remaining
	var timeRemaining int
	if match.ShopPhaseDeadline != nil {
		remaining := time.Until(*match.ShopPhaseDeadline)
		if remaining > 0 {
			timeRemaining = int(remaining.Seconds())
		}
	}

	// Build enhanced response with affordability and upgrade previews
	enhancedCards := s.enhanceShopCards(cards, participant.CoinsRemaining, len(team))
	enhancedTeam := s.enhanceTeamCards(team, participant.CoinsRemaining, len(team))
	affordability := s.computeShopAffordability(participant.CoinsRemaining, len(team), false)

	return &EnhancedShopResponse{
		MatchID:       matchID,
		Status:        string(match.Status),
		Coins:         participant.CoinsRemaining,
		Cards:         enhancedCards,
		Team:          enhancedTeam,
		TeamOrder:     teamOrderToInt(participant.TeamOrder),
		IsReady:       false,
		TeamSubmitted: false,
		Deadline:      match.ShopPhaseDeadline,
		TimeRemaining: timeRemaining,
		CanReroll:     affordability.CanReroll,
		Affordability: affordability,
	}, nil
}

// BuyCard purchases a card from the shop.
func (s *ShopService) BuyCard(ctx context.Context, matchID string, userID int64, cardIndex int) (*EnhancedShopResponse, error) {
	defer nrutil.StartSegment(ctx, "service:shop:buy-card")()

	_, participant, err := s.validateShopPhaseAccess(ctx, matchID, userID, true)
	if err != nil {
		return nil, err
	}

	// Parse current state
	cards, team, err := parseParticipantShopState(participant)
	if err != nil {
		return nil, err
	}

	// Validate purchase
	if cardIndex < 0 || cardIndex >= len(cards) {
		return nil, apperror.ErrInvalidCardIndex
	}
	if cards[cardIndex] == nil || cards[cardIndex].IsPurchased {
		return nil, apperror.ErrCardAlreadyPurchased
	}
	if participant.CoinsRemaining < shop.CardCost {
		return nil, apperror.ErrNotEnoughCoins
	}
	if len(team) >= shop.TeamSize {
		return nil, apperror.ErrTeamFull
	}

	// Execute purchase
	cards[cardIndex].IsPurchased = true
	newCard := cards[cardIndex].ToCard()
	newCard.Position = len(team)
	team = append(team, newCard)
	coins := participant.CoinsRemaining - shop.CardCost

	// Save state
	shopCardsJSON, err := jsonutil.Marshal(cards)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal shop cards: %w", err)
	}
	teamJSON, err := jsonutil.Marshal(team)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal team: %w", err)
	}

	if err := s.gameRepo.UpdateParticipantShop(ctx, matchID, userID, coins, shopCardsJSON, teamJSON, participant.TeamOrder); err != nil {
		return nil, fmt.Errorf("failed to save shop state: %w", err)
	}

	// Record card transaction metric
	recordCardTransaction(s.nrApp, "buy", matchID, userID, shop.CardCost, newCard.ATK, newCard.HP)

	return s.GetShop(ctx, matchID, userID)
}

// Reroll replaces all shop cards with new ones, only available before first card purchase.
// Requires coins for both the reroll cost and enough to complete the full team after.
func (s *ShopService) Reroll(ctx context.Context, matchID string, userID int64) (*EnhancedShopResponse, error) {
	defer nrutil.StartSegment(ctx, "service:shop:reroll")()

	match, participant, err := s.validateShopPhaseAccess(ctx, matchID, userID, true)
	if err != nil {
		return nil, err
	}
	if participant.CoinsRemaining < shop.RerollCost {
		return nil, apperror.ErrNotEnoughCoins
	}

	// Unmarshal team to check size for validation
	var currentTeam []*battle.Card
	if participant.Team != nil {
		if err := jsonutil.Unmarshal(*participant.Team, &currentTeam); err != nil {
			return nil, fmt.Errorf("failed to unmarshal team: %w", err)
		}
	}

	// Reroll is only allowed before first purchase (team must be empty)
	if len(currentTeam) > 0 {
		return nil, fmt.Errorf("reroll not allowed after purchasing cards")
	}

	// Calculate coins needed to complete team
	remainingCards := shop.TeamSize - len(currentTeam)
	coinsNeededForCards := remainingCards * shop.CardCost

	// Check if player can afford to complete team after reroll
	if participant.CoinsRemaining < (shop.RerollCost + coinsNeededForCards) {
		return nil, apperror.ErrNotEnoughCoins
	}

	// Parse current cards
	var cards []*battle.ShopCard
	if err := jsonutil.Unmarshal(*participant.ShopCards, &cards); err != nil {
		return nil, fmt.Errorf("failed to parse shop cards: %w", err)
	}

	// Get all current card IDs to exclude from reroll (ensures fresh cards)
	excludeIDs := make([]int64, 0)
	unpurchasedCount := 0
	for _, card := range cards {
		if card != nil {
			excludeIDs = append(excludeIDs, card.CardID)
			if !card.IsPurchased {
				unpurchasedCount++
			}
		}
	}

	// Deal new cards for unpurchased slots
	newCards, err := s.dealer.DealRerollCards(ctx, match.ChatID, unpurchasedCount, excludeIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to deal reroll cards: %w", err)
	}

	// Replace unpurchased cards
	newCardIdx := 0
	for i, card := range cards {
		if card != nil && !card.IsPurchased && newCardIdx < len(newCards) {
			newCards[newCardIdx].Index = i
			cards[i] = newCards[newCardIdx]
			newCardIdx++
		}
	}

	// Deduct cost
	coins := participant.CoinsRemaining - shop.RerollCost

	// Save state
	shopCardsJSON, err := jsonutil.Marshal(cards)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal shop cards: %w", err)
	}

	teamJSON, err := jsonutil.Marshal(currentTeam)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal team: %w", err)
	}

	if err := s.gameRepo.UpdateParticipantShop(ctx, matchID, userID, coins, shopCardsJSON, teamJSON, participant.TeamOrder); err != nil {
		return nil, fmt.Errorf("failed to save shop state: %w", err)
	}

	// Record card transaction metric
	recordCardTransaction(s.nrApp, "reroll", matchID, userID, shop.RerollCost, 0, 0)

	return s.GetShop(ctx, matchID, userID)
}

// UpgradeCard applies an upgrade to a team card.
func (s *ShopService) UpgradeCard(ctx context.Context, matchID string, userID int64, teamSlot int, upgradeType shop.UpgradeType) (*EnhancedShopResponse, error) {
	defer nrutil.StartSegment(ctx, "service:shop:upgrade-card")()

	_, participant, err := s.validateShopPhaseAccess(ctx, matchID, userID, true)
	if err != nil {
		return nil, err
	}
	if participant.CoinsRemaining < shop.UpgradeCost {
		return nil, apperror.ErrNotEnoughCoins
	}

	// Parse team
	var team []*battle.Card
	if participant.Team != nil {
		if err := jsonutil.Unmarshal(*participant.Team, &team); err != nil {
			return nil, fmt.Errorf("failed to parse team: %w", err)
		}
	}

	// Calculate coins needed to complete team
	remainingCards := shop.TeamSize - len(team)
	coinsNeededForCards := remainingCards * shop.CardCost

	// Check if player can afford to complete team after upgrade
	if participant.CoinsRemaining < (shop.UpgradeCost + coinsNeededForCards) {
		return nil, apperror.ErrNotEnoughCoins
	}

	if teamSlot < 0 || teamSlot >= len(team) {
		return nil, apperror.ErrInvalidCardIndex
	}

	// Apply upgrade
	switch upgradeType {
	case shop.UpgradeATK:
		team[teamSlot].ATK += shop.ATKUpgradeAmount
		team[teamSlot].ATKUpgrades++
	case shop.UpgradeHP:
		team[teamSlot].HP += shop.HPUpgradeAmount
		team[teamSlot].MaxHP += shop.HPUpgradeAmount
		team[teamSlot].HPUpgrades++
	default:
		return nil, shop.ErrInvalidUpgradeType
	}

	coins := participant.CoinsRemaining - shop.UpgradeCost

	// Save state
	var cards []*battle.ShopCard
	if err := jsonutil.Unmarshal(*participant.ShopCards, &cards); err != nil {
		return nil, fmt.Errorf("failed to parse shop cards: %w", err)
	}
	shopCardsJSON, err := jsonutil.Marshal(cards)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal shop cards: %w", err)
	}
	teamJSON, err := jsonutil.Marshal(team)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal team: %w", err)
	}

	if err := s.gameRepo.UpdateParticipantShop(ctx, matchID, userID, coins, shopCardsJSON, teamJSON, participant.TeamOrder); err != nil {
		return nil, fmt.Errorf("failed to save shop state: %w", err)
	}

	// Record card transaction metric
	upgradedCard := team[teamSlot]
	recordCardTransaction(s.nrApp, "upgrade", matchID, userID, shop.UpgradeCost, upgradedCard.ATK, upgradedCard.HP)

	return s.GetShop(ctx, matchID, userID)
}

// SetTeamOrder sets the battle order for a player's team.
func (s *ShopService) SetTeamOrder(ctx context.Context, matchID string, userID int64, order []int) (*EnhancedShopResponse, error) {
	defer nrutil.StartSegment(ctx, "service:shop:set-team-order")()

	_, participant, err := s.validateShopPhaseAccess(ctx, matchID, userID, false)
	if err != nil {
		return nil, err
	}

	// Validate order
	var team []*battle.Card
	if participant.Team != nil {
		if err := jsonutil.Unmarshal(*participant.Team, &team); err != nil {
			return nil, fmt.Errorf("failed to parse team: %w", err)
		}
	}
	if len(order) != len(team) {
		return nil, apperror.ErrInvalidTeamOrder
	}

	// Save state
	var cards []*battle.ShopCard
	if err := jsonutil.Unmarshal(*participant.ShopCards, &cards); err != nil {
		return nil, fmt.Errorf("failed to parse shop cards: %w", err)
	}
	shopCardsJSON, err := jsonutil.Marshal(cards)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal shop cards: %w", err)
	}
	teamJSON, err := jsonutil.Marshal(team)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal team: %w", err)
	}

	// Convert order to int64
	order64 := make([]int64, len(order))
	for i, v := range order {
		order64[i] = int64(v)
	}

	if err := s.gameRepo.UpdateParticipantShop(ctx, matchID, userID, participant.CoinsRemaining, shopCardsJSON, teamJSON, order64); err != nil {
		return nil, fmt.Errorf("failed to save shop state: %w", err)
	}

	return s.GetShop(ctx, matchID, userID)
}

// SubmitTeam marks the participant's team as ready for battle.
// The caller (ArenaService.SubmitTeam) triggers checkAndStartBattle after this returns.
func (s *ShopService) SubmitTeam(ctx context.Context, matchID string, userID int64) error {
	defer nrutil.StartSegment(ctx, "service:shop:submit-team")()

	_, participant, err := s.validateShopPhaseAccess(ctx, matchID, userID, false)
	if err != nil {
		return err
	}

	// Validate team has 3 cards
	var team []*battle.Card
	if participant.Team != nil {
		if err := jsonutil.Unmarshal(*participant.Team, &team); err != nil {
			return fmt.Errorf("failed to parse team: %w", err)
		}
	}
	if len(team) != shop.TeamSize {
		return shop.ErrTeamIncomplete
	}

	// Submit team
	if err := s.gameRepo.SubmitTeam(ctx, matchID, userID); err != nil {
		return fmt.Errorf("failed to submit team: %w", err)
	}

	return nil
}
