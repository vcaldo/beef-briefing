package services

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"sync"

	"beef-briefing/apps/api-service/internal/apperror"
	"beef-briefing/apps/api-service/internal/client"
	"beef-briefing/apps/api-service/internal/game/battle"
	"beef-briefing/apps/api-service/internal/jsonutil"
	"beef-briefing/apps/api-service/internal/nrutil"
	"beef-briefing/apps/api-service/internal/repository"

	"github.com/newrelic/go-agent/v3/newrelic"
)

// BattleService handles battle execution and resolution.
// Extracted from ArenaService to improve maintainability and reduce file size.
type BattleService struct {
	db           *sql.DB
	gameRepo     repository.GameRepositoryInterface
	nrApp        *newrelic.Application
	matchMutexes sync.Map // map[string]*sync.Mutex for per-match locking
	botClient    BotClientInterface
}

// BotClientInterface defines the interface for bot webhook notifications.
type BotClientInterface interface {
	NotifyParticipantChange(ctx context.Context, matchID string, chatID, telegramMessageID int64) error
	NotifyBattleComplete(ctx context.Context, notification *client.BattleResultNotification) error
}

// NewBattleService creates a new BattleService instance.
func NewBattleService(
	db *sql.DB,
	gameRepo repository.GameRepositoryInterface,
	nrApp *newrelic.Application,
	botClient BotClientInterface,
) *BattleService {
	return &BattleService{
		db:        db,
		gameRepo:  gameRepo,
		nrApp:     nrApp,
		botClient: botClient,
	}
}

// recordBattleCompletion records a custom event for battle completion.
// This tracks key metrics for business analytics and monitoring.
func recordBattleCompletion(nrApp *newrelic.Application, matchID string, format string, winnerID *int64, isDraw bool, numRounds int, teamADamage, teamBDamage int) {
	if nrApp == nil {
		return
	}

	params := map[string]interface{}{
		"match_id":      matchID,
		"format":        format, // "1v1" or "arena"
		"is_draw":       isDraw,
		"num_rounds":    numRounds,
		"team_a_damage": teamADamage,
		"team_b_damage": teamBDamage,
	}

	if winnerID != nil {
		params["winner_id"] = *winnerID
	}

	nrApp.RecordCustomEvent("arena.battle.completed", params)
}

// CheckAndStartBattle checks if all participants are ready and starts battle.
// Uses per-match mutex to prevent race conditions when multiple participants submit simultaneously.
//
// Note on sync.Map cleanup: We intentionally do NOT delete map entries after use.
// Deleting while other goroutines may be waiting on LoadOrStore creates a race condition
// where a new mutex could be created for the same match, allowing concurrent execution.
// Since match IDs are unique UUIDs and battles only happen once per match, letting entries
// persist is safe. For long-running services, periodic cleanup could be added if memory
// becomes a concern, but the overhead is negligible (one mutex per completed match).
func (s *BattleService) CheckAndStartBattle(ctx context.Context, matchID string) {
	// Get or create mutex for this match to prevent race condition
	mutexInterface, _ := s.matchMutexes.LoadOrStore(matchID, &sync.Mutex{})
	mutex := mutexInterface.(*sync.Mutex)

	mutex.Lock()
	defer mutex.Unlock()

	total, _ := s.gameRepo.GetParticipantCount(ctx, matchID)
	ready, _ := s.gameRepo.GetReadyParticipantCount(ctx, matchID)

	if ready >= total && total >= 2 {
		s.StartBattle(ctx, matchID)
	}
}

// StartBattle initiates the battle phase.
// Transitions match to battle phase and executes battle simulation.
func (s *BattleService) StartBattle(ctx context.Context, matchID string) (*BattleResponse, error) {
	defer nrutil.StartSegment(ctx, "service:battle:start-battle")()

	match, err := s.gameRepo.GetMatch(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get match: %w", err)
	}
	if match == nil {
		return nil, apperror.ErrMatchNotFound
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

	// For 3+ participants, dispatch by match format
	if match.Format != nil && *match.Format == repository.MatchFormatBracket {
		return s.runBracket(ctx, matchID, participants)
	}
	return s.runFreeForAll(ctx, matchID, participants)
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

// fightResult holds the outcome of a single fight simulation between two participants.
// It contains the battle result plus metadata needed by callers (player names and IDs).
type fightResult struct {
	result     *battle.Result
	ownerNameA string
	ownerNameB string
	playerAID  int64
	playerBID  int64
}

// runSingleFight executes a single fight between two participants: team parsing,
// team order normalization, battle simulation, and round persistence.
// It does NOT update the leaderboard, complete the match, or send bot notifications.
func (s *BattleService) runSingleFight(ctx context.Context, matchID string, pA, pB *repository.ParticipantWithUser, roundNumber int) (*fightResult, error) {
	defer nrutil.StartSegment(ctx, "service:battle:run-single-fight")()

	// Parse teams
	var teamACards, teamBCards []*battle.Card
	if err := jsonutil.Unmarshal(*pA.Team, &teamACards); err != nil {
		return nil, fmt.Errorf("failed to parse team A: %w", err)
	}
	if err := jsonutil.Unmarshal(*pB.Team, &teamBCards); err != nil {
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

	// Save round (use teamA.Cards/teamB.Cards to preserve correct Position values set by NewTeam)
	teamAJSON, err := jsonutil.Marshal(teamA.Cards)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal team A: %w", err)
	}
	teamBJSON, err := jsonutil.Marshal(teamB.Cards)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal team B: %w", err)
	}
	eventsJSON, err := jsonutil.Marshal(result.Events)
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

	return &fightResult{
		result:     result,
		ownerNameA: ownerNameA,
		ownerNameB: ownerNameB,
		playerAID:  pA.UserID,
		playerBID:  pB.UserID,
	}, nil
}

// runBattle executes a single 1v1 battle between two participants and records the results.
// It calls runSingleFight for the core simulation, then handles leaderboard updates,
// match completion, and bot notifications. Returns a BattleResponse with the outcome.
func (s *BattleService) runBattle(ctx context.Context, matchID string, pA, pB *repository.ParticipantWithUser, roundNumber int) (*BattleResponse, error) {
	defer nrutil.StartSegment(ctx, "service:battle:run-battle")()

	fight, err := s.runSingleFight(ctx, matchID, pA, pB, roundNumber)
	if err != nil {
		return nil, err
	}
	result := fight.result

	// Record battle completion metric
	recordBattleCompletion(s.nrApp, matchID, "1v1", result.WinnerID, result.IsDraw, result.NumRounds, result.TeamADamage, result.TeamBDamage)

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

	// Notify bot to update message with winner
	if s.botClient != nil && match.TelegramMessageID != nil && *match.TelegramMessageID != 0 {
		go s.botClient.NotifyParticipantChange(context.Background(), matchID, match.ChatID, *match.TelegramMessageID)
	}

	// Notify players via DM with battle results
	if s.botClient != nil {
		go s.botClient.NotifyBattleComplete(context.Background(), &client.BattleResultNotification{
			MatchID:     matchID,
			ChatID:      match.ChatID,
			MatchType:   string(match.MatchType),
			Format:      "1v1",
			PlayerAID:   pA.UserID,
			PlayerBID:   pB.UserID,
			PlayerAName: fight.ownerNameA,
			PlayerBName: fight.ownerNameB,
			WinnerID:    result.WinnerID,
			IsDraw:      result.IsDraw,
			TeamADamage: result.TeamADamage,
			TeamBDamage: result.TeamBDamage,
			NumRounds:   result.NumRounds,
		})
	}

	// Group events into combats
	combats := battle.GroupEventsIntoCombats(result.Events, pA.UserID, pB.UserID)

	return &BattleResponse{
		MatchID:     matchID,
		WinnerID:    result.WinnerID,
		IsDraw:      result.IsDraw,
		Combats:     combats,
		Events:      result.Events,
		NumCombats:  len(combats),
		NumRounds:   result.NumRounds,
		TeamADamage: result.TeamADamage,
		TeamBDamage: result.TeamBDamage,
		TeamAFinal:  result.TeamAFinal,
		TeamBFinal:  result.TeamBFinal,
		PlayerAID:   pA.UserID,
		PlayerBID:   pB.UserID,
		PlayerAName: fight.ownerNameA,
		PlayerBName: fight.ownerNameB,
		Format:      "1v1",
	}, nil
}

// playerStats tracks in-memory stats for each participant during a free-for-all tournament.
// This avoids reading from DB stats that may not be initialized yet (Bug 2 fix).
type playerStats struct {
	wins             int
	losses           int
	draws            int
	totalDamageDealt int
}

// runFreeForAll executes free-for-all format with round-robin tournament.
// Each participant faces every other participant. Winner is determined by:
// 1. Most wins across all battles
// 2. Total damage dealt (as tiebreaker if wins are equal)
// 3. Lower user_id (deterministic tiebreaker if damage is tied)
//
// Fixes over the previous runArena implementation:
// - Bug 1 fix: Uses runSingleFight instead of runBattle, so CompleteMatch and
//   leaderboard updates are not called prematurely after each round.
// - Bug 2 fix: Tracks wins/losses/damage in-memory instead of reading from
//   uninitialized DB stats.
func (s *BattleService) runFreeForAll(ctx context.Context, matchID string, participants []*repository.ParticipantWithUser) (*BattleResponse, error) {
	defer nrutil.StartSegment(ctx, "service:battle:run-free-for-all")()

	// Build name lookup and in-memory stats tracking
	nameMap := make(map[int64]string)
	stats := make(map[int64]*playerStats)
	for _, p := range participants {
		name := p.FirstName
		if name == "" {
			name = p.Username
		}
		nameMap[p.UserID] = name
		stats[p.UserID] = &playerStats{}
	}

	roundNumber := 0
	var ffaRounds []FfaRoundSummary

	// Round-robin: each player fights each other player using runSingleFight (Bug 1 fix)
	for i := 0; i < len(participants); i++ {
		for j := i + 1; j < len(participants); j++ {
			roundNumber++
			fight, err := s.runSingleFight(ctx, matchID, participants[i], participants[j], roundNumber)
			if err != nil {
				return nil, err
			}
			result := fight.result

			// Track stats in-memory (Bug 2 fix)
			if result.WinnerID != nil {
				if *result.WinnerID == fight.playerAID {
					stats[fight.playerAID].wins++
					stats[fight.playerBID].losses++
				} else {
					stats[fight.playerBID].wins++
					stats[fight.playerAID].losses++
				}
			} else if result.IsDraw {
				stats[fight.playerAID].draws++
				stats[fight.playerBID].draws++
			}
			stats[fight.playerAID].totalDamageDealt += result.TeamADamage
			stats[fight.playerBID].totalDamageDealt += result.TeamBDamage

			// Build round summary for response
			ffaRounds = append(ffaRounds, FfaRoundSummary{
				RoundNumber: roundNumber,
				PlayerAID:   fight.playerAID,
				PlayerAName: fight.ownerNameA,
				PlayerBID:   fight.playerBID,
				PlayerBName: fight.ownerNameB,
				WinnerID:    result.WinnerID,
				IsDraw:      result.IsDraw,
				PlayerADmg:  result.TeamADamage,
				PlayerBDmg:  result.TeamBDamage,
				NumRounds:   result.NumRounds,
			})
		}
	}

	// Determine winner from in-memory stats (same priority: wins > damage > lower user_id)
	type rankedPlayer struct {
		userID           int64
		wins             int
		losses           int
		draws            int
		totalDamageDealt int
	}

	ranked := make([]rankedPlayer, 0, len(participants))
	for _, p := range participants {
		s := stats[p.UserID]
		ranked = append(ranked, rankedPlayer{
			userID:           p.UserID,
			wins:             s.wins,
			losses:           s.losses,
			draws:            s.draws,
			totalDamageDealt: s.totalDamageDealt,
		})
	}

	// Sort by wins desc, then damage desc, then user_id asc
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].wins != ranked[j].wins {
			return ranked[i].wins > ranked[j].wins
		}
		if ranked[i].totalDamageDealt != ranked[j].totalDamageDealt {
			return ranked[i].totalDamageDealt > ranked[j].totalDamageDealt
		}
		return ranked[i].userID < ranked[j].userID
	})

	// Build standings
	standings := make([]ArenaStanding, len(ranked))
	for i, r := range ranked {
		photoURL := ""
		for _, p := range participants {
			if p.UserID == r.userID && p.PhotoURL != nil {
				photoURL = *p.PhotoURL
				break
			}
		}
		standings[i] = ArenaStanding{
			UserID:           r.userID,
			Name:             nameMap[r.userID],
			PhotoURL:         photoURL,
			Rank:             i + 1,
			Wins:             r.wins,
			Losses:           r.losses,
			Draws:            r.draws,
			TotalDamageDealt: r.totalDamageDealt,
		}
	}

	// Champion is the first in ranked order
	var winnerID *int64
	if len(ranked) > 0 {
		winnerID = &ranked[0].userID
	}

	// Update leaderboard ONCE per participant: champion=win, all others=loss
	match, err := s.gameRepo.GetMatch(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get match for leaderboard update: %w", err)
	}
	if match == nil {
		return nil, fmt.Errorf("match not found for leaderboard update: %s", matchID)
	}

	for _, p := range participants {
		isWin := winnerID != nil && p.UserID == *winnerID
		s.gameRepo.UpdateLeaderboard(ctx, p.UserID, match.ChatID, match.MatchType, isWin, nil, false, false)
	}

	// Complete match ONCE with champion
	s.gameRepo.CompleteMatch(ctx, matchID, winnerID)

	// Notify bot to update message with winner
	if s.botClient != nil {
		if match.TelegramMessageID != nil && *match.TelegramMessageID != 0 {
			go s.botClient.NotifyParticipantChange(context.Background(), matchID, match.ChatID, *match.TelegramMessageID)
		}

		// Notify all participants via DM with tournament results
		participantResults := make([]client.BattleParticipantResult, len(ranked))
		for i, r := range ranked {
			participantResults[i] = client.BattleParticipantResult{
				UserID:      r.userID,
				Name:        nameMap[r.userID],
				Wins:        r.wins,
				TotalDamage: r.totalDamageDealt,
				Rank:        i + 1,
			}
		}
		go s.botClient.NotifyBattleComplete(context.Background(), &client.BattleResultNotification{
			MatchID:      matchID,
			ChatID:       match.ChatID,
			MatchType:    string(match.MatchType),
			Format:       "free_for_all",
			WinnerID:     winnerID,
			NumRounds:    roundNumber,
			Participants: participantResults,
		})
	}

	// Record free-for-all completion metric
	recordBattleCompletion(s.nrApp, matchID, "free_for_all", winnerID, false, roundNumber, 0, 0)

	// Free-for-all format returns summary response with standings and round summaries
	return &BattleResponse{
		MatchID:    matchID,
		WinnerID:   winnerID,
		IsDraw:     false,
		Combats:    []battle.Combat{},
		Events:     []battle.BattleEvent{},
		NumCombats: 0,
		NumRounds:  roundNumber,
		Format:     "free_for_all",
		Standings:  standings,
		FfaRounds:  ffaRounds,
	}, nil
}

// bracketRoundName returns the display name for a bracket round based on the number of matches.
func bracketRoundName(numMatches int) string {
	switch numMatches {
	case 1:
		return "Final"
	case 2:
		return "Semifinals"
	case 4:
		return "Quarterfinals"
	default:
		return fmt.Sprintf("Round of %d", numMatches*2)
	}
}

// runBracket executes bracket tournament format for even player counts.
// Participants are paired sequentially: [P1,P2], [P3,P4], ...
// Winners advance to the next round until one champion remains.
//
// Round naming: 1 match = "Final", 2 = "Semifinals", 4 = "Quarterfinals", etc.
// Leaderboard is updated ONCE per participant (champion=win, others=loss).
// CompleteMatch is called ONCE with the champion.
func (s *BattleService) runBracket(ctx context.Context, matchID string, participants []*repository.ParticipantWithUser) (*BattleResponse, error) {
	defer nrutil.StartSegment(ctx, "service:battle:run-bracket")()

	// Build name and photo lookup maps
	nameMap := make(map[int64]string)
	photoMap := make(map[int64]string)
	for _, p := range participants {
		name := p.FirstName
		if name == "" {
			name = p.Username
		}
		nameMap[p.UserID] = name
		if p.PhotoURL != nil {
			photoMap[p.UserID] = *p.PhotoURL
		}
	}

	// Build participant lookup by UserID for winner advancement
	participantMap := make(map[int64]*repository.ParticipantWithUser)
	for _, p := range participants {
		participantMap[p.UserID] = p
	}

	// Track stats in-memory for standings
	stats := make(map[int64]*playerStats)
	for _, p := range participants {
		stats[p.UserID] = &playerStats{}
	}

	var bracketRounds []BracketRound
	roundNumber := 0         // sequential round number for DB persistence
	currentRound := participants // players in current bracket round

	for len(currentRound) > 1 {
		numMatches := len(currentRound) / 2
		roundName := bracketRoundName(numMatches)

		var matches []BracketMatch
		var winners []*repository.ParticipantWithUser

		for i := 0; i < len(currentRound); i += 2 {
			roundNumber++
			pA := currentRound[i]
			pB := currentRound[i+1]

			fight, err := s.runSingleFight(ctx, matchID, pA, pB, roundNumber)
			if err != nil {
				return nil, err
			}
			result := fight.result

			// Track stats
			if result.WinnerID != nil {
				if *result.WinnerID == fight.playerAID {
					stats[fight.playerAID].wins++
					stats[fight.playerBID].losses++
					winners = append(winners, pA)
				} else {
					stats[fight.playerBID].wins++
					stats[fight.playerAID].losses++
					winners = append(winners, pB)
				}
			} else {
				// Draw: advance player A by default (lower index = higher seed)
				stats[fight.playerAID].draws++
				stats[fight.playerBID].draws++
				winners = append(winners, pA)
			}
			stats[fight.playerAID].totalDamageDealt += result.TeamADamage
			stats[fight.playerBID].totalDamageDealt += result.TeamBDamage

			matches = append(matches, BracketMatch{
				MatchNumber: roundNumber,
				PlayerAID:   fight.playerAID,
				PlayerAName: fight.ownerNameA,
				PlayerBID:   fight.playerBID,
				PlayerBName: fight.ownerNameB,
				WinnerID:    result.WinnerID,
				IsDraw:      result.IsDraw,
				PlayerADmg:  result.TeamADamage,
				PlayerBDmg:  result.TeamBDamage,
				NumRounds:   result.NumRounds,
			})
		}

		bracketRounds = append(bracketRounds, BracketRound{
			Name:    roundName,
			Matches: matches,
		})

		currentRound = winners
	}

	// Champion is the last remaining player
	var winnerID *int64
	if len(currentRound) == 1 {
		winnerID = &currentRound[0].UserID
	}

	// Build standings sorted by: wins desc, damage desc, user_id asc
	type rankedPlayer struct {
		userID           int64
		wins             int
		losses           int
		draws            int
		totalDamageDealt int
	}

	ranked := make([]rankedPlayer, 0, len(participants))
	for _, p := range participants {
		s := stats[p.UserID]
		ranked = append(ranked, rankedPlayer{
			userID:           p.UserID,
			wins:             s.wins,
			losses:           s.losses,
			draws:            s.draws,
			totalDamageDealt: s.totalDamageDealt,
		})
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].wins != ranked[j].wins {
			return ranked[i].wins > ranked[j].wins
		}
		if ranked[i].totalDamageDealt != ranked[j].totalDamageDealt {
			return ranked[i].totalDamageDealt > ranked[j].totalDamageDealt
		}
		return ranked[i].userID < ranked[j].userID
	})

	standings := make([]ArenaStanding, len(ranked))
	for i, r := range ranked {
		standings[i] = ArenaStanding{
			UserID:           r.userID,
			Name:             nameMap[r.userID],
			PhotoURL:         photoMap[r.userID],
			Rank:             i + 1,
			Wins:             r.wins,
			Losses:           r.losses,
			Draws:            r.draws,
			TotalDamageDealt: r.totalDamageDealt,
		}
	}

	// Update leaderboard ONCE per participant: champion=win, others=loss
	match, err := s.gameRepo.GetMatch(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get match for leaderboard update: %w", err)
	}
	if match == nil {
		return nil, fmt.Errorf("match not found for leaderboard update: %s", matchID)
	}

	for _, p := range participants {
		isWin := winnerID != nil && p.UserID == *winnerID
		s.gameRepo.UpdateLeaderboard(ctx, p.UserID, match.ChatID, match.MatchType, isWin, nil, false, false)
	}

	// Complete match ONCE with champion
	s.gameRepo.CompleteMatch(ctx, matchID, winnerID)

	// Bot notifications
	if s.botClient != nil {
		if match.TelegramMessageID != nil && *match.TelegramMessageID != 0 {
			go s.botClient.NotifyParticipantChange(context.Background(), matchID, match.ChatID, *match.TelegramMessageID)
		}

		participantResults := make([]client.BattleParticipantResult, len(ranked))
		for i, r := range ranked {
			participantResults[i] = client.BattleParticipantResult{
				UserID:      r.userID,
				Name:        nameMap[r.userID],
				Wins:        r.wins,
				TotalDamage: r.totalDamageDealt,
				Rank:        i + 1,
			}
		}
		go s.botClient.NotifyBattleComplete(context.Background(), &client.BattleResultNotification{
			MatchID:      matchID,
			ChatID:       match.ChatID,
			MatchType:    string(match.MatchType),
			Format:       "bracket",
			WinnerID:     winnerID,
			NumRounds:    roundNumber,
			Participants: participantResults,
		})
	}

	// Record bracket completion metric
	recordBattleCompletion(s.nrApp, matchID, "bracket", winnerID, false, roundNumber, 0, 0)

	return &BattleResponse{
		MatchID:       matchID,
		WinnerID:      winnerID,
		IsDraw:        false,
		Combats:       []battle.Combat{},
		Events:        []battle.BattleEvent{},
		NumCombats:    0,
		NumRounds:     roundNumber,
		Format:        "bracket",
		BracketRounds: bracketRounds,
		Standings:     standings,
	}, nil
}

// GetBattle retrieves battle results for a participant.
func (s *BattleService) GetBattle(ctx context.Context, matchID string, userID int64) (*BattleResponse, error) {
	defer nrutil.StartSegment(ctx, "service:battle:get-battle")()

	match, err := s.gameRepo.GetMatch(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get match: %w", err)
	}
	if match == nil {
		return nil, apperror.ErrMatchNotFound
	}

	// Verify participant
	participant, err := s.gameRepo.GetParticipant(ctx, matchID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participant: %w", err)
	}
	if participant == nil {
		return nil, apperror.ErrNotParticipant
	}

	rounds, err := s.gameRepo.GetMatchRounds(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get rounds: %w", err)
	}

	// No rounds yet - return minimal response
	if len(rounds) == 0 {
		return &BattleResponse{
			MatchID:     matchID,
			Combats:     []battle.Combat{},
			Events:      []battle.BattleEvent{},
			NumCombats:  0,
			DamageDealt: 0,
			DamageTaken: 0,
		}, nil
	}

	// Get participant names and photo URLs
	participants, err := s.gameRepo.GetMatchParticipants(ctx, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participants: %w", err)
	}

	nameMap := make(map[int64]string)
	photoMap := make(map[int64]string)
	for _, p := range participants {
		name := p.FirstName
		if name == "" {
			name = p.Username
		}
		nameMap[p.UserID] = name
		if p.PhotoURL != nil {
			photoMap[p.UserID] = *p.PhotoURL
		}
	}

	// Determine format from match record
	format := "1v1"
	if match.Format != nil {
		format = string(*match.Format)
	}

	// Branch by format
	switch format {
	case string(repository.MatchFormatBracket):
		return s.getBattleBracket(ctx, matchID, match, rounds, nameMap, photoMap)
	case string(repository.MatchFormatFreeForAll):
		return s.getBattleFreeForAll(ctx, matchID, match, rounds, nameMap, photoMap)
	default:
		// 1v1 and legacy arena: use first round with full events/combats
		return s.getBattle1v1(ctx, matchID, rounds[0], nameMap, userID)
	}
}

// getBattle1v1 builds a BattleResponse for a 1v1 or legacy arena match from a single round.
func (s *BattleService) getBattle1v1(ctx context.Context, matchID string, round *repository.MatchRound, nameMap map[int64]string, userID int64) (*BattleResponse, error) {
	// Parse teams from stored JSON
	var teamACards, teamBCards []*battle.Card
	if err := jsonutil.Unmarshal(round.PlayerATeam, &teamACards); err != nil {
		return nil, fmt.Errorf("failed to parse team A: %w", err)
	}
	if err := jsonutil.Unmarshal(round.PlayerBTeam, &teamBCards); err != nil {
		return nil, fmt.Errorf("failed to parse team B: %w", err)
	}

	// Parse battle events
	var events []battle.BattleEvent
	if err := jsonutil.Unmarshal(round.BattleLog, &events); err != nil {
		return nil, fmt.Errorf("failed to parse battle log: %w", err)
	}

	// Group events into combats
	combats := battle.GroupEventsIntoCombats(events, round.PlayerAID, round.PlayerBID)

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

	// Calculate player-relative damage summary
	var damageDealt, damageTaken int
	if userID == round.PlayerAID {
		damageDealt = round.PlayerADmg
		damageTaken = round.PlayerBDmg
	} else {
		damageDealt = round.PlayerBDmg
		damageTaken = round.PlayerADmg
	}

	// Add New Relic transaction attributes for damage metrics
	if txn := newrelic.FromContext(ctx); txn != nil {
		txn.AddAttribute("damage_dealt", damageDealt)
		txn.AddAttribute("damage_taken", damageTaken)
		txn.AddAttribute("damage_diff", damageDealt-damageTaken)
	}

	return &BattleResponse{
		MatchID:     matchID,
		WinnerID:    round.WinnerID,
		IsDraw:      round.IsDraw,
		Combats:     combats,
		Events:      events,
		NumCombats:  len(combats),
		NumRounds:   round.TotalRounds,
		TeamADamage: round.PlayerADmg,
		TeamBDamage: round.PlayerBDmg,
		TeamAFinal:  teamAFinal,
		TeamBFinal:  teamBFinal,
		PlayerAID:   round.PlayerAID,
		PlayerBID:   round.PlayerBID,
		PlayerAName: nameMap[round.PlayerAID],
		PlayerBName: nameMap[round.PlayerBID],
		DamageDealt: damageDealt,
		DamageTaken: damageTaken,
		Format:      "1v1",
	}, nil
}

// getBattleBracket builds a BattleResponse for a bracket tournament from all rounds.
// Reconstructs the bracket structure by grouping rounds into elimination rounds
// based on how runBracket creates them (sequential pairing with winner advancement).
func (s *BattleService) getBattleBracket(_ context.Context, matchID string, match *repository.Match, rounds []*repository.MatchRound, nameMap map[int64]string, photoMap map[int64]string) (*BattleResponse, error) {
	// Track stats from each round to build standings
	stats := make(map[int64]*playerStats)
	for id := range nameMap {
		stats[id] = &playerStats{}
	}

	// Reconstruct bracket rounds from flat round list.
	// Bracket rounds are stored sequentially: for N participants, first round has N/2 matches,
	// next has N/4, etc. We reconstruct by consuming rounds in decreasing match counts.
	totalParticipants := len(nameMap)
	var bracketRounds []BracketRound
	roundIdx := 0
	matchesInRound := totalParticipants / 2

	for matchesInRound >= 1 && roundIdx < len(rounds) {
		roundName := bracketRoundName(matchesInRound)
		matches := make([]BracketMatch, 0, matchesInRound)

		for m := 0; m < matchesInRound && roundIdx < len(rounds); m++ {
			r := rounds[roundIdx]
			roundIdx++

			// Track stats
			if r.WinnerID != nil {
				if *r.WinnerID == r.PlayerAID {
					stats[r.PlayerAID].wins++
					stats[r.PlayerBID].losses++
				} else {
					stats[r.PlayerBID].wins++
					stats[r.PlayerAID].losses++
				}
			} else if r.IsDraw {
				stats[r.PlayerAID].draws++
				stats[r.PlayerBID].draws++
			}
			stats[r.PlayerAID].totalDamageDealt += r.PlayerADmg
			stats[r.PlayerBID].totalDamageDealt += r.PlayerBDmg

			matches = append(matches, BracketMatch{
				MatchNumber: r.RoundNumber,
				PlayerAID:   r.PlayerAID,
				PlayerAName: nameMap[r.PlayerAID],
				PlayerBID:   r.PlayerBID,
				PlayerBName: nameMap[r.PlayerBID],
				WinnerID:    r.WinnerID,
				IsDraw:      r.IsDraw,
				PlayerADmg:  r.PlayerADmg,
				PlayerBDmg:  r.PlayerBDmg,
				NumRounds:   r.TotalRounds,
			})
		}

		bracketRounds = append(bracketRounds, BracketRound{
			Name:    roundName,
			Matches: matches,
		})

		matchesInRound /= 2
	}

	// Build standings sorted by: wins desc, damage desc, user_id asc
	standings := buildStandings(stats, nameMap, photoMap)

	return &BattleResponse{
		MatchID:       matchID,
		WinnerID:      match.WinnerUserID,
		IsDraw:        false,
		Combats:       []battle.Combat{},
		Events:        []battle.BattleEvent{},
		NumCombats:    0,
		NumRounds:     len(rounds),
		Format:        "bracket",
		BracketRounds: bracketRounds,
		Standings:     standings,
	}, nil
}

// getBattleFreeForAll builds a BattleResponse for a free-for-all tournament from all rounds.
// Each round is a fight between two participants in the round-robin.
func (s *BattleService) getBattleFreeForAll(_ context.Context, matchID string, match *repository.Match, rounds []*repository.MatchRound, nameMap map[int64]string, photoMap map[int64]string) (*BattleResponse, error) {
	stats := make(map[int64]*playerStats)
	for id := range nameMap {
		stats[id] = &playerStats{}
	}

	ffaRounds := make([]FfaRoundSummary, 0, len(rounds))
	for _, r := range rounds {
		// Track stats
		if r.WinnerID != nil {
			if *r.WinnerID == r.PlayerAID {
				stats[r.PlayerAID].wins++
				stats[r.PlayerBID].losses++
			} else {
				stats[r.PlayerBID].wins++
				stats[r.PlayerAID].losses++
			}
		} else if r.IsDraw {
			stats[r.PlayerAID].draws++
			stats[r.PlayerBID].draws++
		}
		stats[r.PlayerAID].totalDamageDealt += r.PlayerADmg
		stats[r.PlayerBID].totalDamageDealt += r.PlayerBDmg

		ffaRounds = append(ffaRounds, FfaRoundSummary{
			RoundNumber: r.RoundNumber,
			PlayerAID:   r.PlayerAID,
			PlayerAName: nameMap[r.PlayerAID],
			PlayerBID:   r.PlayerBID,
			PlayerBName: nameMap[r.PlayerBID],
			WinnerID:    r.WinnerID,
			IsDraw:      r.IsDraw,
			PlayerADmg:  r.PlayerADmg,
			PlayerBDmg:  r.PlayerBDmg,
			NumRounds:   r.TotalRounds,
		})
	}

	// Build standings sorted by: wins desc, damage desc, user_id asc
	standings := buildStandings(stats, nameMap, photoMap)

	return &BattleResponse{
		MatchID:    matchID,
		WinnerID:   match.WinnerUserID,
		IsDraw:     false,
		Combats:    []battle.Combat{},
		Events:     []battle.BattleEvent{},
		NumCombats: 0,
		NumRounds:  len(rounds),
		Format:     "free_for_all",
		Standings:  standings,
		FfaRounds:  ffaRounds,
	}, nil
}

// buildStandings creates sorted ArenaStanding slice from in-memory stats.
// Sorted by: wins desc, damage desc, user_id asc.
func buildStandings(stats map[int64]*playerStats, nameMap map[int64]string, photoMap map[int64]string) []ArenaStanding {
	type rankedPlayer struct {
		userID           int64
		wins             int
		losses           int
		draws            int
		totalDamageDealt int
	}

	ranked := make([]rankedPlayer, 0, len(stats))
	for id, s := range stats {
		ranked = append(ranked, rankedPlayer{
			userID:           id,
			wins:             s.wins,
			losses:           s.losses,
			draws:            s.draws,
			totalDamageDealt: s.totalDamageDealt,
		})
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].wins != ranked[j].wins {
			return ranked[i].wins > ranked[j].wins
		}
		if ranked[i].totalDamageDealt != ranked[j].totalDamageDealt {
			return ranked[i].totalDamageDealt > ranked[j].totalDamageDealt
		}
		return ranked[i].userID < ranked[j].userID
	})

	standings := make([]ArenaStanding, len(ranked))
	for i, r := range ranked {
		standings[i] = ArenaStanding{
			UserID:           r.userID,
			Name:             nameMap[r.userID],
			PhotoURL:         photoMap[r.userID],
			Rank:             i + 1,
			Wins:             r.wins,
			Losses:           r.losses,
			Draws:            r.draws,
			TotalDamageDealt: r.totalDamageDealt,
		}
	}

	return standings
}
