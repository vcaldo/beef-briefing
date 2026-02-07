package services

import (
	"context"
)

// Battle delegation methods - ArenaService delegates to BattleService

// checkAndStartBattle checks if all participants are ready and starts battle.
func (s *ArenaService) checkAndStartBattle(ctx context.Context, matchID string) {
	s.battleService.CheckAndStartBattle(ctx, matchID)
}

// StartBattle initiates the battle phase.
func (s *ArenaService) StartBattle(ctx context.Context, matchID string) (*BattleResponse, error) {
	return s.battleService.StartBattle(ctx, matchID)
}

// GetBattle retrieves battle results for a participant.
func (s *ArenaService) GetBattle(ctx context.Context, matchID string, userID int64) (*BattleResponse, error) {
	return s.battleService.GetBattle(ctx, matchID, userID)
}

// GetBattleReplay retrieves battle results for any user in the same chat (no participant check).
func (s *ArenaService) GetBattleReplay(ctx context.Context, matchID string, userID int64) (*BattleResponse, error) {
	return s.battleService.GetBattleReplay(ctx, matchID, userID)
}

// GetRoundBattle retrieves battle results for a specific round within a multi-round match.
func (s *ArenaService) GetRoundBattle(ctx context.Context, matchID string, roundNumber int, userID int64) (*BattleResponse, error) {
	return s.battleService.GetRoundBattle(ctx, matchID, roundNumber, userID)
}
