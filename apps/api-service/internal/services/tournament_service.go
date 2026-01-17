package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"beef-briefing/apps/api-service/internal/apperror"
	"beef-briefing/apps/api-service/internal/game/shop"
	"beef-briefing/apps/api-service/internal/jsonutil"
	"beef-briefing/apps/api-service/internal/nrutil"
	"beef-briefing/apps/api-service/internal/repository"

	"github.com/newrelic/go-agent/v3/newrelic"
)

// TournamentService handles ranked tournament logic
type TournamentService struct {
	db       *sql.DB
	gameRepo repository.GameRepositoryInterface
	dealer   shop.DealerInterface
	nrApp    *newrelic.Application
}

// NewTournamentService creates a new TournamentService
func NewTournamentService(
	db *sql.DB,
	gameRepo repository.GameRepositoryInterface,
	dealer shop.DealerInterface,
	nrApp *newrelic.Application,
) *TournamentService {
	return &TournamentService{
		db:       db,
		gameRepo: gameRepo,
		dealer:   dealer,
		nrApp:    nrApp,
	}
}

// recordTournamentEvent records a custom event for tournament lifecycle events.
func (s *TournamentService) recordTournamentEvent(eventType string, tournamentID int64, chatID int64, participantCount int) {
	if s.nrApp == nil {
		return
	}

	params := map[string]interface{}{
		"event_type":        eventType,
		"tournament_id":     tournamentID,
		"chat_id":           chatID,
		"participant_count": participantCount,
	}

	s.nrApp.RecordCustomEvent("arena.tournament.event", params)
}

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
func (s *TournamentService) GetOrCreateTodayTournament(ctx context.Context, chatID int64, date string) (*TournamentResponse, error) {
	defer nrutil.StartSegment(ctx, "service:tournament:get-or-create-today-tournament")()

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
func (s *TournamentService) GetTodayTournament(ctx context.Context, chatID int64, date string) (*TournamentResponse, error) {
	defer nrutil.StartSegment(ctx, "service:tournament:get-today-tournament")()

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
func (s *TournamentService) GetTournamentByID(ctx context.Context, tournamentID int64) (*TournamentResponse, error) {
	defer nrutil.StartSegment(ctx, "service:tournament:get-tournament-by-id")()

	tournament, err := s.gameRepo.GetTournamentByID(ctx, tournamentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tournament: %w", err)
	}
	if tournament == nil {
		return nil, apperror.ErrTournamentNotFound
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
func (s *TournamentService) SetTournamentAnnounced(ctx context.Context, tournamentID int64, messageID int64) error {
	return s.gameRepo.SetTournamentAnnounced(ctx, tournamentID, messageID)
}

// JoinTournament adds a user to a tournament
func (s *TournamentService) JoinTournament(ctx context.Context, tournamentID int64, userID int64) (*TournamentResponse, error) {
	defer nrutil.StartSegment(ctx, "service:tournament:join-tournament")()

	// Get tournament
	tournament, err := s.gameRepo.GetTournamentByID(ctx, tournamentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tournament: %w", err)
	}
	if tournament == nil {
		return nil, apperror.ErrTournamentNotFound
	}

	// Check tournament is open
	if tournament.Status != repository.TournamentStatusOpen {
		if tournament.Status == repository.TournamentStatusScheduled {
			return nil, apperror.ErrTournamentNotOpen
		}
		return nil, apperror.ErrTournamentRegistrationClosed
	}

	// Check if already registered
	isParticipant, err := s.gameRepo.IsTournamentParticipant(ctx, tournamentID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check participant: %w", err)
	}
	if isParticipant {
		return nil, apperror.ErrAlreadyRegistered
	}

	// Add participant
	if err := s.gameRepo.AddTournamentParticipant(ctx, tournamentID, userID); err != nil {
		return nil, fmt.Errorf("failed to add participant: %w", err)
	}

	// Return updated tournament
	return s.GetTournamentByID(ctx, tournamentID)
}

// LeaveTournament removes a user from a tournament
func (s *TournamentService) LeaveTournament(ctx context.Context, tournamentID int64, userID int64) (*TournamentResponse, error) {
	defer nrutil.StartSegment(ctx, "service:tournament:leave-tournament")()

	// Get tournament
	tournament, err := s.gameRepo.GetTournamentByID(ctx, tournamentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tournament: %w", err)
	}
	if tournament == nil {
		return nil, apperror.ErrTournamentNotFound
	}

	// Check tournament is still open
	if tournament.Status != repository.TournamentStatusOpen {
		return nil, apperror.ErrTournamentRegistrationClosed
	}

	// Check if registered
	isParticipant, err := s.gameRepo.IsTournamentParticipant(ctx, tournamentID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check participant: %w", err)
	}
	if !isParticipant {
		return nil, apperror.ErrNotRegistered
	}

	// Remove participant
	if err := s.gameRepo.RemoveTournamentParticipant(ctx, tournamentID, userID); err != nil {
		return nil, fmt.Errorf("failed to remove participant: %w", err)
	}

	// Return updated tournament
	return s.GetTournamentByID(ctx, tournamentID)
}

// GetTournamentsNeedingAnnouncement returns tournaments that need to be announced
func (s *TournamentService) GetTournamentsNeedingAnnouncement(ctx context.Context, currentTime time.Time) ([]*repository.TournamentInfo, error) {
	return s.gameRepo.GetTournamentsNeedingAnnouncement(ctx, currentTime)
}

// GetTournamentsNeedingClose returns tournaments that need registration closed
func (s *TournamentService) GetTournamentsNeedingClose(ctx context.Context, currentTime time.Time) ([]*repository.TournamentInfo, error) {
	return s.gameRepo.GetTournamentsNeedingClose(ctx, currentTime)
}

// CloseAndStartTournament closes registration and starts the tournament match
func (s *TournamentService) CloseAndStartTournament(ctx context.Context, tournamentID int64) (*TournamentStartResult, error) {
	defer nrutil.StartSegment(ctx, "service:tournament:close-and-start-tournament")()

	// Get tournament
	tournament, err := s.gameRepo.GetTournamentByID(ctx, tournamentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tournament: %w", err)
	}
	if tournament == nil {
		return nil, apperror.ErrTournamentNotFound
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
	s.recordTournamentEvent("started", tournamentID, tournament.ChatID, len(participants))

	return &TournamentStartResult{
		Tournament:       tournament,
		Match:            match,
		ParticipantCount: len(participants),
		Format:           format,
		Skipped:          false,
	}, nil
}

// dealCardsToAllParticipants deals shop cards to all match participants
func (s *TournamentService) dealCardsToAllParticipants(ctx context.Context, matchID string, chatID int64) error {
	defer nrutil.StartSegment(ctx, "service:tournament:deal-cards-to-all-participants")()

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
		cardsJSON, err := jsonutil.Marshal(cards)
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
func (s *TournamentService) GetTournamentsWithPendingRounds(ctx context.Context) ([]*repository.RankedTournament, error) {
	return s.gameRepo.GetTournamentsWithPendingRounds(ctx)
}

// CompleteTournament marks a tournament as completed
func (s *TournamentService) CompleteTournament(ctx context.Context, tournamentID int64, winnerUserID *int64) error {
	return s.gameRepo.CompleteTournament(ctx, tournamentID, winnerUserID)
}

// GetChatsWithTimezone returns all chats with their timezone settings
func (s *TournamentService) GetChatsWithTimezone(ctx context.Context) ([]*repository.ChatTimezone, error) {
	return s.gameRepo.GetChatsWithTimezone(ctx)
}
