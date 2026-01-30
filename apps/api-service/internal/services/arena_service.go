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

// Arena game constants
const (
	MinimumCardsRequired = 10
	ShopPhaseDuration    = 3 * time.Minute
	JoinWindowDuration   = 5 * time.Minute
)


// ArenaService handles arena game logic
type ArenaService struct {
	db                *sql.DB
	gameRepo          repository.GameRepositoryInterface
	dealer            *shop.Dealer
	shopService       *ShopService
	battleService     *BattleService
	tournamentService *TournamentService
	storageClient     MinIOClientInterface
	cardService       CardServiceInterface
	nrApp             *newrelic.Application
}

// ArenaServiceDeps holds the dependencies for ArenaService.
// This struct enables dependency injection for testing.
type ArenaServiceDeps struct {
	GameRepo      repository.GameRepositoryInterface
	StorageClient MinIOClientInterface
	CardService   CardServiceInterface
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

// NewArenaService creates a new ArenaService with all required dependencies.
// For production use, pass nil for deps to use default dependencies.
// For testing, provide mock implementations via deps.
func NewArenaService(
	db *sql.DB,
	storageClient MinIOClientInterface,
	cardService CardServiceInterface,
	nrApp *newrelic.Application,
	deps *ArenaServiceDeps,
) *ArenaService {
	svc := &ArenaService{
		db:    db,
		nrApp: nrApp,
	}

	if deps != nil {
		// Use provided dependencies (for testing)
		svc.gameRepo = deps.GameRepo
		svc.storageClient = deps.StorageClient
		svc.cardService = deps.CardService
	} else {
		// Use default concrete implementations (for production)
		svc.gameRepo = repository.NewGameRepository(db, nrApp)
		svc.storageClient = storageClient
		svc.cardService = cardService
	}

	// Create dealer with the resolved dependencies
	svc.dealer = shop.NewDealer(db, nrApp, svc.storageClient, svc.cardService)

	// Create shop service with the resolved dependencies
	svc.shopService = NewShopService(db, svc.gameRepo, svc.dealer, nrApp)

	// Create battle service with the resolved dependencies
	svc.battleService = NewBattleService(db, svc.gameRepo, nrApp)

	// Create tournament service with the resolved dependencies
	svc.tournamentService = NewTournamentService(db, svc.gameRepo, svc.dealer, nrApp)

	return svc
}

// MatchResponse represents a match with participant info
type MatchResponse struct {
	*repository.Match
	Participants []*repository.ParticipantWithUser `json:"participants"`
	CardCount    int                               `json:"card_count"`
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
	Combats     []battle.Combat      `json:"combats"`
	Events      []battle.BattleEvent `json:"events"`
	NumCombats  int                  `json:"num_combats"`
	NumRounds   int                  `json:"num_rounds"`
	TeamADamage int                  `json:"team_a_damage"`
	TeamBDamage int                  `json:"team_b_damage"`
	DamageDealt int                  `json:"damage_dealt"`
	DamageTaken int                  `json:"damage_taken"`
	TeamAFinal  *battle.Team         `json:"team_a_final"`
	TeamBFinal  *battle.Team         `json:"team_b_final"`
	PlayerAID   int64                `json:"player_a_id"`
	PlayerBID   int64                `json:"player_b_id"`
	PlayerAName string               `json:"player_a_name"`
	PlayerBName string               `json:"player_b_name"`
}

// CreateMatch creates a new regular match
func (s *ArenaService) CreateMatch(ctx context.Context, chatID int64, creatorUserID int64) (*MatchResponse, error) {
	defer nrutil.StartSegment(ctx, "service:arena:create-match")()

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
			return nil, apperror.ErrActiveMatchExists
		}
	}

	// Check minimum cards
	cardCount, err := s.dealer.GetCardCount(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get card count: %w", err)
	}
	if cardCount < MinimumCardsRequired {
		return nil, apperror.ErrNotEnoughCards
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

	// Populate photo URLs
	s.populateParticipantPhotoURLs(ctx, participants)

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
	defer nrutil.StartSegment(ctx, "service:arena:get-match")()

	match, err := s.gameRepo.GetMatch(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get match: %w", err)
	}
	if match == nil {
		return nil, apperror.ErrMatchNotFound
	}

	participants, err := s.gameRepo.GetMatchParticipants(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participants: %w", err)
	}

	// Populate photo URLs
	s.populateParticipantPhotoURLs(ctx, participants)

	cardCount, _ := s.dealer.GetCardCount(ctx, match.ChatID)

	return &MatchResponse{
		Match:        match,
		Participants: participants,
		CardCount:    cardCount,
	}, nil
}

// GetActiveMatches retrieves active matches for a chat
func (s *ArenaService) GetActiveMatches(ctx context.Context, chatID int64) ([]*MatchResponse, error) {
	defer nrutil.StartSegment(ctx, "service:arena:get-active-matches")()

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
		// Populate photo URLs
		s.populateParticipantPhotoURLs(ctx, participants)
		responses = append(responses, &MatchResponse{
			Match:        match,
			Participants: participants,
		})
	}

	return responses, nil
}

// GetUserActiveMatch retrieves a user's active match in a specific chat
func (s *ArenaService) GetUserActiveMatch(ctx context.Context, chatID, userID int64) (*MatchResponse, error) {
	defer nrutil.StartSegment(ctx, "service:arena:get-user-active-match")()

	match, err := s.gameRepo.GetUserActiveMatch(ctx, chatID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user active match: %w", err)
	}

	// Return nil if no active match found
	if match == nil {
		return nil, nil
	}

	// Get participants for the match
	participants, err := s.gameRepo.GetMatchParticipants(ctx, match.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get match participants: %w", err)
	}

	// Populate photo URLs
	s.populateParticipantPhotoURLs(ctx, participants)

	return &MatchResponse{
		Match:        match,
		Participants: participants,
	}, nil
}

// JoinMatch adds a user to a match
func (s *ArenaService) JoinMatch(ctx context.Context, matchID string, userID int64) (*MatchResponse, error) {
	defer nrutil.StartSegment(ctx, "service:arena:join-match")()

	match, err := s.gameRepo.GetMatch(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get match: %w", err)
	}
	if match == nil {
		return nil, apperror.ErrMatchNotFound
	}

	// Check match is open
	if match.Status != repository.MatchStatusOpen {
		return nil, apperror.ErrMatchNotOpen
	}

	// Check join deadline for regular matches
	if match.JoinDeadline != nil && time.Now().After(*match.JoinDeadline) {
		return nil, apperror.ErrMatchNotOpen
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
	defer nrutil.StartSegment(ctx, "service:arena:leave-match")()

	match, err := s.gameRepo.GetMatch(ctx, matchID)
	if err != nil {
		return fmt.Errorf("failed to get match: %w", err)
	}
	if match == nil {
		return apperror.ErrMatchNotFound
	}

	// Can only leave during open phase
	if match.Status != repository.MatchStatusOpen {
		return apperror.ErrMatchNotOpen
	}

	return s.gameRepo.RemoveParticipant(ctx, matchID, userID)
}

// StartMatch starts a match early (creator only)
func (s *ArenaService) StartMatch(ctx context.Context, matchID string, userID int64) (*MatchResponse, error) {
	defer nrutil.StartSegment(ctx, "service:arena:start-match")()

	match, err := s.gameRepo.GetMatch(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get match: %w", err)
	}
	if match == nil {
		return nil, apperror.ErrMatchNotFound
	}

	// Verify creator
	if match.CreatorUserID == nil || *match.CreatorUserID != userID {
		return nil, apperror.ErrNotCreator
	}

	// Check match is open
	if match.Status != repository.MatchStatusOpen {
		return nil, apperror.ErrMatchNotOpen
	}

	// Get participant count
	count, err := s.gameRepo.GetParticipantCount(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participant count: %w", err)
	}

	// Need at least 2 participants
	if count < 2 {
		return nil, apperror.ErrNotEnoughParticipants
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
	defer nrutil.StartSegment(ctx, "service:arena:deal-cards-to-participants")()

	participants, err := s.gameRepo.GetMatchParticipants(ctx, matchID)
	if err != nil {
		return err
	}

	for _, p := range participants {
		cards, err := s.dealer.DealCards(ctx, chatID, shop.ShopSize)
		if err != nil {
			return fmt.Errorf("failed to deal cards for user %d: %w", p.UserID, err)
		}

		shopCardsJSON, err := jsonutil.Marshal(cards)
		if err != nil {
			return fmt.Errorf("failed to marshal shop cards for user %d: %w", p.UserID, err)
		}
		teamJSON, err := jsonutil.Marshal([]*battle.Card{})
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

// GetLeaderboard retrieves leaderboard for a chat with total count
func (s *ArenaService) GetLeaderboard(ctx context.Context, chatID int64, matchType string, limit, offset int) ([]*repository.LeaderboardEntry, int, error) {
	defer nrutil.StartSegment(ctx, "service:arena:get-leaderboard")()

	mt := repository.MatchTypeRanked
	if matchType == "regular" {
		mt = repository.MatchTypeRegular
	}
	entries, total, err := s.gameRepo.GetLeaderboard(ctx, chatID, mt, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	// Populate photo URLs
	s.populateLeaderboardPhotoURLs(ctx, entries)

	return entries, total, nil
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
	defer nrutil.StartSegment(ctx, "service:arena:get-pending-matches")()

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
	defer nrutil.StartSegment(ctx, "service:arena:auto-start-match")()

	match, err := s.gameRepo.GetMatch(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get match: %w", err)
	}
	if match == nil {
		return nil, apperror.ErrMatchNotFound
	}
	if match.Status != repository.MatchStatusOpen {
		return nil, apperror.ErrMatchNotOpen
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
		cardsJSON, err := jsonutil.Marshal(cards)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal cards for user %d: %w", p.UserID, err)
		}
		emptyTeam, err := jsonutil.Marshal([]*battle.Card{})
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
// Uses a greedy strategy: auto-buys the highest-ATK cards available.
// Falls back to any available card if not enough high-ATK cards remain.
// Called when shop phase deadline expires. Starts battle immediately after all teams are ready.
// Returns list of user IDs that were force-submitted and whether battle was started.
func (s *ArenaService) ForceSubmitTeams(ctx context.Context, matchID string) (*ForceSubmitResult, error) {
	defer nrutil.StartSegment(ctx, "service:arena:force-submit-teams")()

	match, err := s.gameRepo.GetMatch(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get match: %w", err)
	}
	if match == nil {
		return nil, apperror.ErrMatchNotFound
	}
	if match.Status != repository.MatchStatusShopPhase {
		return nil, apperror.ErrMatchNotInShopPhase
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
	defer nrutil.StartSegment(ctx, "service:arena:force-submit-team")()

	// Parse current shop state
	var cards []*battle.ShopCard
	if p.ShopCards != nil {
		if err := jsonutil.Unmarshal(*p.ShopCards, &cards); err != nil {
			return fmt.Errorf("failed to parse shop cards: %w", err)
		}
	}

	var team []*battle.Card
	if p.Team != nil {
		if err := jsonutil.Unmarshal(*p.Team, &team); err != nil {
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
	cardsJSON, err := jsonutil.Marshal(cards)
	if err != nil {
		return fmt.Errorf("failed to marshal cards: %w", err)
	}
	teamJSON, err := jsonutil.Marshal(team)
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

// GetMatchHistory retrieves a user's match history
func (s *ArenaService) GetMatchHistory(ctx context.Context, chatID, userID int64, limit, offset int) ([]*repository.MatchHistoryEntry, int, error) {
	defer nrutil.StartSegment(ctx, "service:arena:get-matchHistory")()

	return s.gameRepo.GetMatchHistory(ctx, chatID, userID, limit, offset)
}

// GetH2HRecord retrieves head-to-head record against a specific opponent
func (s *ArenaService) GetH2HRecord(ctx context.Context, chatID, userID, opponentID int64) (*repository.H2HRecord, error) {
	defer nrutil.StartSegment(ctx, "service:arena:get-h2h-record")()

	return s.gameRepo.GetH2HRecord(ctx, chatID, userID, opponentID)
}

// GetRecentMatchesVsOpponent retrieves recent matches against a specific opponent
func (s *ArenaService) GetRecentMatchesVsOpponent(ctx context.Context, chatID, userID, opponentID int64, limit int) ([]*repository.MatchHistoryEntry, error) {
	defer nrutil.StartSegment(ctx, "service:arena:get-recent-matches-vs-opponent")()

	return s.gameRepo.GetRecentMatchesVsOpponent(ctx, chatID, userID, opponentID, limit)
}

// ArenaProfileResponse represents a user's arena profile
type ArenaProfileResponse struct {
	*repository.UserProfile
	RecentMatches []*repository.MatchHistoryEntry `json:"recent_matches"`
}

// GetProfile retrieves a user's profile with stats, ranks, and recent matches
func (s *ArenaService) GetProfile(ctx context.Context, chatID, userID int64) (*ArenaProfileResponse, error) {
	defer nrutil.StartSegment(ctx, "service:arena:get-profile")()

	// Get user profile with rank positions
	profile, err := s.gameRepo.GetUserProfile(ctx, chatID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user profile: %w", err)
	}
	if profile == nil {
		return nil, nil
	}

	// Generate presigned URL for profile photo
	if profile.PhotoObjectKey != nil && *profile.PhotoObjectKey != "" {
		if url, err := s.storageClient.GetPresignedURL(ctx, *profile.PhotoObjectKey, time.Hour); err == nil && url != "" {
			profile.PhotoURL = &url
		}
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

// populateParticipantPhotoURLs generates presigned URLs for participant photos
func (s *ArenaService) populateParticipantPhotoURLs(ctx context.Context, participants []*repository.ParticipantWithUser) {
	for _, p := range participants {
		if p.PhotoObjectKey != nil && *p.PhotoObjectKey != "" {
			if url, err := s.storageClient.GetPresignedURL(ctx, *p.PhotoObjectKey, time.Hour); err == nil && url != "" {
				p.PhotoURL = &url
			}
		}
	}
}

// populateLeaderboardPhotoURLs generates presigned URLs for leaderboard entry photos
func (s *ArenaService) populateLeaderboardPhotoURLs(ctx context.Context, entries []*repository.LeaderboardEntry) {
	for _, e := range entries {
		if e.PhotoObjectKey != nil && *e.PhotoObjectKey != "" {
			if url, err := s.storageClient.GetPresignedURL(ctx, *e.PhotoObjectKey, time.Hour); err == nil && url != "" {
				e.PhotoURL = &url
			}
		}
	}
}
