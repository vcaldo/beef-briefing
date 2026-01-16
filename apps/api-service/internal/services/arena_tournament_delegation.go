package services

import (
	"context"
	"time"

	"beef-briefing/apps/api-service/internal/repository"
)

// =============================================================================
// Tournament delegation methods - delegate to TournamentService
// =============================================================================

// GetOrCreateTodayTournament delegates to TournamentService
func (s *ArenaService) GetOrCreateTodayTournament(ctx context.Context, chatID int64, date string) (*TournamentResponse, error) {
	return s.tournamentService.GetOrCreateTodayTournament(ctx, chatID, date)
}

// GetTodayTournament delegates to TournamentService
func (s *ArenaService) GetTodayTournament(ctx context.Context, chatID int64, date string) (*TournamentResponse, error) {
	return s.tournamentService.GetTodayTournament(ctx, chatID, date)
}

// GetTournamentByID delegates to TournamentService
func (s *ArenaService) GetTournamentByID(ctx context.Context, tournamentID int64) (*TournamentResponse, error) {
	return s.tournamentService.GetTournamentByID(ctx, tournamentID)
}

// SetTournamentAnnounced delegates to TournamentService
func (s *ArenaService) SetTournamentAnnounced(ctx context.Context, tournamentID int64, messageID int64) error {
	return s.tournamentService.SetTournamentAnnounced(ctx, tournamentID, messageID)
}

// JoinTournament delegates to TournamentService
func (s *ArenaService) JoinTournament(ctx context.Context, tournamentID int64, userID int64) (*TournamentResponse, error) {
	return s.tournamentService.JoinTournament(ctx, tournamentID, userID)
}

// LeaveTournament delegates to TournamentService
func (s *ArenaService) LeaveTournament(ctx context.Context, tournamentID int64, userID int64) (*TournamentResponse, error) {
	return s.tournamentService.LeaveTournament(ctx, tournamentID, userID)
}

// GetTournamentsNeedingAnnouncement delegates to TournamentService
func (s *ArenaService) GetTournamentsNeedingAnnouncement(ctx context.Context, currentTime time.Time) ([]*repository.TournamentInfo, error) {
	return s.tournamentService.GetTournamentsNeedingAnnouncement(ctx, currentTime)
}

// GetTournamentsNeedingClose delegates to TournamentService
func (s *ArenaService) GetTournamentsNeedingClose(ctx context.Context, currentTime time.Time) ([]*repository.TournamentInfo, error) {
	return s.tournamentService.GetTournamentsNeedingClose(ctx, currentTime)
}

// CloseAndStartTournament delegates to TournamentService
func (s *ArenaService) CloseAndStartTournament(ctx context.Context, tournamentID int64) (*TournamentStartResult, error) {
	return s.tournamentService.CloseAndStartTournament(ctx, tournamentID)
}

// GetTournamentsWithPendingRounds delegates to TournamentService
func (s *ArenaService) GetTournamentsWithPendingRounds(ctx context.Context) ([]*repository.RankedTournament, error) {
	return s.tournamentService.GetTournamentsWithPendingRounds(ctx)
}

// CompleteTournament delegates to TournamentService
func (s *ArenaService) CompleteTournament(ctx context.Context, tournamentID int64, winnerUserID *int64) error {
	return s.tournamentService.CompleteTournament(ctx, tournamentID, winnerUserID)
}

// GetChatsWithTimezone delegates to TournamentService
func (s *ArenaService) GetChatsWithTimezone(ctx context.Context) ([]*repository.ChatTimezone, error) {
	return s.tournamentService.GetChatsWithTimezone(ctx)
}
