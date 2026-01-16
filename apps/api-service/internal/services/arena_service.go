package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"beef-briefing/apps/api-service/internal/game/battle"
	"beef-briefing/apps/api-service/internal/game/shop"
	"beef-briefing/apps/api-service/internal/repository"
	"beef-briefing/apps/api-service/internal/storage"

	"github.com/newrelic/go-agent/v3/newrelic"
)

// Arena game constants
const (
	MinimumCardsRequired = 10
	ShopPhaseDuration    = 3 * time.Minute
	JoinWindowDuration   = 5 * time.Minute
)

// ArenaService errors
var (
	ErrNotEnoughCards        = errors.New("not enough cards in group (minimum 10 required)")
	ErrMatchNotFound         = errors.New("match not found")
	ErrMatchNotOpen          = errors.New("match is not open for joining")
	ErrAlreadyJoined         = errors.New("already joined this match")
	ErrNotParticipant        = errors.New("not a participant in this match")
	ErrNotCreator            = errors.New("only the match creator can perform this action")
	ErrMatchNotInShopPhase   = errors.New("match is not in shop phase")
	ErrShopPhaseExpired      = errors.New("shop phase has expired")
	ErrTeamAlreadySubmitted  = errors.New("team already submitted")
	ErrInvalidCardIndex      = errors.New("invalid card index")
	ErrNotEnoughCoins        = errors.New("not enough coins")
	ErrTeamFull              = errors.New("team is full (max 3 cards)")
	ErrCardAlreadyPurchased  = errors.New("card already purchased")
	ErrActiveMatchExists     = errors.New("an active match already exists. Please wait for it to complete before creating a new one.")
)

// ArenaService handles arena game logic
type ArenaService struct {
	db            *sql.DB
	gameRepo      *repository.GameRepository
	dealer        *shop.Dealer
	storageClient *storage.MinIOClient
	cardService   *CardService
	nrApp         *newrelic.Application
	matchMutexes  sync.Map // map[string]*sync.Mutex for per-match locking
}

// validateShopPhaseAccess validates that a user can access the shop for a match.
// It checks: match exists, match is in shop phase, deadline not expired, user is participant, not already submitted.
// Set checkDeadline to false to skip deadline validation (for SetTeamOrder which allows after deadline).
func (s *ArenaService) validateShopPhaseAccess(ctx context.Context, matchID string, userID int64, checkDeadline bool) (*repository.Match, *repository.Participant, error) {
	match, err := s.gameRepo.GetMatch(ctx, matchID)
	if err != nil {
		if txn := newrelic.FromContext(ctx); txn != nil {
			txn.NoticeError(err)
		}
		return nil, nil, fmt.Errorf("failed to get match: %w", err)
	}
	if match == nil {
		err := ErrMatchNotFound
		if txn := newrelic.FromContext(ctx); txn != nil {
			txn.NoticeError(err)
		}
		return nil, nil, err
	}
	if match.Status != repository.MatchStatusShopPhase {
		err := ErrMatchNotInShopPhase
		if txn := newrelic.FromContext(ctx); txn != nil {
			txn.NoticeError(err)
		}
		return nil, nil, err
	}
	if checkDeadline && match.ShopPhaseDeadline != nil && time.Now().After(*match.ShopPhaseDeadline) {
		err := ErrShopPhaseExpired
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
		err := ErrNotParticipant
		if txn := newrelic.FromContext(ctx); txn != nil {
			txn.NoticeError(err)
		}
		return nil, nil, err
	}
	if participant.Status == repository.ParticipantStatusReady {
		err := ErrTeamAlreadySubmitted
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
		if err := json.Unmarshal(*participant.ShopCards, &cards); err != nil {
			return nil, nil, fmt.Errorf("failed to parse shop cards: %w", err)
		}
	}
	if participant.Team != nil {
		if err := json.Unmarshal(*participant.Team, &team); err != nil {
			return nil, nil, fmt.Errorf("failed to parse team: %w", err)
		}
	}
	return cards, team, nil
}

// recordBattleCompletion records a custom event for battle completion.
// This tracks key metrics for business analytics and monitoring.
func recordBattleCompletion(nrApp *newrelic.Application, matchID string, format string, winnerID *int64, isDraw bool, numRounds int, teamADamage, teamBDamage int) {
	if nrApp == nil {
		return
	}

	// Create custom event for battle completion
	params := map[string]interface{}{
		"match_id":       matchID,
		"format":         format, // "1v1" or "arena"
		"is_draw":        isDraw,
		"num_rounds":     numRounds,
		"team_a_damage":  teamADamage,
		"team_b_damage":  teamBDamage,
	}

	if winnerID != nil {
		params["winner_id"] = *winnerID
	}

	nrApp.RecordCustomEvent("arena.battle.completed", params)
}

// recordCardTransaction records a custom event for card purchases, rerolls, and upgrades.
// eventType should be "buy", "reroll", or "upgrade".
func recordCardTransaction(nrApp *newrelic.Application, eventType string, matchID string, userID int64, coinsSpent int, cardATK, cardHP int) {
	if nrApp == nil {
		return
	}

	params := map[string]interface{}{
		"event_type":   eventType,
		"match_id":     matchID,
		"user_id":      userID,
		"coins_spent":  coinsSpent,
	}

	if cardATK > 0 {
		params["card_atk"] = cardATK
	}
	if cardHP > 0 {
		params["card_hp"] = cardHP
	}

	nrApp.RecordCustomEvent("arena.card.transaction", params)
}

// recordMatchEvent records a custom event for match lifecycle events.
// eventType should be "created", "started", "completed", etc.
func recordMatchEvent(nrApp *newrelic.Application, eventType string, matchID string, chatID int64, creatorID int64, matchType string) {
	if nrApp == nil {
		return
	}

	params := map[string]interface{}{
		"event_type":  eventType,
		"match_id":    matchID,
		"chat_id":     chatID,
		"creator_id":  creatorID,
		"match_type":  matchType,
	}

	nrApp.RecordCustomEvent("arena.match.event", params)
}

// recordTournamentEvent records a custom event for tournament lifecycle events.
func recordTournamentEvent(nrApp *newrelic.Application, eventType string, tournamentID int64, chatID int64, participantCount int) {
	if nrApp == nil {
		return
	}

	params := map[string]interface{}{
		"event_type":        eventType,
		"tournament_id":     tournamentID,
		"chat_id":           chatID,
		"participant_count": participantCount,
	}

	nrApp.RecordCustomEvent("arena.tournament.event", params)
}

// NewArenaService creates a new arena service
func NewArenaService(
	db *sql.DB,
	gameRepo *repository.GameRepository,
	storageClient *storage.MinIOClient,
	cardService *CardService,
	nrApp *newrelic.Application,
) *ArenaService {
	return &ArenaService{
		db:            db,
		gameRepo:      gameRepo,
		dealer:        shop.NewDealer(db, nrApp, storageClient, cardService),
		storageClient: storageClient,
		cardService:   cardService,
		nrApp:         nrApp,
	}
}

// MatchResponse represents a match with participant info
type MatchResponse struct {
	*repository.Match
	Participants []*repository.ParticipantWithUser `json:"participants"`
	CardCount    int                               `json:"card_count"`
}

// ShopResponse represents the shop state for a player
type ShopResponse struct {
	MatchID       string             `json:"match_id"`
	Status        string             `json:"status"`
	Coins         int                `json:"coins"`
	Cards         []*battle.ShopCard `json:"cards"`
	Team          []*battle.Card     `json:"team"`
	TeamOrder     []int              `json:"team_order"`
	IsReady       bool               `json:"is_ready"`
	Deadline      *time.Time         `json:"deadline,omitempty"`
	TimeRemaining int                `json:"time_remaining_seconds"`
}

// ShopAffordability represents what actions a player can afford
type ShopAffordability struct {
	CanBuy               bool    `json:"can_buy"`
	CanReroll            bool    `json:"can_reroll"`
	CanUpgrade           bool    `json:"can_upgrade"`
	CanSubmit            bool    `json:"can_submit"`
	BuyDisabledReason    *string `json:"buy_disabled_reason"`
	RerollDisabledReason *string `json:"reroll_disabled_reason"`
	UpgradeDisabledReason *string `json:"upgrade_disabled_reason"`
	SubmitDisabledReason *string `json:"submit_disabled_reason"`
}

// EnhancedShopCard wraps ShopCard with affordability info
type EnhancedShopCard struct {
	*battle.ShopCard
	CanAfford         bool    `json:"can_afford"`
	BuyDisabledReason *string `json:"buy_disabled_reason"`
}

// EnhancedTeamCard wraps Card with upgrade preview info
type EnhancedTeamCard struct {
	*battle.Card
	CanUpgradeATK         bool    `json:"can_upgrade_atk"`
	CanUpgradeHP          bool    `json:"can_upgrade_hp"`
	UpgradeATKDisabledReason *string `json:"upgrade_atk_disabled_reason"`
	UpgradeHPDisabledReason  *string `json:"upgrade_hp_disabled_reason"`
	ATKIfUpgraded         int     `json:"atk_if_upgraded"`
	HPIfUpgraded          int     `json:"hp_if_upgraded"`
	MaxHPIfUpgraded       int     `json:"max_hp_if_upgraded"`
}

// EnhancedShopResponse is the full shop response with affordability and upgrade previews
type EnhancedShopResponse struct {
	MatchID              string                `json:"match_id"`
	Status               string                `json:"status"`
	Coins                int                   `json:"coins"`
	Cards                []*EnhancedShopCard   `json:"cards"`
	Team                 []*EnhancedTeamCard   `json:"team"`
	TeamOrder            []int                 `json:"team_order"`
	IsReady              bool                  `json:"is_ready"`
	TeamSubmitted        bool                  `json:"team_submitted"`
	Deadline             *time.Time            `json:"deadline,omitempty"`
	TimeRemaining        int                   `json:"time_remaining_seconds"`
	CanReroll            bool                  `json:"can_reroll"`
	Affordability        ShopAffordability     `json:"affordability"`
}

// BattleResponse represents battle results in the format the frontend expects
type BattleResponse struct {
	MatchID     string               `json:"match_id"`
	WinnerID    *int64               `json:"winner_id,omitempty"`
	IsDraw      bool                 `json:"is_draw"`
	Events      []battle.BattleEvent `json:"events"`
	NumRounds   int                  `json:"num_rounds"`
	TeamADamage int                  `json:"team_a_damage"`
	TeamBDamage int                  `json:"team_b_damage"`
	TeamAFinal  *battle.Team         `json:"team_a_final"`
	TeamBFinal  *battle.Team         `json:"team_b_final"`
	PlayerAID   int64                `json:"player_a_id"`
	PlayerBID   int64                `json:"player_b_id"`
	PlayerAName string               `json:"player_a_name"`
	PlayerBName string               `json:"player_b_name"`
}

// CreateMatch creates a new regular match
func (s *ArenaService) CreateMatch(ctx context.Context, chatID int64, creatorUserID int64) (*MatchResponse, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:create-match")
		defer segment.End()
	}

	// Check for existing active regular match
	activeMatches, err := s.gameRepo.GetActiveMatches(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to check active matches: %w", err)
	}

	// Only allow one active regular match at a time
	for _, match := range activeMatches {
		if match.MatchType == repository.MatchTypeRegular &&
			(match.Status == repository.MatchStatusOpen ||
				match.Status == repository.MatchStatusShopPhase ||
				match.Status == repository.MatchStatusBattlePhase) {
			return nil, ErrActiveMatchExists
		}
	}

	// Check minimum cards
	cardCount, err := s.dealer.GetCardCount(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get card count: %w", err)
	}
	if cardCount < MinimumCardsRequired {
		return nil, ErrNotEnoughCards
	}

	// Create match
	match, err := s.gameRepo.CreateMatch(ctx, chatID, repository.MatchTypeRegular, &creatorUserID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create match: %w", err)
	}

	// Auto-join creator
	_, err = s.gameRepo.AddParticipant(ctx, match.ID, creatorUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to add creator as participant: %w", err)
	}

	// Fetch participants
	participants, err := s.gameRepo.GetMatchParticipants(ctx, match.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participants: %w", err)
	}

	// Record match creation metric
	recordMatchEvent(s.nrApp, "created", match.ID, chatID, creatorUserID, string(repository.MatchTypeRegular))

	return &MatchResponse{
		Match:        match,
		Participants: participants,
		CardCount:    cardCount,
	}, nil
}

// GetMatch retrieves a match with participants
func (s *ArenaService) GetMatch(ctx context.Context, matchID string) (*MatchResponse, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:get-match")
		defer segment.End()
	}

	match, err := s.gameRepo.GetMatch(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get match: %w", err)
	}
	if match == nil {
		return nil, ErrMatchNotFound
	}

	participants, err := s.gameRepo.GetMatchParticipants(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participants: %w", err)
	}

	cardCount, _ := s.dealer.GetCardCount(ctx, match.ChatID)

	return &MatchResponse{
		Match:        match,
		Participants: participants,
		CardCount:    cardCount,
	}, nil
}

// GetActiveMatches retrieves active matches for a chat
func (s *ArenaService) GetActiveMatches(ctx context.Context, chatID int64) ([]*MatchResponse, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:get-active-matches")
		defer segment.End()
	}

	matches, err := s.gameRepo.GetActiveMatches(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active matches: %w", err)
	}

	responses := make([]*MatchResponse, 0, len(matches))
	for _, match := range matches {
		participants, err := s.gameRepo.GetMatchParticipants(ctx, match.ID)
		if err != nil {
			continue // Skip on error
		}
		responses = append(responses, &MatchResponse{
			Match:        match,
			Participants: participants,
		})
	}

	return responses, nil
}

// JoinMatch adds a user to a match
func (s *ArenaService) JoinMatch(ctx context.Context, matchID string, userID int64) (*MatchResponse, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:join-match")
		defer segment.End()
	}

	match, err := s.gameRepo.GetMatch(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get match: %w", err)
	}
	if match == nil {
		return nil, ErrMatchNotFound
	}

	// Check match is open
	if match.Status != repository.MatchStatusOpen {
		return nil, ErrMatchNotOpen
	}

	// Check join deadline for regular matches
	if match.JoinDeadline != nil && time.Now().After(*match.JoinDeadline) {
		return nil, ErrMatchNotOpen
	}

	// Add participant
	_, err = s.gameRepo.AddParticipant(ctx, matchID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to add participant: %w", err)
	}

	return s.GetMatch(ctx, matchID)
}

// LeaveMatch removes a user from a match
func (s *ArenaService) LeaveMatch(ctx context.Context, matchID string, userID int64) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:leave-match")
		defer segment.End()
	}

	match, err := s.gameRepo.GetMatch(ctx, matchID)
	if err != nil {
		return fmt.Errorf("failed to get match: %w", err)
	}
	if match == nil {
		return ErrMatchNotFound
	}

	// Can only leave during open phase
	if match.Status != repository.MatchStatusOpen {
		return ErrMatchNotOpen
	}

	return s.gameRepo.RemoveParticipant(ctx, matchID, userID)
}

// StartMatch starts a match early (creator only)
func (s *ArenaService) StartMatch(ctx context.Context, matchID string, userID int64) (*MatchResponse, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:start-match")
		defer segment.End()
	}

	match, err := s.gameRepo.GetMatch(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get match: %w", err)
	}
	if match == nil {
		return nil, ErrMatchNotFound
	}

	// Verify creator
	if match.CreatorUserID == nil || *match.CreatorUserID != userID {
		return nil, ErrNotCreator
	}

	// Check match is open
	if match.Status != repository.MatchStatusOpen {
		return nil, ErrMatchNotOpen
	}

	// Get participant count
	count, err := s.gameRepo.GetParticipantCount(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participant count: %w", err)
	}

	// Need at least 2 participants
	if count < 2 {
		return nil, errors.New("need at least 2 participants to start")
	}

	// Determine format based on participant count
	format := repository.MatchFormat1v1
	if count > 2 {
		format = repository.MatchFormatArena
	}

	// Start shop phase (also sets format)
	deadline := time.Now().Add(ShopPhaseDuration)
	if err := s.gameRepo.StartShopPhase(ctx, matchID, format, deadline); err != nil {
		return nil, fmt.Errorf("failed to start shop phase: %w", err)
	}

	// Deal cards to all participants
	if err := s.dealCardsToParticipants(ctx, matchID, match.ChatID); err != nil {
		return nil, fmt.Errorf("failed to deal cards: %w", err)
	}

	return s.GetMatch(ctx, matchID)
}

// dealCardsToParticipants deals initial shop cards to all participants
func (s *ArenaService) dealCardsToParticipants(ctx context.Context, matchID string, chatID int64) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:deal-cards-to-participants")
		defer segment.End()
	}

	participants, err := s.gameRepo.GetMatchParticipants(ctx, matchID)
	if err != nil {
		return err
	}

	for _, p := range participants {
		cards, err := s.dealer.DealCards(ctx, chatID, shop.ShopSize)
		if err != nil {
			return fmt.Errorf("failed to deal cards for user %d: %w", p.UserID, err)
		}

		shopCardsJSON, err := json.Marshal(cards)
		if err != nil {
			return fmt.Errorf("failed to marshal shop cards for user %d: %w", p.UserID, err)
		}
		teamJSON, err := json.Marshal([]*battle.Card{})
		if err != nil {
			return fmt.Errorf("failed to marshal team for user %d: %w", p.UserID, err)
		}
		teamOrder := []int64{0, 1, 2}

		if err := s.gameRepo.UpdateParticipantShop(ctx, matchID, p.UserID, shop.StartingCoins, shopCardsJSON, teamJSON, teamOrder); err != nil {
			return fmt.Errorf("failed to save shop state for user %d: %w", p.UserID, err)
		}
	}

	return nil
}

// computeShopAffordability computes what actions a player can afford in the shop.
// Considers coins available and coins needed to complete team (3 cards total).
// Returns affordability status and disabled reasons for each action if not affordable.
func (s *ArenaService) computeShopAffordability(coins int, teamSize int, isReady bool) ShopAffordability {
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
func (s *ArenaService) enhanceShopCards(cards []*battle.ShopCard, coins int, teamSize int) []*EnhancedShopCard {
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
func (s *ArenaService) enhanceTeamCards(team []*battle.Card, coins int, teamSize int) []*EnhancedTeamCard {
	remainingCards := shop.TeamSize - teamSize
	coinsNeededForCards := remainingCards * shop.CardCost
	canAffordUpgrade := coins >= (shop.UpgradeCost + coinsNeededForCards)

	enhanced := make([]*EnhancedTeamCard, len(team))
	for i, card := range team {
		enhanced[i] = &EnhancedTeamCard{
			Card:                card,
			CanUpgradeATK:       canAffordUpgrade,
			CanUpgradeHP:        canAffordUpgrade,
			ATKIfUpgraded:       card.ATK + shop.ATKUpgradeAmount,
			HPIfUpgraded:        card.HP + shop.HPUpgradeAmount,
			MaxHPIfUpgraded:     card.MaxHP + shop.HPUpgradeAmount,
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

// GetShop retrieves the enhanced shop state for a player (includes affordability and upgrade previews).
// Returns gracefully after team submission or phase transition instead of errors.
func (s *ArenaService) GetShop(ctx context.Context, matchID string, userID int64) (*EnhancedShopResponse, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:get-shop")
		defer segment.End()
	}

	// Get match and participant without strict phase validation
	match, err := s.gameRepo.GetMatch(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get match: %w", err)
	}
	if match == nil {
		return nil, ErrMatchNotFound
	}

	participant, err := s.gameRepo.GetParticipant(ctx, matchID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participant: %w", err)
	}
	if participant == nil {
		return nil, ErrNotParticipant
	}

	isReady := participant.Status == repository.ParticipantStatusReady

	// If match moved to battle or completed phase, return status-only response
	if match.Status == repository.MatchStatusBattlePhase || match.Status == repository.MatchStatusCompleted {
		// Convert TeamOrder from int64 to int for API response
		teamOrder := make([]int, len(participant.TeamOrder))
		for i, v := range participant.TeamOrder {
			teamOrder[i] = int(v)
		}

		// Return read-only state showing battle/completed phase
		msg := "match has moved to battle phase"
		return &EnhancedShopResponse{
			MatchID:       matchID,
			Status:        string(match.Status),
			Coins:         participant.CoinsRemaining,
			Cards:         nil, // No shop cards when not in shop phase
			Team:          nil, // Will be populated in battle view
			TeamOrder:     teamOrder,
			IsReady:       isReady,
			TeamSubmitted: isReady,
			Deadline:      match.ShopPhaseDeadline,
			TimeRemaining: 0,
			Affordability: ShopAffordability{
				CanBuy:                 false,
				CanReroll:              false,
				CanUpgrade:             false,
				CanSubmit:              false,
				BuyDisabledReason:      &msg,
				RerollDisabledReason:   &msg,
				UpgradeDisabledReason:  &msg,
				SubmitDisabledReason:   &msg,
			},
		}, nil
	}

	// Check if we're in shop phase
	if match.Status != repository.MatchStatusShopPhase {
		return nil, fmt.Errorf("failed to get shop: %w", ErrMatchNotInShopPhase)
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

	// Convert TeamOrder from int64 to int for API response
	teamOrder := make([]int, len(participant.TeamOrder))
	for i, v := range participant.TeamOrder {
		teamOrder[i] = int(v)
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
		TeamOrder:     teamOrder,
		IsReady:       false,
		TeamSubmitted: false,
		Deadline:      match.ShopPhaseDeadline,
		TimeRemaining: timeRemaining,
		CanReroll:     affordability.CanReroll,
		Affordability: affordability,
	}, nil
}

// buildReadOnlyShopState builds a read-only shop state for a user who already submitted their team.
// Returns the user's team and order but no shop cards, with all affordability flags set to false.
func (s *ArenaService) buildReadOnlyShopState(ctx context.Context, match *repository.Match, participant *repository.Participant) (*EnhancedShopResponse, error) {
	// Parse team (cards not needed for submitted state)
	_, team, err := parseParticipantShopState(participant)
	if err != nil {
		return nil, err
	}

	// Convert TeamOrder from int64 to int for API response
	teamOrder := make([]int, len(participant.TeamOrder))
	for i, v := range participant.TeamOrder {
		teamOrder[i] = int(v)
	}

	// Build read-only response with submitted team
	msg := "team already submitted"
	return &EnhancedShopResponse{
		MatchID:       match.ID,
		Status:        string(match.Status),
		Coins:         participant.CoinsRemaining,
		Cards:         nil, // No shop cards for submitted players
		Team:          s.enhanceTeamCards(team, participant.CoinsRemaining, len(team)),
		TeamOrder:     teamOrder,
		IsReady:       true,
		TeamSubmitted: true,
		Deadline:      match.ShopPhaseDeadline,
		TimeRemaining: 0, // Submitted players don't need timer
		CanReroll:     false,
		Affordability: ShopAffordability{
			CanBuy:                 false,
			CanReroll:              false,
			CanUpgrade:             false,
			CanSubmit:              false,
			BuyDisabledReason:      &msg,
			RerollDisabledReason:   &msg,
			UpgradeDisabledReason:  &msg,
			SubmitDisabledReason:   &msg,
		},
	}, nil
}

// BuyCard purchases a card from the shop
func (s *ArenaService) BuyCard(ctx context.Context, matchID string, userID int64, cardIndex int) (*EnhancedShopResponse, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:buy-card")
		defer segment.End()
	}

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
		return nil, ErrInvalidCardIndex
	}
	if cards[cardIndex] == nil || cards[cardIndex].IsPurchased {
		return nil, ErrCardAlreadyPurchased
	}
	if participant.CoinsRemaining < shop.CardCost {
		return nil, ErrNotEnoughCoins
	}
	if len(team) >= shop.TeamSize {
		return nil, ErrTeamFull
	}

	// Execute purchase
	cards[cardIndex].IsPurchased = true
	newCard := cards[cardIndex].ToCard()
	newCard.Position = len(team)
	team = append(team, newCard)
	coins := participant.CoinsRemaining - shop.CardCost

	// Save state
	shopCardsJSON, err := json.Marshal(cards)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal shop cards: %w", err)
	}
	teamJSON, err := json.Marshal(team)
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

// Reroll replaces unpurchased cards with new ones
func (s *ArenaService) Reroll(ctx context.Context, matchID string, userID int64) (*EnhancedShopResponse, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:reroll")
		defer segment.End()
	}

	match, participant, err := s.validateShopPhaseAccess(ctx, matchID, userID, true)
	if err != nil {
		return nil, err
	}
	if participant.CoinsRemaining < shop.RerollCost {
		return nil, ErrNotEnoughCoins
	}

	// Unmarshal team to check size for validation
	var currentTeam []*battle.Card
	if participant.Team != nil {
		if err := json.Unmarshal(*participant.Team, &currentTeam); err != nil {
			return nil, fmt.Errorf("failed to unmarshal team: %w", err)
		}
	}

	// Calculate coins needed to complete team
	remainingCards := shop.TeamSize - len(currentTeam)
	coinsNeededForCards := remainingCards * shop.CardCost

	// Check if player can afford to complete team after reroll
	if participant.CoinsRemaining < (shop.RerollCost + coinsNeededForCards) {
		return nil, ErrNotEnoughCoins
	}

	// Parse current cards
	var cards []*battle.ShopCard
	if err := json.Unmarshal(*participant.ShopCards, &cards); err != nil {
		return nil, fmt.Errorf("failed to parse shop cards: %w", err)
	}

	// Get IDs of already purchased cards to exclude
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
	shopCardsJSON, err := json.Marshal(cards)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal shop cards: %w", err)
	}

	teamJSON, err := json.Marshal(currentTeam)
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

// UpgradeCard applies an upgrade to a team card
func (s *ArenaService) UpgradeCard(ctx context.Context, matchID string, userID int64, teamSlot int, upgradeType shop.UpgradeType) (*EnhancedShopResponse, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:upgrade-card")
		defer segment.End()
	}

	_, participant, err := s.validateShopPhaseAccess(ctx, matchID, userID, true)
	if err != nil {
		return nil, err
	}
	if participant.CoinsRemaining < shop.UpgradeCost {
		return nil, ErrNotEnoughCoins
	}

	// Parse team
	var team []*battle.Card
	if participant.Team != nil {
		if err := json.Unmarshal(*participant.Team, &team); err != nil {
			return nil, fmt.Errorf("failed to parse team: %w", err)
		}
	}

	// Calculate coins needed to complete team
	remainingCards := shop.TeamSize - len(team)
	coinsNeededForCards := remainingCards * shop.CardCost

	// Check if player can afford to complete team after upgrade
	if participant.CoinsRemaining < (shop.UpgradeCost + coinsNeededForCards) {
		return nil, ErrNotEnoughCoins
	}

	if teamSlot < 0 || teamSlot >= len(team) {
		return nil, ErrInvalidCardIndex
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
	if err := json.Unmarshal(*participant.ShopCards, &cards); err != nil {
		return nil, fmt.Errorf("failed to parse shop cards: %w", err)
	}
	shopCardsJSON, err := json.Marshal(cards)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal shop cards: %w", err)
	}
	teamJSON, err := json.Marshal(team)
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

// SetTeamOrder sets the battle order for a player's team
func (s *ArenaService) SetTeamOrder(ctx context.Context, matchID string, userID int64, order []int) (*EnhancedShopResponse, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:set-team-order")
		defer segment.End()
	}

	_, participant, err := s.validateShopPhaseAccess(ctx, matchID, userID, false)
	if err != nil {
		return nil, err
	}

	// Validate order
	var team []*battle.Card
	if participant.Team != nil {
		if err := json.Unmarshal(*participant.Team, &team); err != nil {
			return nil, fmt.Errorf("failed to parse team: %w", err)
		}
	}
	if len(order) != len(team) {
		return nil, errors.New("order length must match team size")
	}

	// Save state
	var cards []*battle.ShopCard
	if err := json.Unmarshal(*participant.ShopCards, &cards); err != nil {
		return nil, fmt.Errorf("failed to parse shop cards: %w", err)
	}
	shopCardsJSON, err := json.Marshal(cards)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal shop cards: %w", err)
	}
	teamJSON, err := json.Marshal(team)
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

// SubmitTeam submits the team for battle
func (s *ArenaService) SubmitTeam(ctx context.Context, matchID string, userID int64) (*EnhancedShopResponse, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:submit-team")
		defer segment.End()
	}

	_, participant, err := s.validateShopPhaseAccess(ctx, matchID, userID, false)
	if err != nil {
		return nil, err
	}

	// Validate team has 3 cards
	var team []*battle.Card
	if participant.Team != nil {
		if err := json.Unmarshal(*participant.Team, &team); err != nil {
			return nil, fmt.Errorf("failed to parse team: %w", err)
		}
	}
	if len(team) != shop.TeamSize {
		return nil, shop.ErrTeamIncomplete
	}

	// Submit team
	if err := s.gameRepo.SubmitTeam(ctx, matchID, userID); err != nil {
		return nil, fmt.Errorf("failed to submit team: %w", err)
	}

	// Check if all participants are ready
	// Create detached context for background battle check with timeout protection
	asyncCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	go func() {
		defer cancel()
		s.checkAndStartBattle(asyncCtx, matchID)
	}()

	return s.GetShop(ctx, matchID, userID)
}

// checkAndStartBattle checks if all participants are ready and starts battle
func (s *ArenaService) checkAndStartBattle(ctx context.Context, matchID string) {
	// Get or create mutex for this match to prevent race condition
	mutexInterface, _ := s.matchMutexes.LoadOrStore(matchID, &sync.Mutex{})
	mutex := mutexInterface.(*sync.Mutex)

	mutex.Lock()
	defer mutex.Unlock()
	defer s.matchMutexes.Delete(matchID) // Cleanup after

	total, _ := s.gameRepo.GetParticipantCount(ctx, matchID)
	ready, _ := s.gameRepo.GetReadyParticipantCount(ctx, matchID)

	if ready >= total && total >= 2 {
		s.StartBattle(ctx, matchID)
	}
}

// StartBattle initiates the battle phase
func (s *ArenaService) StartBattle(ctx context.Context, matchID string) (*BattleResponse, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:start-battle")
		defer segment.End()
	}

	match, err := s.gameRepo.GetMatch(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get match: %w", err)
	}
	if match == nil {
		return nil, ErrMatchNotFound
	}

	// Transition to battle phase
	if err := s.gameRepo.StartBattlePhase(ctx, matchID); err != nil {
		return nil, fmt.Errorf("failed to start battle phase: %w", err)
	}

	// Get participants
	participants, err := s.gameRepo.GetMatchParticipants(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participants: %w", err)
	}

	// For 1v1, run single battle
	if len(participants) == 2 {
		return s.runBattle(ctx, matchID, participants[0], participants[1], 1)
	}

	// For arena format, run tournament bracket (simplified: round-robin for now)
	return s.runArena(ctx, matchID, participants)
}

// normalizeTeamOrder ensures the team order array matches the team size.
// If lengths don't match, returns a sequential order [0, 1, 2, ...]
func normalizeTeamOrder(order []int64, teamSize int) []int64 {
	if len(order) == teamSize {
		return order
	}

	// Build valid sequential order
	normalized := make([]int64, teamSize)
	for i := 0; i < teamSize; i++ {
		normalized[i] = int64(i)
	}
	return normalized
}

// runBattle executes a single 1v1 battle between two participants and records the results.
// It handles team parsing, team order normalization, battle simulation, round persistence,
// and leaderboard updates. Returns a BattleResponse with the outcome and auto-completes
// the match for 1v1 format battles.
func (s *ArenaService) runBattle(ctx context.Context, matchID string, pA, pB *repository.ParticipantWithUser, roundNumber int) (*BattleResponse, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:run-battle")
		defer segment.End()
	}

	// Parse teams
	var teamACards, teamBCards []*battle.Card
	if err := json.Unmarshal(*pA.Team, &teamACards); err != nil {
		return nil, fmt.Errorf("failed to parse team A: %w", err)
	}
	if err := json.Unmarshal(*pB.Team, &teamBCards); err != nil {
		return nil, fmt.Errorf("failed to parse team B: %w", err)
	}

	// Validate team order lengths match team sizes, log warnings if mismatched
	if len(pA.TeamOrder) != len(teamACards) {
		slog.Warn("team order length mismatch for player A",
			"user_id", pA.UserID,
			"match_id", matchID,
			"order_len", len(pA.TeamOrder),
			"team_len", len(teamACards))
		pA.TeamOrder = normalizeTeamOrder(pA.TeamOrder, len(teamACards))
	}
	if len(pB.TeamOrder) != len(teamBCards) {
		slog.Warn("team order length mismatch for player B",
			"user_id", pB.UserID,
			"match_id", matchID,
			"order_len", len(pB.TeamOrder),
			"team_len", len(teamBCards))
		pB.TeamOrder = normalizeTeamOrder(pB.TeamOrder, len(teamBCards))
	}

	// Apply team order, filtering out invalid indices
	orderedA := make([]*battle.Card, 0, len(teamACards))
	orderedB := make([]*battle.Card, 0, len(teamBCards))

	for i, idx := range pA.TeamOrder {
		if idx >= 0 && idx < int64(len(teamACards)) {
			orderedA = append(orderedA, teamACards[idx])
		} else {
			slog.Warn("invalid team order index for player A",
				"user_id", pA.UserID,
				"match_id", matchID,
				"position", i,
				"index", idx,
				"team_size", len(teamACards))
		}
	}

	for i, idx := range pB.TeamOrder {
		if idx >= 0 && idx < int64(len(teamBCards)) {
			orderedB = append(orderedB, teamBCards[idx])
		} else {
			slog.Warn("invalid team order index for player B",
				"user_id", pB.UserID,
				"match_id", matchID,
				"position", i,
				"index", idx,
				"team_size", len(teamBCards))
		}
	}

	// Validate teams have valid cards after ordering
	if len(orderedA) == 0 || len(orderedB) == 0 {
		return nil, fmt.Errorf("invalid team composition after ordering: playerA=%d cards, playerB=%d cards",
			len(orderedA), len(orderedB))
	}

	// Prefer FirstName for display, fall back to Username if empty
	ownerNameA := pA.FirstName
	if ownerNameA == "" {
		ownerNameA = pA.Username
	}
	ownerNameB := pB.FirstName
	if ownerNameB == "" {
		ownerNameB = pB.Username
	}

	teamA := battle.NewTeam(pA.UserID, ownerNameA, orderedA)
	teamB := battle.NewTeam(pB.UserID, ownerNameB, orderedB)

	// Run battle simulation
	result := battle.Simulate(teamA, teamB)

	// Record battle completion metric
	recordBattleCompletion(s.nrApp, matchID, "1v1", result.WinnerID, result.IsDraw, result.NumRounds, result.TeamADamage, result.TeamBDamage)

	// Save round
	teamAJSON, err := json.Marshal(orderedA)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal team A: %w", err)
	}
	teamBJSON, err := json.Marshal(orderedB)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal team B: %w", err)
	}
	eventsJSON, err := json.Marshal(result.Events)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal events: %w", err)
	}

	_, err = s.gameRepo.CreateRound(ctx, matchID, roundNumber,
		pA.UserID, pB.UserID,
		teamAJSON, teamBJSON, eventsJSON,
		result.WinnerID, result.IsDraw,
		result.TeamADamage, result.TeamBDamage, result.NumRounds)
	if err != nil {
		return nil, fmt.Errorf("failed to save round: %w", err)
	}

	// Update leaderboard
	match, err := s.gameRepo.GetMatch(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get match for leaderboard update: %w", err)
	}
	if match == nil {
		return nil, fmt.Errorf("match not found for leaderboard update: %s", matchID)
	}

	if result.WinnerID != nil {
		if *result.WinnerID == pA.UserID {
			s.gameRepo.UpdateLeaderboard(ctx, pA.UserID, match.ChatID, match.MatchType, true, &pB.UserID, false, false)
			s.gameRepo.UpdateLeaderboard(ctx, pB.UserID, match.ChatID, match.MatchType, false, &pA.UserID, false, false)
		} else {
			s.gameRepo.UpdateLeaderboard(ctx, pB.UserID, match.ChatID, match.MatchType, true, &pA.UserID, false, false)
			s.gameRepo.UpdateLeaderboard(ctx, pA.UserID, match.ChatID, match.MatchType, false, &pB.UserID, false, false)
		}
	} else if result.IsDraw {
		// Handle draws: update both players with no win, but increment match count
		s.gameRepo.UpdateLeaderboard(ctx, pA.UserID, match.ChatID, match.MatchType, false, &pB.UserID, false, true)
		s.gameRepo.UpdateLeaderboard(ctx, pB.UserID, match.ChatID, match.MatchType, false, &pA.UserID, false, true)
	}

	// Complete match for 1v1
	s.gameRepo.CompleteMatch(ctx, matchID, result.WinnerID)

	return &BattleResponse{
		MatchID:     matchID,
		WinnerID:    result.WinnerID,
		IsDraw:      result.IsDraw,
		Events:      result.Events,
		NumRounds:   result.NumRounds,
		TeamADamage: result.TeamADamage,
		TeamBDamage: result.TeamBDamage,
		TeamAFinal:  result.TeamAFinal,
		TeamBFinal:  result.TeamBFinal,
		PlayerAID:   pA.UserID,
		PlayerBID:   pB.UserID,
		PlayerAName: ownerNameA,
		PlayerBName: ownerNameB,
	}, nil
}

// runArena executes arena format with round-robin tournament bracket.
// Each participant faces every other participant. Winner is determined by:
// 1. Most wins across all battles
// 2. Total damage dealt (as tiebreaker if wins are equal)
// 3. Lower user_id (deterministic tiebreaker if damage is tied)
// Completes the match with the tournament winner.
func (s *ArenaService) runArena(ctx context.Context, matchID string, participants []*repository.ParticipantWithUser) (*BattleResponse, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:run-arena")
		defer segment.End()
	}

	roundNumber := 0

	// Round-robin: each player fights each other player
	for i := 0; i < len(participants); i++ {
		for j := i + 1; j < len(participants); j++ {
			roundNumber++
			_, err := s.runBattle(ctx, matchID, participants[i], participants[j], roundNumber)
			if err != nil {
				return nil, err
			}
		}
	}

	// Determine winner by most wins, then total damage, then user_id (deterministic tiebreaker)
	participants, _ = s.gameRepo.GetMatchParticipants(ctx, matchID)
	var winner *repository.ParticipantWithUser
	maxWins := -1
	var maxDamage int
	var tieBreakID int64 = 9223372036854775807 // math.MaxInt64

	for _, p := range participants {
		isNewLeader := false

		if p.Wins > maxWins {
			// More wins always takes priority
			isNewLeader = true
		} else if p.Wins == maxWins {
			// If tied on wins, check total damage
			if p.TotalDamageDealt > maxDamage {
				isNewLeader = true
			} else if p.TotalDamageDealt == maxDamage && p.UserID < tieBreakID {
				// If still tied on damage, use lower user_id (deterministic)
				isNewLeader = true
			}
		}

		if isNewLeader {
			maxWins = p.Wins
			maxDamage = p.TotalDamageDealt
			tieBreakID = p.UserID
			winner = p
		}
	}

	var winnerID *int64
	if winner != nil {
		winnerID = &winner.UserID
	}

	s.gameRepo.CompleteMatch(ctx, matchID, winnerID)

	// Record arena completion metric
	recordBattleCompletion(s.nrApp, matchID, "arena", winnerID, false, roundNumber, 0, 0)

	// Arena format returns minimal response (multi-player tournaments)
	// The Mini App doesn't use arena format - it uses 1v1
	return &BattleResponse{
		MatchID:   matchID,
		WinnerID:  winnerID,
		IsDraw:    false,
		Events:    []battle.BattleEvent{},
		NumRounds: roundNumber,
	}, nil
}

// GetBattle retrieves battle results
func (s *ArenaService) GetBattle(ctx context.Context, matchID string, userID int64) (*BattleResponse, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:get-battle")
		defer segment.End()
	}

	match, err := s.gameRepo.GetMatch(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get match: %w", err)
	}
	if match == nil {
		return nil, ErrMatchNotFound
	}

	// Verify participant
	participant, err := s.gameRepo.GetParticipant(ctx, matchID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participant: %w", err)
	}
	if participant == nil {
		return nil, ErrNotParticipant
	}

	rounds, err := s.gameRepo.GetMatchRounds(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get rounds: %w", err)
	}

	// No rounds yet - return minimal response
	if len(rounds) == 0 {
		return &BattleResponse{
			MatchID: matchID,
			Events:  []battle.BattleEvent{},
		}, nil
	}

	// Get the first round (for 1v1 matches, there's only one round)
	round := rounds[0]

	// Parse teams from stored JSON
	var teamACards, teamBCards []*battle.Card
	if err := json.Unmarshal(round.PlayerATeam, &teamACards); err != nil {
		return nil, fmt.Errorf("failed to parse team A: %w", err)
	}
	if err := json.Unmarshal(round.PlayerBTeam, &teamBCards); err != nil {
		return nil, fmt.Errorf("failed to parse team B: %w", err)
	}

	// Parse battle events
	var events []battle.BattleEvent
	if err := json.Unmarshal(round.BattleLog, &events); err != nil {
		return nil, fmt.Errorf("failed to parse battle log: %w", err)
	}

	// Get participant names
	participants, err := s.gameRepo.GetMatchParticipants(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participants: %w", err)
	}

	// Build name lookup map
	nameMap := make(map[int64]string)
	for _, p := range participants {
		name := p.FirstName
		if name == "" {
			name = p.Username
		}
		nameMap[p.UserID] = name
	}

	// Build teams with owner info
	teamAFinal := &battle.Team{
		OwnerID:   round.PlayerAID,
		OwnerName: nameMap[round.PlayerAID],
		Cards:     teamACards,
	}
	teamBFinal := &battle.Team{
		OwnerID:   round.PlayerBID,
		OwnerName: nameMap[round.PlayerBID],
		Cards:     teamBCards,
	}

	return &BattleResponse{
		MatchID:     matchID,
		WinnerID:    round.WinnerID,
		IsDraw:      round.IsDraw,
		Events:      events,
		NumRounds:   round.TotalRounds,
		TeamADamage: round.PlayerADmg,
		TeamBDamage: round.PlayerBDmg,
		TeamAFinal:  teamAFinal,
		TeamBFinal:  teamBFinal,
		PlayerAID:   round.PlayerAID,
		PlayerBID:   round.PlayerBID,
		PlayerAName: nameMap[round.PlayerAID],
		PlayerBName: nameMap[round.PlayerBID],
	}, nil
}

// GetLeaderboard retrieves leaderboard for a chat
func (s *ArenaService) GetLeaderboard(ctx context.Context, chatID int64, matchType string, limit, offset int) ([]*repository.LeaderboardEntry, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:get-leaderboard")
		defer segment.End()
	}

	mt := repository.MatchTypeRanked
	if matchType == "regular" {
		mt = repository.MatchTypeRegular
	}
	return s.gameRepo.GetLeaderboard(ctx, chatID, mt, limit, offset)
}

// =============================================================================
// Bot/Scheduler methods (for background processing)
// =============================================================================

// PendingMatch represents a match that needs action
type PendingMatch struct {
	*repository.Match
	Action           string `json:"action"` // "auto_start" or "force_submit"
	ParticipantCount int    `json:"participant_count"`
}

// GetPendingMatches returns matches with expired deadlines that need action
func (s *ArenaService) GetPendingMatches(ctx context.Context) ([]*PendingMatch, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:get-pending-matches")
		defer segment.End()
	}

	pendingMatches := make([]*PendingMatch, 0)
	now := time.Now()

	// Get open matches with expired join deadline
	openMatches, err := s.gameRepo.GetMatchesByStatus(ctx, repository.MatchStatusOpen)
	if err != nil {
		return nil, fmt.Errorf("failed to get open matches: %w", err)
	}

	for _, match := range openMatches {
		if match.JoinDeadline != nil && now.After(*match.JoinDeadline) {
			count, _ := s.gameRepo.GetParticipantCount(ctx, match.ID)
			pendingMatches = append(pendingMatches, &PendingMatch{
				Match:            match,
				Action:           "auto_start",
				ParticipantCount: count,
			})
		}
	}

	// Get shop phase matches with expired shop deadline
	shopMatches, err := s.gameRepo.GetMatchesByStatus(ctx, repository.MatchStatusShopPhase)
	if err != nil {
		return nil, fmt.Errorf("failed to get shop phase matches: %w", err)
	}

	for _, match := range shopMatches {
		if match.ShopPhaseDeadline != nil && now.After(*match.ShopPhaseDeadline) {
			count, _ := s.gameRepo.GetParticipantCount(ctx, match.ID)
			pendingMatches = append(pendingMatches, &PendingMatch{
				Match:            match,
				Action:           "force_submit",
				ParticipantCount: count,
			})
		}
	}

	return pendingMatches, nil
}

// AutoStartResult represents the result of an auto-start attempt
type AutoStartResult struct {
	MatchID      string `json:"match_id"`
	ChatID       int64  `json:"chat_id"`
	Action       string `json:"action"` // "started" or "cancelled"
	Reason       string `json:"reason"` // Explanation
	Participants int    `json:"participants"`
}

// AutoStartMatch handles expired join deadline - starts match if 2+ participants, otherwise cancels
func (s *ArenaService) AutoStartMatch(ctx context.Context, matchID string) (*AutoStartResult, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:auto-start-match")
		defer segment.End()
	}

	match, err := s.gameRepo.GetMatch(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get match: %w", err)
	}
	if match == nil {
		return nil, ErrMatchNotFound
	}
	if match.Status != repository.MatchStatusOpen {
		return nil, ErrMatchNotOpen
	}

	participantCount, err := s.gameRepo.GetParticipantCount(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participant count: %w", err)
	}

	// Need at least 2 participants
	if participantCount < 2 {
		// Cancel the match
		if err := s.gameRepo.CancelMatch(ctx, matchID); err != nil {
			return nil, fmt.Errorf("failed to cancel match: %w", err)
		}
		return &AutoStartResult{
			MatchID:      matchID,
			ChatID:       match.ChatID,
			Action:       "cancelled",
			Reason:       "not enough participants (minimum 2 required)",
			Participants: participantCount,
		}, nil
	}

	// Start the match - determine format based on participant count
	format := repository.MatchFormat1v1
	if participantCount > 2 {
		format = repository.MatchFormatArena
	}

	// Start shop phase
	deadline := time.Now().Add(ShopPhaseDuration)
	if err := s.gameRepo.StartShopPhase(ctx, matchID, format, deadline); err != nil {
		return nil, fmt.Errorf("failed to start shop phase: %w", err)
	}

	// Deal cards to all participants
	participants, err := s.gameRepo.GetMatchParticipants(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participants: %w", err)
	}

	for _, p := range participants {
		cards, err := s.dealer.DealCards(ctx, match.ChatID, shop.ShopSize)
		if err != nil {
			return nil, fmt.Errorf("failed to deal cards for user %d: %w", p.UserID, err)
		}
		cardsJSON, err := json.Marshal(cards)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal cards for user %d: %w", p.UserID, err)
		}
		emptyTeam, err := json.Marshal([]*battle.Card{})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal empty team for user %d: %w", p.UserID, err)
		}
		if err := s.gameRepo.UpdateParticipantShop(ctx, matchID, p.UserID, shop.StartingCoins, cardsJSON, emptyTeam, []int64{0, 1, 2}); err != nil {
			return nil, fmt.Errorf("failed to initialize shop for user %d: %w", p.UserID, err)
		}
	}

	return &AutoStartResult{
		MatchID:      matchID,
		ChatID:       match.ChatID,
		Action:       "started",
		Reason:       fmt.Sprintf("match started with %d participants", participantCount),
		Participants: participantCount,
	}, nil
}

// ForceSubmitResult represents the result of a force-submit operation
type ForceSubmitResult struct {
	MatchID       string  `json:"match_id"`
	ChatID        int64   `json:"chat_id"`
	ForcedUsers   []int64 `json:"forced_users"` // Users who had their teams auto-submitted
	BattleStarted bool    `json:"battle_started"`
}

// ForceSubmitTeams auto-assigns teams for participants who haven't submitted.
// Uses a greedy strategy: auto-buys the 3 highest-ATK cards available.
// Called when shop phase deadline expires. Starts battle immediately after all teams are ready.
// Returns list of user IDs that were force-submitted and whether battle was started.
func (s *ArenaService) ForceSubmitTeams(ctx context.Context, matchID string) (*ForceSubmitResult, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:force-submit-teams")
		defer segment.End()
	}

	match, err := s.gameRepo.GetMatch(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get match: %w", err)
	}
	if match == nil {
		return nil, ErrMatchNotFound
	}
	if match.Status != repository.MatchStatusShopPhase {
		return nil, ErrMatchNotInShopPhase
	}

	participants, err := s.gameRepo.GetMatchParticipants(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participants: %w", err)
	}

	forcedUsers := make([]int64, 0)

	for _, p := range participants {
		if p.Status == repository.ParticipantStatusReady {
			continue // Already submitted
		}

		// Force submit for this participant
		if err := s.forceSubmitTeam(ctx, matchID, p); err != nil {
			return nil, fmt.Errorf("failed to force submit for user %d: %w", p.UserID, err)
		}
		forcedUsers = append(forcedUsers, p.UserID)
	}

	// Start battle now that all teams are submitted
	_, err = s.StartBattle(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to start battle: %w", err)
	}

	return &ForceSubmitResult{
		MatchID:       matchID,
		ChatID:        match.ChatID,
		ForcedUsers:   forcedUsers,
		BattleStarted: true,
	}, nil
}

// forceSubmitTeam auto-buys cards and submits team for a participant
func (s *ArenaService) forceSubmitTeam(ctx context.Context, matchID string, p *repository.ParticipantWithUser) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:force-submit-team")
		defer segment.End()
	}

	// Parse current shop state
	var cards []*battle.ShopCard
	if p.ShopCards != nil {
		if err := json.Unmarshal(*p.ShopCards, &cards); err != nil {
			return fmt.Errorf("failed to parse shop cards: %w", err)
		}
	}

	var team []*battle.Card
	if p.Team != nil {
		if err := json.Unmarshal(*p.Team, &team); err != nil {
			return fmt.Errorf("failed to parse team: %w", err)
		}
	}

	coins := p.CoinsRemaining

	// Auto-buy cards until we have 3 or run out of coins
	for len(team) < shop.TeamSize && coins >= shop.CardCost {
		// Find best available card (highest ATK)
		bestIdx := -1
		bestATK := -1
		for i, card := range cards {
			if card != nil && !card.IsPurchased && card.ATK > bestATK {
				bestATK = card.ATK
				bestIdx = i
			}
		}
		if bestIdx == -1 {
			break // No cards available
		}

		// Buy the card
		cards[bestIdx].IsPurchased = true
		team = append(team, cards[bestIdx].ToCard())
		coins -= shop.CardCost
	}

	// If still not enough cards, something is wrong
	if len(team) < shop.TeamSize {
		// Create dummy cards if needed (shouldn't happen normally)
		for len(team) < shop.TeamSize {
			if len(cards) > 0 {
				// Just grab any card
				for i, card := range cards {
					if card != nil && !card.IsPurchased {
						cards[i].IsPurchased = true
						team = append(team, card.ToCard())
						break
					}
				}
			}
			if len(team) < shop.TeamSize {
				break // Can't complete team
			}
		}
	}

	// Sort team by ATK descending (glass cannon strategy)
	// Order: [0]=highest ATK, [1]=second, [2]=lowest
	order := make([]int64, len(team))
	for i := range order {
		order[i] = int64(i)
	}

	// Sort by ATK
	for i := 0; i < len(team)-1; i++ {
		for j := i + 1; j < len(team); j++ {
			if team[order[j]].ATK > team[order[i]].ATK {
				order[i], order[j] = order[j], order[i]
			}
		}
	}

	// Save shop state
	cardsJSON, err := json.Marshal(cards)
	if err != nil {
		return fmt.Errorf("failed to marshal cards: %w", err)
	}
	teamJSON, err := json.Marshal(team)
	if err != nil {
		return fmt.Errorf("failed to marshal team: %w", err)
	}
	if err := s.gameRepo.UpdateParticipantShop(ctx, matchID, p.UserID, coins, cardsJSON, teamJSON, order); err != nil {
		return fmt.Errorf("failed to save shop state: %w", err)
	}

	// Submit team
	if err := s.gameRepo.SubmitTeam(ctx, matchID, p.UserID); err != nil {
		return fmt.Errorf("failed to submit team: %w", err)
	}

	return nil
}

// =====================================================
// RANKED TOURNAMENT METHODS
// =====================================================

// Tournament errors
var (
	ErrTournamentNotFound           = errors.New("tournament not found")
	ErrTournamentNotOpen            = errors.New("tournament is not open for registration")
	ErrTournamentRegistrationClosed = errors.New("tournament registration has closed")
	ErrAlreadyRegistered            = errors.New("already registered for this tournament")
	ErrNotRegistered                = errors.New("not registered for this tournament")
	ErrNoParticipants               = errors.New("no participants registered")
)

// TournamentResponse represents a tournament with participant info
type TournamentResponse struct {
	*repository.RankedTournament
	Participants []*repository.TournamentParticipant `json:"participants,omitempty"`
	CardCount    int                                 `json:"card_count"`
}

// TournamentStartResult represents the result of starting a tournament
type TournamentStartResult struct {
	Tournament       *repository.RankedTournament `json:"tournament"`
	Match            *repository.Match            `json:"match"`
	ParticipantCount int                          `json:"participant_count"`
	Format           repository.MatchFormat       `json:"format"`
	Skipped          bool                         `json:"skipped"`
	Reason           string                       `json:"reason,omitempty"`
}

// GetOrCreateTodayTournament gets or creates today's tournament for a chat
func (s *ArenaService) GetOrCreateTodayTournament(ctx context.Context, chatID int64, date string) (*TournamentResponse, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:get-or-create-today-tournament")
		defer segment.End()
	}

	tournament, err := s.gameRepo.GetOrCreateTournament(ctx, chatID, date)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create tournament: %w", err)
	}

	participants, err := s.gameRepo.GetTournamentParticipants(ctx, tournament.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participants: %w", err)
	}

	cardCount, _ := s.dealer.GetCardCount(ctx, chatID)

	return &TournamentResponse{
		RankedTournament: tournament,
		Participants:     participants,
		CardCount:        cardCount,
	}, nil
}

// GetTodayTournament retrieves today's tournament for a chat
func (s *ArenaService) GetTodayTournament(ctx context.Context, chatID int64, date string) (*TournamentResponse, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:get-today-tournament")
		defer segment.End()
	}

	tournament, err := s.gameRepo.GetTodayTournament(ctx, chatID, date)
	if err != nil {
		return nil, fmt.Errorf("failed to get tournament: %w", err)
	}
	if tournament == nil {
		return nil, nil
	}

	participants, err := s.gameRepo.GetTournamentParticipants(ctx, tournament.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participants: %w", err)
	}

	cardCount, _ := s.dealer.GetCardCount(ctx, chatID)

	return &TournamentResponse{
		RankedTournament: tournament,
		Participants:     participants,
		CardCount:        cardCount,
	}, nil
}

// GetTournamentByID retrieves a tournament by ID
func (s *ArenaService) GetTournamentByID(ctx context.Context, tournamentID int64) (*TournamentResponse, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:get-tournament-by-id")
		defer segment.End()
	}

	tournament, err := s.gameRepo.GetTournamentByID(ctx, tournamentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tournament: %w", err)
	}
	if tournament == nil {
		return nil, ErrTournamentNotFound
	}

	participants, err := s.gameRepo.GetTournamentParticipants(ctx, tournament.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participants: %w", err)
	}

	cardCount, _ := s.dealer.GetCardCount(ctx, tournament.ChatID)

	return &TournamentResponse{
		RankedTournament: tournament,
		Participants:     participants,
		CardCount:        cardCount,
	}, nil
}

// SetTournamentAnnounced marks a tournament as announced (open for registration)
func (s *ArenaService) SetTournamentAnnounced(ctx context.Context, tournamentID int64, messageID int64) error {
	return s.gameRepo.SetTournamentAnnounced(ctx, tournamentID, messageID)
}

// JoinTournament adds a user to a tournament
func (s *ArenaService) JoinTournament(ctx context.Context, tournamentID int64, userID int64) (*TournamentResponse, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:join-tournament")
		defer segment.End()
	}

	// Get tournament
	tournament, err := s.gameRepo.GetTournamentByID(ctx, tournamentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tournament: %w", err)
	}
	if tournament == nil {
		return nil, ErrTournamentNotFound
	}

	// Check tournament is open
	if tournament.Status != repository.TournamentStatusOpen {
		if tournament.Status == repository.TournamentStatusScheduled {
			return nil, ErrTournamentNotOpen
		}
		return nil, ErrTournamentRegistrationClosed
	}

	// Check if already registered
	isParticipant, err := s.gameRepo.IsTournamentParticipant(ctx, tournamentID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check participant: %w", err)
	}
	if isParticipant {
		return nil, ErrAlreadyRegistered
	}

	// Add participant
	if err := s.gameRepo.AddTournamentParticipant(ctx, tournamentID, userID); err != nil {
		return nil, fmt.Errorf("failed to add participant: %w", err)
	}

	// Return updated tournament
	return s.GetTournamentByID(ctx, tournamentID)
}

// LeaveTournament removes a user from a tournament
func (s *ArenaService) LeaveTournament(ctx context.Context, tournamentID int64, userID int64) (*TournamentResponse, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:leave-tournament")
		defer segment.End()
	}

	// Get tournament
	tournament, err := s.gameRepo.GetTournamentByID(ctx, tournamentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tournament: %w", err)
	}
	if tournament == nil {
		return nil, ErrTournamentNotFound
	}

	// Check tournament is still open
	if tournament.Status != repository.TournamentStatusOpen {
		return nil, ErrTournamentRegistrationClosed
	}

	// Check if registered
	isParticipant, err := s.gameRepo.IsTournamentParticipant(ctx, tournamentID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check participant: %w", err)
	}
	if !isParticipant {
		return nil, ErrNotRegistered
	}

	// Remove participant
	if err := s.gameRepo.RemoveTournamentParticipant(ctx, tournamentID, userID); err != nil {
		return nil, fmt.Errorf("failed to remove participant: %w", err)
	}

	// Return updated tournament
	return s.GetTournamentByID(ctx, tournamentID)
}

// GetTournamentsNeedingAnnouncement returns tournaments that need to be announced
func (s *ArenaService) GetTournamentsNeedingAnnouncement(ctx context.Context, currentTime time.Time) ([]*repository.TournamentInfo, error) {
	return s.gameRepo.GetTournamentsNeedingAnnouncement(ctx, currentTime)
}

// GetTournamentsNeedingClose returns tournaments that need registration closed
func (s *ArenaService) GetTournamentsNeedingClose(ctx context.Context, currentTime time.Time) ([]*repository.TournamentInfo, error) {
	return s.gameRepo.GetTournamentsNeedingClose(ctx, currentTime)
}

// CloseAndStartTournament closes registration and starts the tournament match
func (s *ArenaService) CloseAndStartTournament(ctx context.Context, tournamentID int64) (*TournamentStartResult, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:close-and-start-tournament")
		defer segment.End()
	}

	// Get tournament
	tournament, err := s.gameRepo.GetTournamentByID(ctx, tournamentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tournament: %w", err)
	}
	if tournament == nil {
		return nil, ErrTournamentNotFound
	}

	// Check tournament is open
	if tournament.Status != repository.TournamentStatusOpen {
		return nil, fmt.Errorf("tournament is not open (status: %s)", tournament.Status)
	}

	// Get participants
	participants, err := s.gameRepo.GetTournamentParticipants(ctx, tournament.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participants: %w", err)
	}

	// Check participant count
	if len(participants) < 2 {
		// Skip tournament - not enough participants
		if err := s.gameRepo.SkipTournament(ctx, tournamentID); err != nil {
			return nil, fmt.Errorf("failed to skip tournament: %w", err)
		}

		reason := "not enough participants"
		if len(participants) == 0 {
			reason = "no participants"
		} else {
			reason = "only 1 participant"
		}

		// Refetch tournament
		tournament, _ = s.gameRepo.GetTournamentByID(ctx, tournamentID)

		return &TournamentStartResult{
			Tournament:       tournament,
			ParticipantCount: len(participants),
			Skipped:          true,
			Reason:           reason,
		}, nil
	}

	// Determine format based on participant count
	var format repository.MatchFormat
	if len(participants) == 2 {
		format = repository.MatchFormat1v1
	} else {
		format = repository.MatchFormatArena
	}

	// Create ranked match
	tournamentDate := tournament.TournamentDate
	match, err := s.gameRepo.CreateMatch(ctx, tournament.ChatID, repository.MatchTypeRanked, nil, &tournamentDate)
	if err != nil {
		return nil, fmt.Errorf("failed to create match: %w", err)
	}

	// Add all tournament participants to the match
	for _, p := range participants {
		if _, err := s.gameRepo.AddParticipant(ctx, match.ID, p.UserID); err != nil {
			return nil, fmt.Errorf("failed to add participant to match: %w", err)
		}
	}

	// Start shop phase
	shopDeadline := time.Now().Add(ShopPhaseDuration)
	if err := s.gameRepo.StartShopPhase(ctx, match.ID, format, shopDeadline); err != nil {
		return nil, fmt.Errorf("failed to start shop phase: %w", err)
	}

	// Deal cards to all participants
	if err := s.dealCardsToAllParticipants(ctx, match.ID, tournament.ChatID); err != nil {
		return nil, fmt.Errorf("failed to deal cards: %w", err)
	}

	// Link tournament to match and update status
	if err := s.gameRepo.CloseTournamentRegistration(ctx, tournamentID, match.ID); err != nil {
		return nil, fmt.Errorf("failed to close tournament registration: %w", err)
	}

	// Refetch updated data
	tournament, _ = s.gameRepo.GetTournamentByID(ctx, tournamentID)
	match, _ = s.gameRepo.GetMatch(ctx, match.ID)

	// Record tournament start metric
	recordTournamentEvent(s.nrApp, "started", tournamentID, tournament.ChatID, len(participants))

	return &TournamentStartResult{
		Tournament:       tournament,
		Match:            match,
		ParticipantCount: len(participants),
		Format:           format,
		Skipped:          false,
	}, nil
}

// dealCardsToAllParticipants deals shop cards to all match participants
func (s *ArenaService) dealCardsToAllParticipants(ctx context.Context, matchID string, chatID int64) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:deal-cards-to-all-participants")
		defer segment.End()
	}

	participants, err := s.gameRepo.GetMatchParticipants(ctx, matchID)
	if err != nil {
		return fmt.Errorf("failed to get participants: %w", err)
	}

	for _, p := range participants {
		// Deal cards
		cards, err := s.dealer.DealCards(ctx, chatID, shop.ShopSize)
		if err != nil {
			return fmt.Errorf("failed to deal cards for user %d: %w", p.UserID, err)
		}

		// Save shop state
		cardsJSON, err := json.Marshal(cards)
		if err != nil {
			return fmt.Errorf("failed to marshal cards: %w", err)
		}

		emptyTeam := json.RawMessage("[]")
		if err := s.gameRepo.UpdateParticipantShop(ctx, matchID, p.UserID, shop.StartingCoins, cardsJSON, emptyTeam, []int64{0, 1, 2}); err != nil {
			return fmt.Errorf("failed to update shop state for user %d: %w", p.UserID, err)
		}
	}

	return nil
}

// GetTournamentsWithPendingRounds returns tournaments that need next round execution
func (s *ArenaService) GetTournamentsWithPendingRounds(ctx context.Context) ([]*repository.RankedTournament, error) {
	return s.gameRepo.GetTournamentsWithPendingRounds(ctx)
}

// CompleteTournament marks a tournament as completed
func (s *ArenaService) CompleteTournament(ctx context.Context, tournamentID int64, winnerUserID *int64) error {
	return s.gameRepo.CompleteTournament(ctx, tournamentID, winnerUserID)
}

// GetChatsWithTimezone returns all chats with their timezone settings
func (s *ArenaService) GetChatsWithTimezone(ctx context.Context) ([]*repository.ChatTimezone, error) {
	return s.gameRepo.GetChatsWithTimezone(ctx)
}

// GetMatchHistory retrieves a user's match history
func (s *ArenaService) GetMatchHistory(ctx context.Context, chatID, userID int64, limit, offset int) ([]*repository.MatchHistoryEntry, int, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:get-matchHistory")
		defer segment.End()
	}

	return s.gameRepo.GetMatchHistory(ctx, chatID, userID, limit, offset)
}

// GetH2HRecord retrieves head-to-head record against a specific opponent
func (s *ArenaService) GetH2HRecord(ctx context.Context, chatID, userID, opponentID int64) (*repository.H2HRecord, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:get-h2h-record")
		defer segment.End()
	}

	return s.gameRepo.GetH2HRecord(ctx, chatID, userID, opponentID)
}

// GetRecentMatchesVsOpponent retrieves recent matches against a specific opponent
func (s *ArenaService) GetRecentMatchesVsOpponent(ctx context.Context, chatID, userID, opponentID int64, limit int) ([]*repository.MatchHistoryEntry, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:get-recent-matches-vs-opponent")
		defer segment.End()
	}

	return s.gameRepo.GetRecentMatchesVsOpponent(ctx, chatID, userID, opponentID, limit)
}

// ArenaProfileResponse represents a user's arena profile
type ArenaProfileResponse struct {
	*repository.UserProfile
	RecentMatches []*repository.MatchHistoryEntry `json:"recent_matches"`
}

// GetProfile retrieves a user's profile with stats, ranks, and recent matches
func (s *ArenaService) GetProfile(ctx context.Context, chatID, userID int64) (*ArenaProfileResponse, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("service:arena:get-profile")
		defer segment.End()
	}

	// Get user profile with rank positions
	profile, err := s.gameRepo.GetUserProfile(ctx, chatID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user profile: %w", err)
	}
	if profile == nil {
		return nil, nil
	}

	// Get recent matches (limit to 10)
	matches, _, err := s.gameRepo.GetMatchHistory(ctx, chatID, userID, 10, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent matches: %w", err)
	}

	return &ArenaProfileResponse{
		UserProfile:   profile,
		RecentMatches: matches,
	}, nil
}

// GetPhotoPresignedURL generates a presigned URL for a photo object key
func (s *ArenaService) GetPhotoPresignedURL(ctx context.Context, objectKey string) (string, error) {
	if objectKey == "" {
		return "", nil
	}
	return s.storageClient.GetPresignedURL(ctx, objectKey, 24*time.Hour)
}
