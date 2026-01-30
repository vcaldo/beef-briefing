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

// MatchService handles match lifecycle operations (creation, joining, starting).
// This is a sub-service of ArenaService focused specifically on match management.
type MatchService struct {
	db       *sql.DB
	gameRepo repository.GameRepositoryInterface
	dealer   shop.DealerInterface
	nrApp    *newrelic.Application
}

// NewMatchService creates a new MatchService instance.
func NewMatchService(
	db *sql.DB,
	gameRepo repository.GameRepositoryInterface,
	dealer shop.DealerInterface,
	nrApp *newrelic.Application,
) *MatchService {
	return &MatchService{
		db:       db,
		gameRepo: gameRepo,
		dealer:   dealer,
		nrApp:    nrApp,
	}
}

// CreateMatch creates a new regular match
func (s *MatchService) CreateMatch(ctx context.Context, chatID int64, creatorUserID int64) (*MatchResponse, error) {
	defer nrutil.StartSegment(ctx, "service:match:create-match")()

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

	// Record match creation metric
	recordMatchEvent(s.nrApp, "created", match.ID, chatID, creatorUserID, string(repository.MatchTypeRegular))

	return &MatchResponse{
		Match:        match,
		Participants: participants,
		CardCount:    cardCount,
	}, nil
}

// GetMatch retrieves a match with participants
func (s *MatchService) GetMatch(ctx context.Context, matchID string) (*MatchResponse, error) {
	defer nrutil.StartSegment(ctx, "service:match:get-match")()

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

	cardCount, _ := s.dealer.GetCardCount(ctx, match.ChatID)

	return &MatchResponse{
		Match:        match,
		Participants: participants,
		CardCount:    cardCount,
	}, nil
}

// GetActiveMatches retrieves active matches for a chat
func (s *MatchService) GetActiveMatches(ctx context.Context, chatID int64) ([]*MatchResponse, error) {
	defer nrutil.StartSegment(ctx, "service:match:get-active-matches")()

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
func (s *MatchService) JoinMatch(ctx context.Context, matchID string, userID int64) (*MatchResponse, error) {
	defer nrutil.StartSegment(ctx, "service:match:join-match")()

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
func (s *MatchService) LeaveMatch(ctx context.Context, matchID string, userID int64) error {
	defer nrutil.StartSegment(ctx, "service:match:leave-match")()

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
func (s *MatchService) StartMatch(ctx context.Context, matchID string, userID int64) (*MatchResponse, error) {
	defer nrutil.StartSegment(ctx, "service:match:start-match")()

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
func (s *MatchService) dealCardsToParticipants(ctx context.Context, matchID string, chatID int64) error {
	defer nrutil.StartSegment(ctx, "service:match:deal-cards-to-participants")()

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

// GetPendingMatches returns matches with expired deadlines that need action
func (s *MatchService) GetPendingMatches(ctx context.Context) ([]*PendingMatch, error) {
	defer nrutil.StartSegment(ctx, "service:match:get-pending-matches")()

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

// AutoStartMatch handles expired join deadline - starts match if 2+ participants, otherwise cancels
func (s *MatchService) AutoStartMatch(ctx context.Context, matchID string) (*AutoStartResult, error) {
	defer nrutil.StartSegment(ctx, "service:match:auto-start-match")()

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

// GetMatchHistory retrieves a user's match history
func (s *MatchService) GetMatchHistory(ctx context.Context, chatID, userID int64, limit, offset int) ([]*repository.MatchHistoryEntry, int, error) {
	defer nrutil.StartSegment(ctx, "service:match:get-match-history")()

	return s.gameRepo.GetMatchHistory(ctx, chatID, userID, limit, offset)
}

// GetH2HRecord retrieves head-to-head record against a specific opponent
func (s *MatchService) GetH2HRecord(ctx context.Context, chatID, userID, opponentID int64) (*repository.H2HRecord, error) {
	defer nrutil.StartSegment(ctx, "service:match:get-h2h-record")()

	return s.gameRepo.GetH2HRecord(ctx, chatID, userID, opponentID)
}

// GetRecentMatchesVsOpponent retrieves recent matches against a specific opponent
func (s *MatchService) GetRecentMatchesVsOpponent(ctx context.Context, chatID, userID, opponentID int64, limit int) ([]*repository.MatchHistoryEntry, error) {
	defer nrutil.StartSegment(ctx, "service:match:get-recent-matches-vs-opponent")()

	return s.gameRepo.GetRecentMatchesVsOpponent(ctx, chatID, userID, opponentID, limit)
}

// GetChatOpenMatch retrieves an open regular match for a chat
func (s *MatchService) GetChatOpenMatch(ctx context.Context, chatID int64) (*repository.Match, error) {
	defer nrutil.StartSegment(ctx, "service:match:get-chat-open-match")()

	return s.gameRepo.GetChatOpenMatch(ctx, chatID)
}

// SetTelegramMessageID updates the telegram message ID for a match
func (s *MatchService) SetTelegramMessageID(ctx context.Context, matchID string, messageID int64) error {
	defer nrutil.StartSegment(ctx, "service:match:set-telegram-message-id")()

	return s.gameRepo.SetTelegramMessageID(ctx, matchID, messageID)
}
