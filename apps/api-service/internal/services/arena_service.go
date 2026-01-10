package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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
	ErrNotEnoughCards      = errors.New("not enough cards in group (minimum 10 required)")
	ErrMatchNotFound       = errors.New("match not found")
	ErrMatchNotOpen        = errors.New("match is not open for joining")
	ErrAlreadyJoined       = errors.New("already joined this match")
	ErrNotParticipant      = errors.New("not a participant in this match")
	ErrNotCreator          = errors.New("only the match creator can perform this action")
	ErrMatchNotInShopPhase = errors.New("match is not in shop phase")
	ErrShopPhaseExpired    = errors.New("shop phase has expired")
	ErrTeamAlreadySubmitted = errors.New("team already submitted")
	ErrInvalidCardIndex    = errors.New("invalid card index")
	ErrNotEnoughCoins      = errors.New("not enough coins")
	ErrTeamFull            = errors.New("team is full (max 3 cards)")
	ErrCardAlreadyPurchased = errors.New("card already purchased")
)

// ArenaService handles arena game logic
type ArenaService struct {
	db            *sql.DB
	gameRepo      *repository.GameRepository
	dealer        *shop.Dealer
	storageClient *storage.MinIOClient
	nrApp         *newrelic.Application
}

// NewArenaService creates a new arena service
func NewArenaService(
	db *sql.DB,
	gameRepo *repository.GameRepository,
	storageClient *storage.MinIOClient,
	nrApp *newrelic.Application,
) *ArenaService {
	return &ArenaService{
		db:            db,
		gameRepo:      gameRepo,
		dealer:        shop.NewDealer(db, nrApp),
		storageClient: storageClient,
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
	MatchID       string              `json:"match_id"`
	Status        string              `json:"status"`
	Coins         int                 `json:"coins"`
	Cards         []*battle.ShopCard  `json:"cards"`
	Team          []*battle.Card      `json:"team"`
	TeamOrder     []int               `json:"team_order"`
	IsReady       bool                `json:"is_ready"`
	Deadline      *time.Time          `json:"deadline,omitempty"`
	TimeRemaining int                 `json:"time_remaining_seconds"`
}

// BattleResponse represents battle results
type BattleResponse struct {
	MatchID    string                    `json:"match_id"`
	Status     string                    `json:"status"`
	Rounds     []*repository.MatchRound  `json:"rounds"`
	WinnerID   *int64                    `json:"winner_id,omitempty"`
	IsComplete bool                      `json:"is_complete"`
}

// CreateMatch creates a new regular match
func (s *ArenaService) CreateMatch(ctx context.Context, chatID int64, creatorUserID int64) (*MatchResponse, error) {
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

	return &MatchResponse{
		Match:        match,
		Participants: participants,
		CardCount:    cardCount,
	}, nil
}

// GetMatch retrieves a match with participants
func (s *ArenaService) GetMatch(ctx context.Context, matchID string) (*MatchResponse, error) {
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
	if err := s.gameRepo.UpdateMatchFormat(ctx, matchID, format); err != nil {
		return nil, fmt.Errorf("failed to update match format: %w", err)
	}

	// Start shop phase
	deadline := time.Now().Add(ShopPhaseDuration)
	if err := s.gameRepo.StartShopPhase(ctx, matchID, deadline); err != nil {
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
	participants, err := s.gameRepo.GetMatchParticipants(ctx, matchID)
	if err != nil {
		return err
	}

	for _, p := range participants {
		cards, err := s.dealer.DealCards(ctx, chatID, shop.ShopSize)
		if err != nil {
			return fmt.Errorf("failed to deal cards for user %d: %w", p.UserID, err)
		}

		shopCardsJSON, _ := json.Marshal(cards)
		teamJSON, _ := json.Marshal([]*battle.Card{})
		teamOrder := []int{0, 1, 2}

		if err := s.gameRepo.UpdateParticipantShop(ctx, matchID, p.UserID, shop.StartingCoins, shopCardsJSON, teamJSON, teamOrder); err != nil {
			return fmt.Errorf("failed to save shop state for user %d: %w", p.UserID, err)
		}
	}

	return nil
}

// GetShop retrieves the shop state for a player
func (s *ArenaService) GetShop(ctx context.Context, matchID string, userID int64) (*ShopResponse, error) {
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

	// Parse shop cards
	var cards []*battle.ShopCard
	if participant.ShopCards != nil {
		if err := json.Unmarshal(participant.ShopCards, &cards); err != nil {
			return nil, fmt.Errorf("failed to parse shop cards: %w", err)
		}
	}

	// Parse team
	var team []*battle.Card
	if participant.Team != nil {
		if err := json.Unmarshal(participant.Team, &team); err != nil {
			return nil, fmt.Errorf("failed to parse team: %w", err)
		}
	}

	// Calculate time remaining
	var timeRemaining int
	if match.ShopPhaseDeadline != nil {
		remaining := time.Until(*match.ShopPhaseDeadline)
		if remaining > 0 {
			timeRemaining = int(remaining.Seconds())
		}
	}

	return &ShopResponse{
		MatchID:       matchID,
		Status:        string(match.Status),
		Coins:         participant.CoinsRemaining,
		Cards:         cards,
		Team:          team,
		TeamOrder:     participant.TeamOrder,
		IsReady:       participant.Status == repository.ParticipantStatusReady,
		Deadline:      match.ShopPhaseDeadline,
		TimeRemaining: timeRemaining,
	}, nil
}

// BuyCard purchases a card from the shop
func (s *ArenaService) BuyCard(ctx context.Context, matchID string, userID int64, cardIndex int) (*ShopResponse, error) {
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
	if match.ShopPhaseDeadline != nil && time.Now().After(*match.ShopPhaseDeadline) {
		return nil, ErrShopPhaseExpired
	}

	participant, err := s.gameRepo.GetParticipant(ctx, matchID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participant: %w", err)
	}
	if participant == nil {
		return nil, ErrNotParticipant
	}
	if participant.Status == repository.ParticipantStatusReady {
		return nil, ErrTeamAlreadySubmitted
	}

	// Parse current state
	var cards []*battle.ShopCard
	if err := json.Unmarshal(participant.ShopCards, &cards); err != nil {
		return nil, fmt.Errorf("failed to parse shop cards: %w", err)
	}

	var team []*battle.Card
	if participant.Team != nil {
		if err := json.Unmarshal(participant.Team, &team); err != nil {
			return nil, fmt.Errorf("failed to parse team: %w", err)
		}
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
	team = append(team, newCard)
	coins := participant.CoinsRemaining - shop.CardCost

	// Save state
	shopCardsJSON, _ := json.Marshal(cards)
	teamJSON, _ := json.Marshal(team)

	if err := s.gameRepo.UpdateParticipantShop(ctx, matchID, userID, coins, shopCardsJSON, teamJSON, participant.TeamOrder); err != nil {
		return nil, fmt.Errorf("failed to save shop state: %w", err)
	}

	return s.GetShop(ctx, matchID, userID)
}

// Reroll replaces unpurchased cards with new ones
func (s *ArenaService) Reroll(ctx context.Context, matchID string, userID int64) (*ShopResponse, error) {
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
	if match.ShopPhaseDeadline != nil && time.Now().After(*match.ShopPhaseDeadline) {
		return nil, ErrShopPhaseExpired
	}

	participant, err := s.gameRepo.GetParticipant(ctx, matchID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participant: %w", err)
	}
	if participant == nil {
		return nil, ErrNotParticipant
	}
	if participant.Status == repository.ParticipantStatusReady {
		return nil, ErrTeamAlreadySubmitted
	}
	if participant.CoinsRemaining < shop.RerollCost {
		return nil, ErrNotEnoughCoins
	}

	// Parse current cards
	var cards []*battle.ShopCard
	if err := json.Unmarshal(participant.ShopCards, &cards); err != nil {
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
	shopCardsJSON, _ := json.Marshal(cards)

	var team []*battle.Card
	if participant.Team != nil {
		json.Unmarshal(participant.Team, &team)
	}
	teamJSON, _ := json.Marshal(team)

	if err := s.gameRepo.UpdateParticipantShop(ctx, matchID, userID, coins, shopCardsJSON, teamJSON, participant.TeamOrder); err != nil {
		return nil, fmt.Errorf("failed to save shop state: %w", err)
	}

	return s.GetShop(ctx, matchID, userID)
}

// UpgradeCard applies an upgrade to a team card
func (s *ArenaService) UpgradeCard(ctx context.Context, matchID string, userID int64, teamSlot int, upgradeType shop.UpgradeType) (*ShopResponse, error) {
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
	if match.ShopPhaseDeadline != nil && time.Now().After(*match.ShopPhaseDeadline) {
		return nil, ErrShopPhaseExpired
	}

	participant, err := s.gameRepo.GetParticipant(ctx, matchID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participant: %w", err)
	}
	if participant == nil {
		return nil, ErrNotParticipant
	}
	if participant.Status == repository.ParticipantStatusReady {
		return nil, ErrTeamAlreadySubmitted
	}
	if participant.CoinsRemaining < shop.UpgradeCost {
		return nil, ErrNotEnoughCoins
	}

	// Parse team
	var team []*battle.Card
	if participant.Team != nil {
		if err := json.Unmarshal(participant.Team, &team); err != nil {
			return nil, fmt.Errorf("failed to parse team: %w", err)
		}
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
	json.Unmarshal(participant.ShopCards, &cards)
	shopCardsJSON, _ := json.Marshal(cards)
	teamJSON, _ := json.Marshal(team)

	if err := s.gameRepo.UpdateParticipantShop(ctx, matchID, userID, coins, shopCardsJSON, teamJSON, participant.TeamOrder); err != nil {
		return nil, fmt.Errorf("failed to save shop state: %w", err)
	}

	return s.GetShop(ctx, matchID, userID)
}

// SetTeamOrder sets the battle order for a player's team
func (s *ArenaService) SetTeamOrder(ctx context.Context, matchID string, userID int64, order []int) (*ShopResponse, error) {
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

	participant, err := s.gameRepo.GetParticipant(ctx, matchID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participant: %w", err)
	}
	if participant == nil {
		return nil, ErrNotParticipant
	}
	if participant.Status == repository.ParticipantStatusReady {
		return nil, ErrTeamAlreadySubmitted
	}

	// Validate order
	var team []*battle.Card
	if participant.Team != nil {
		json.Unmarshal(participant.Team, &team)
	}
	if len(order) != len(team) {
		return nil, errors.New("order length must match team size")
	}

	// Save state
	var cards []*battle.ShopCard
	json.Unmarshal(participant.ShopCards, &cards)
	shopCardsJSON, _ := json.Marshal(cards)
	teamJSON, _ := json.Marshal(team)

	if err := s.gameRepo.UpdateParticipantShop(ctx, matchID, userID, participant.CoinsRemaining, shopCardsJSON, teamJSON, order); err != nil {
		return nil, fmt.Errorf("failed to save shop state: %w", err)
	}

	return s.GetShop(ctx, matchID, userID)
}

// SubmitTeam submits the team for battle
func (s *ArenaService) SubmitTeam(ctx context.Context, matchID string, userID int64) (*ShopResponse, error) {
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

	participant, err := s.gameRepo.GetParticipant(ctx, matchID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participant: %w", err)
	}
	if participant == nil {
		return nil, ErrNotParticipant
	}
	if participant.Status == repository.ParticipantStatusReady {
		return nil, ErrTeamAlreadySubmitted
	}

	// Validate team has 3 cards
	var team []*battle.Card
	if participant.Team != nil {
		json.Unmarshal(participant.Team, &team)
	}
	if len(team) != shop.TeamSize {
		return nil, shop.ErrTeamIncomplete
	}

	// Submit team
	if err := s.gameRepo.SubmitTeam(ctx, matchID, userID); err != nil {
		return nil, fmt.Errorf("failed to submit team: %w", err)
	}

	// Check if all participants are ready
	go s.checkAndStartBattle(context.Background(), matchID)

	return s.GetShop(ctx, matchID, userID)
}

// checkAndStartBattle checks if all participants are ready and starts battle
func (s *ArenaService) checkAndStartBattle(ctx context.Context, matchID string) {
	total, _ := s.gameRepo.GetParticipantCount(ctx, matchID)
	ready, _ := s.gameRepo.GetReadyParticipantCount(ctx, matchID)

	if ready >= total && total >= 2 {
		s.StartBattle(ctx, matchID)
	}
}

// StartBattle initiates the battle phase
func (s *ArenaService) StartBattle(ctx context.Context, matchID string) (*BattleResponse, error) {
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

// runBattle executes a single battle between two participants
func (s *ArenaService) runBattle(ctx context.Context, matchID string, pA, pB *repository.ParticipantWithUser, roundNumber int) (*BattleResponse, error) {
	// Parse teams
	var teamACards, teamBCards []*battle.Card
	json.Unmarshal(pA.Team, &teamACards)
	json.Unmarshal(pB.Team, &teamBCards)

	// Apply team order
	orderedA := make([]*battle.Card, len(teamACards))
	orderedB := make([]*battle.Card, len(teamBCards))
	for i, idx := range pA.TeamOrder {
		if idx >= 0 && idx < len(teamACards) {
			orderedA[i] = teamACards[idx]
		}
	}
	for i, idx := range pB.TeamOrder {
		if idx >= 0 && idx < len(teamBCards) {
			orderedB[i] = teamBCards[idx]
		}
	}

	teamA := battle.NewTeam(pA.UserID, orderedA)
	teamB := battle.NewTeam(pB.UserID, orderedB)

	// Run battle simulation
	result := battle.Simulate(teamA, teamB)

	// Save round
	teamAJSON, _ := json.Marshal(orderedA)
	teamBJSON, _ := json.Marshal(orderedB)
	eventsJSON, _ := json.Marshal(result.Events)

	_, err := s.gameRepo.CreateRound(ctx, matchID, roundNumber,
		pA.UserID, pB.UserID,
		teamAJSON, teamBJSON, eventsJSON,
		result.WinnerID, result.IsDraw,
		result.TeamADamage, result.TeamBDamage, result.NumRounds)
	if err != nil {
		return nil, fmt.Errorf("failed to save round: %w", err)
	}

	// Update leaderboard
	match, _ := s.gameRepo.GetMatch(ctx, matchID)
	matchType := repository.MatchTypeRegular
	if match != nil {
		matchType = match.MatchType
	}

	if result.WinnerID != nil {
		if *result.WinnerID == pA.UserID {
			s.gameRepo.UpdateLeaderboard(ctx, pA.UserID, match.ChatID, matchType, true, &pB.UserID, false)
			s.gameRepo.UpdateLeaderboard(ctx, pB.UserID, match.ChatID, matchType, false, &pA.UserID, false)
		} else {
			s.gameRepo.UpdateLeaderboard(ctx, pB.UserID, match.ChatID, matchType, true, &pA.UserID, false)
			s.gameRepo.UpdateLeaderboard(ctx, pA.UserID, match.ChatID, matchType, false, &pB.UserID, false)
		}
	}

	// Complete match for 1v1
	s.gameRepo.CompleteMatch(ctx, matchID, result.WinnerID)

	rounds, _ := s.gameRepo.GetMatchRounds(ctx, matchID)

	return &BattleResponse{
		MatchID:    matchID,
		Status:     string(repository.MatchStatusCompleted),
		Rounds:     rounds,
		WinnerID:   result.WinnerID,
		IsComplete: true,
	}, nil
}

// runArena executes arena format (round-robin for now)
func (s *ArenaService) runArena(ctx context.Context, matchID string, participants []*repository.ParticipantWithUser) (*BattleResponse, error) {
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

	// Determine winner by most wins
	participants, _ = s.gameRepo.GetMatchParticipants(ctx, matchID)
	var winner *repository.ParticipantWithUser
	maxWins := -1
	for _, p := range participants {
		if p.Wins > maxWins {
			maxWins = p.Wins
			winner = p
		}
	}

	var winnerID *int64
	if winner != nil {
		winnerID = &winner.UserID
	}

	s.gameRepo.CompleteMatch(ctx, matchID, winnerID)

	rounds, _ := s.gameRepo.GetMatchRounds(ctx, matchID)

	return &BattleResponse{
		MatchID:    matchID,
		Status:     string(repository.MatchStatusCompleted),
		Rounds:     rounds,
		WinnerID:   winnerID,
		IsComplete: true,
	}, nil
}

// GetBattle retrieves battle results
func (s *ArenaService) GetBattle(ctx context.Context, matchID string, userID int64) (*BattleResponse, error) {
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

	return &BattleResponse{
		MatchID:    matchID,
		Status:     string(match.Status),
		Rounds:     rounds,
		WinnerID:   match.WinnerUserID,
		IsComplete: match.Status == repository.MatchStatusCompleted,
	}, nil
}

// GetLeaderboard retrieves leaderboard for a chat
func (s *ArenaService) GetLeaderboard(ctx context.Context, chatID int64, matchType string, limit, offset int) ([]*repository.LeaderboardEntry, error) {
	mt := repository.MatchTypeRanked
	if matchType == "regular" {
		mt = repository.MatchTypeRegular
	}
	return s.gameRepo.GetLeaderboard(ctx, chatID, mt, limit, offset)
}
