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

// runArena executes arena format with round-robin tournament bracket.
// Each participant faces every other participant. Winner is determined by:
// 1. Most wins across all battles
// 2. Total damage dealt (as tiebreaker if wins are equal)
// 3. Lower user_id (deterministic tiebreaker if damage is tied)
// Completes the match with the tournament winner.
func (s *BattleService) runArena(ctx context.Context, matchID string, participants []*repository.ParticipantWithUser) (*BattleResponse, error) {
	defer nrutil.StartSegment(ctx, "service:battle:run-arena")()

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

	// Notify bot to update message with winner
	match, err := s.gameRepo.GetMatch(ctx, matchID)
	if s.botClient != nil {
		if err == nil && match != nil && match.TelegramMessageID != nil && *match.TelegramMessageID != 0 {
			go s.botClient.NotifyParticipantChange(context.Background(), matchID, match.ChatID, *match.TelegramMessageID)
		}

		// Notify all participants via DM with tournament results
		if err == nil && match != nil {
			// Build ranked participant results
			participantResults := make([]client.BattleParticipantResult, len(participants))
			// Sort by wins desc, then damage desc, then user_id asc for ranking
			sorted := make([]*repository.ParticipantWithUser, len(participants))
			copy(sorted, participants)
			sort.Slice(sorted, func(i, j int) bool {
				if sorted[i].Wins != sorted[j].Wins {
					return sorted[i].Wins > sorted[j].Wins
				}
				if sorted[i].TotalDamageDealt != sorted[j].TotalDamageDealt {
					return sorted[i].TotalDamageDealt > sorted[j].TotalDamageDealt
				}
				return sorted[i].UserID < sorted[j].UserID
			})
			for i, p := range sorted {
				participantResults[i] = client.BattleParticipantResult{
					UserID:      p.UserID,
					Name:        p.FirstName,
					Wins:        p.Wins,
					TotalDamage: p.TotalDamageDealt,
					Rank:        i + 1,
				}
			}
			go s.botClient.NotifyBattleComplete(context.Background(), &client.BattleResultNotification{
				MatchID:      matchID,
				ChatID:       match.ChatID,
				MatchType:    string(match.MatchType),
				Format:       "arena",
				WinnerID:     winnerID,
				NumRounds:    roundNumber,
				Participants: participantResults,
			})
		}
	}

	// Record arena completion metric
	recordBattleCompletion(s.nrApp, matchID, "arena", winnerID, false, roundNumber, 0, 0)

	// Arena format returns summary response (no detailed battle events for replay)
	return &BattleResponse{
		MatchID:    matchID,
		WinnerID:   winnerID,
		IsDraw:     false,
		Combats:    []battle.Combat{},
		Events:     []battle.BattleEvent{},
		NumCombats: 0,
		NumRounds:  roundNumber,
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

	// Get the first round (for 1v1 matches, there's only one round)
	round := rounds[0]

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

	// Calculate player-relative damage summary
	var damageDealt, damageTaken int
	if userID == round.PlayerAID {
		// User is Player A
		damageDealt = round.PlayerADmg
		damageTaken = round.PlayerBDmg
	} else {
		// User is Player B
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
	}, nil
}
