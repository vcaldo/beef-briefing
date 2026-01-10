package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// GameRepository handles database operations for the arena game
type GameRepository struct {
	db    *sql.DB
	nrApp *newrelic.Application
}

// NewGameRepository creates a new GameRepository
func NewGameRepository(db *sql.DB, nrApp *newrelic.Application) *GameRepository {
	return &GameRepository{db: db, nrApp: nrApp}
}

// MatchType enum
type MatchType string

const (
	MatchTypeRanked  MatchType = "ranked"
	MatchTypeRegular MatchType = "regular"
)

// MatchFormat enum
type MatchFormat string

const (
	MatchFormat1v1   MatchFormat = "1v1"
	MatchFormatArena MatchFormat = "arena"
)

// MatchStatus enum
type MatchStatus string

const (
	MatchStatusOpen        MatchStatus = "open"
	MatchStatusShopPhase   MatchStatus = "shop_phase"
	MatchStatusBattlePhase MatchStatus = "battle_phase"
	MatchStatusCompleted   MatchStatus = "completed"
	MatchStatusCancelled   MatchStatus = "cancelled"
)

// ParticipantStatus enum
type ParticipantStatus string

const (
	ParticipantStatusJoined     ParticipantStatus = "joined"
	ParticipantStatusReady      ParticipantStatus = "ready"
	ParticipantStatusEliminated ParticipantStatus = "eliminated"
	ParticipantStatusWinner     ParticipantStatus = "winner"
)

// Match represents a game match
type Match struct {
	ID                 string       `json:"id"`
	ChatID             int64        `json:"chat_id"`
	MatchType          MatchType    `json:"match_type"`
	Format             *MatchFormat `json:"format,omitempty"`
	Status             MatchStatus  `json:"status"`
	CreatedAt          time.Time    `json:"created_at"`
	JoinDeadline       *time.Time   `json:"join_deadline,omitempty"`
	ShopPhaseStartedAt *time.Time   `json:"shop_phase_started_at,omitempty"`
	ShopPhaseDeadline  *time.Time   `json:"shop_phase_deadline,omitempty"`
	BattleStartedAt    *time.Time   `json:"battle_started_at,omitempty"`
	CompletedAt        *time.Time   `json:"completed_at,omitempty"`
	TournamentDate     *string      `json:"tournament_date,omitempty"`
	CreatorUserID      *int64       `json:"creator_user_id,omitempty"`
	CurrentRound       int          `json:"current_round"`
	WinnerUserID       *int64       `json:"winner_user_id,omitempty"`
}

// Participant represents a match participant
type Participant struct {
	ID               int64             `json:"id"`
	MatchID          string            `json:"match_id"`
	UserID           int64             `json:"user_id"`
	Status           ParticipantStatus `json:"status"`
	JoinedAt         time.Time         `json:"joined_at"`
	CoinsRemaining   int               `json:"coins_remaining"`
	ShopCards        *json.RawMessage  `json:"shop_cards,omitempty"`
	Team             *json.RawMessage  `json:"team,omitempty"`
	TeamOrder        pq.Int64Array     `json:"team_order"`
	TeamSubmittedAt  *time.Time        `json:"team_submitted_at,omitempty"`
	Placement        *int              `json:"placement,omitempty"`
	Wins             int               `json:"wins"`
	Losses           int               `json:"losses"`
	TotalDamageDealt int               `json:"total_damage_dealt"`
}

// ParticipantWithUser includes user info
type ParticipantWithUser struct {
	Participant
	FirstName string `json:"first_name"`
	Username  string `json:"username,omitempty"`
}

// MatchRound represents a battle round
type MatchRound struct {
	ID           int64           `json:"id"`
	MatchID      string          `json:"match_id"`
	RoundNumber  int             `json:"round_number"`
	PlayerAID    int64           `json:"player_a_id"`
	PlayerBID    int64           `json:"player_b_id"`
	PlayerATeam  json.RawMessage `json:"player_a_team"`
	PlayerBTeam  json.RawMessage `json:"player_b_team"`
	WinnerID     *int64          `json:"winner_id,omitempty"`
	IsDraw       bool            `json:"is_draw"`
	BattleLog    json.RawMessage `json:"battle_log"`
	PlayerADmg   int             `json:"player_a_damage"`
	PlayerBDmg   int             `json:"player_b_damage"`
	TotalRounds  int             `json:"total_rounds"`
	CreatedAt    time.Time       `json:"created_at"`
}

// LeaderboardEntry represents a user's game stats
type LeaderboardEntry struct {
	UserID                  int64           `json:"user_id"`
	ChatID                  int64           `json:"chat_id"`
	RankedWins              int             `json:"ranked_wins"`
	RankedLosses            int             `json:"ranked_losses"`
	RankedTournamentsPlayed int             `json:"ranked_tournaments_played"`
	RankedTournamentsWon    int             `json:"ranked_tournaments_won"`
	RankedCurrentStreak     int             `json:"ranked_current_streak"`
	RankedBestStreak        int             `json:"ranked_best_streak"`
	RegularWins             int             `json:"regular_wins"`
	RegularLosses           int             `json:"regular_losses"`
	RegularMatchesPlayed    int             `json:"regular_matches_played"`
	RegularCurrentStreak    int             `json:"regular_current_streak"`
	RegularBestStreak       int             `json:"regular_best_streak"`
	HeadToHead              json.RawMessage `json:"head_to_head"`
	FirstMatchAt            *time.Time      `json:"first_match_at,omitempty"`
	LastMatchAt             *time.Time      `json:"last_match_at,omitempty"`
	// User info from join
	FirstName               string          `json:"first_name,omitempty"`
	Username                string          `json:"username,omitempty"`
}

// CreateMatch creates a new match
func (r *GameRepository) CreateMatch(ctx context.Context, chatID int64, matchType MatchType, creatorUserID *int64, tournamentDate *string) (*Match, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:create-match")
		defer segment.End()
	}

	id := uuid.New().String()

	var joinDeadline *time.Time
	if matchType == MatchTypeRegular {
		deadline := time.Now().Add(5 * time.Minute)
		joinDeadline = &deadline
	}

	query := `
		INSERT INTO game_matches (id, chat_id, match_type, creator_user_id, tournament_date, join_deadline)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, chat_id, match_type, format, status, created_at, join_deadline,
		          shop_phase_started_at, shop_phase_deadline, battle_started_at, completed_at,
		          tournament_date, creator_user_id, current_round, winner_user_id
	`

	match := &Match{}
	var format sql.NullString
	var tournDate sql.NullString

	err := r.db.QueryRowContext(ctx, query,
		id, chatID, matchType, creatorUserID, tournamentDate, joinDeadline,
	).Scan(
		&match.ID, &match.ChatID, &match.MatchType, &format, &match.Status,
		&match.CreatedAt, &match.JoinDeadline, &match.ShopPhaseStartedAt,
		&match.ShopPhaseDeadline, &match.BattleStartedAt, &match.CompletedAt,
		&tournDate, &match.CreatorUserID, &match.CurrentRound, &match.WinnerUserID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create match: %w", err)
	}

	if format.Valid {
		f := MatchFormat(format.String)
		match.Format = &f
	}
	if tournDate.Valid {
		match.TournamentDate = &tournDate.String
	}

	return match, nil
}

// GetMatch retrieves a match by ID
func (r *GameRepository) GetMatch(ctx context.Context, matchID string) (*Match, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:get-match")
		defer segment.End()
	}

	query := `
		SELECT id, chat_id, match_type, format, status, created_at, join_deadline,
		       shop_phase_started_at, shop_phase_deadline, battle_started_at, completed_at,
		       tournament_date, creator_user_id, current_round, winner_user_id
		FROM game_matches
		WHERE id = $1
	`

	match := &Match{}
	var format sql.NullString
	var tournDate sql.NullString

	err := r.db.QueryRowContext(ctx, query, matchID).Scan(
		&match.ID, &match.ChatID, &match.MatchType, &format, &match.Status,
		&match.CreatedAt, &match.JoinDeadline, &match.ShopPhaseStartedAt,
		&match.ShopPhaseDeadline, &match.BattleStartedAt, &match.CompletedAt,
		&tournDate, &match.CreatorUserID, &match.CurrentRound, &match.WinnerUserID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get match: %w", err)
	}

	if format.Valid {
		f := MatchFormat(format.String)
		match.Format = &f
	}
	if tournDate.Valid {
		match.TournamentDate = &tournDate.String
	}

	return match, nil
}

// GetActiveMatches retrieves active matches for a chat
func (r *GameRepository) GetActiveMatches(ctx context.Context, chatID int64) ([]*Match, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:get-active-matches")
		defer segment.End()
	}

	query := `
		SELECT id, chat_id, match_type, format, status, created_at, join_deadline,
		       shop_phase_started_at, shop_phase_deadline, battle_started_at, completed_at,
		       tournament_date, creator_user_id, current_round, winner_user_id
		FROM game_matches
		WHERE chat_id = $1 AND status NOT IN ('completed', 'cancelled')
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to query active matches: %w", err)
	}
	defer rows.Close()

	matches := make([]*Match, 0)
	for rows.Next() {
		match := &Match{}
		var format sql.NullString
		var tournDate sql.NullString

		err := rows.Scan(
			&match.ID, &match.ChatID, &match.MatchType, &format, &match.Status,
			&match.CreatedAt, &match.JoinDeadline, &match.ShopPhaseStartedAt,
			&match.ShopPhaseDeadline, &match.BattleStartedAt, &match.CompletedAt,
			&tournDate, &match.CreatorUserID, &match.CurrentRound, &match.WinnerUserID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan match row: %w", err)
		}

		if format.Valid {
			f := MatchFormat(format.String)
			match.Format = &f
		}
		if tournDate.Valid {
			match.TournamentDate = &tournDate.String
		}

		matches = append(matches, match)
	}

	return matches, rows.Err()
}

// UpdateMatchStatus updates the status of a match
func (r *GameRepository) UpdateMatchStatus(ctx context.Context, matchID string, status MatchStatus) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:update-match-status")
		defer segment.End()
	}

	query := `UPDATE game_matches SET status = $2 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, matchID, status)
	return err
}

// UpdateMatchFormat sets the match format
func (r *GameRepository) UpdateMatchFormat(ctx context.Context, matchID string, format MatchFormat) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:update-match-format")
		defer segment.End()
	}

	query := `UPDATE game_matches SET format = $2 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, matchID, format)
	return err
}

// StartShopPhase transitions match to shop phase with format
func (r *GameRepository) StartShopPhase(ctx context.Context, matchID string, format MatchFormat, deadline time.Time) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:start-shop-phase")
		defer segment.End()
	}

	query := `
		UPDATE game_matches
		SET status = 'shop_phase',
		    format = $2,
		    shop_phase_started_at = NOW(),
		    shop_phase_deadline = $3
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, matchID, format, deadline)
	return err
}

// StartBattlePhase transitions match to battle phase
func (r *GameRepository) StartBattlePhase(ctx context.Context, matchID string) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:start-battle-phase")
		defer segment.End()
	}

	query := `
		UPDATE game_matches
		SET status = 'battle_phase',
		    battle_started_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, matchID)
	return err
}

// CompleteMatch marks match as completed
func (r *GameRepository) CompleteMatch(ctx context.Context, matchID string, winnerUserID *int64) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:complete-match")
		defer segment.End()
	}

	query := `
		UPDATE game_matches
		SET status = 'completed',
		    completed_at = NOW(),
		    winner_user_id = $2
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, matchID, winnerUserID)
	return err
}

// AddParticipant adds a user to a match
func (r *GameRepository) AddParticipant(ctx context.Context, matchID string, userID int64) (*Participant, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:add-participant")
		defer segment.End()
	}

	query := `
		INSERT INTO game_match_participants (match_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (match_id, user_id) DO NOTHING
		RETURNING id, match_id, user_id, status, joined_at, coins_remaining,
		          shop_cards, team, team_order, team_submitted_at,
		          placement, wins, losses, total_damage_dealt
	`

	p := &Participant{}
	err := r.db.QueryRowContext(ctx, query, matchID, userID).Scan(
		&p.ID, &p.MatchID, &p.UserID, &p.Status, &p.JoinedAt,
		&p.CoinsRemaining, &p.ShopCards, &p.Team, &p.TeamOrder,
		&p.TeamSubmittedAt, &p.Placement, &p.Wins, &p.Losses, &p.TotalDamageDealt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			// Already exists, fetch existing
			return r.GetParticipant(ctx, matchID, userID)
		}
		return nil, fmt.Errorf("failed to add participant: %w", err)
	}

	return p, nil
}

// GetParticipant retrieves a participant
func (r *GameRepository) GetParticipant(ctx context.Context, matchID string, userID int64) (*Participant, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:get-participant")
		defer segment.End()
	}

	query := `
		SELECT id, match_id, user_id, status, joined_at, coins_remaining,
		       shop_cards, team, team_order, team_submitted_at,
		       placement, wins, losses, total_damage_dealt
		FROM game_match_participants
		WHERE match_id = $1 AND user_id = $2
	`

	p := &Participant{}
	err := r.db.QueryRowContext(ctx, query, matchID, userID).Scan(
		&p.ID, &p.MatchID, &p.UserID, &p.Status, &p.JoinedAt,
		&p.CoinsRemaining, &p.ShopCards, &p.Team, &p.TeamOrder,
		&p.TeamSubmittedAt, &p.Placement, &p.Wins, &p.Losses, &p.TotalDamageDealt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get participant: %w", err)
	}

	return p, nil
}

// GetMatchParticipants retrieves all participants for a match
func (r *GameRepository) GetMatchParticipants(ctx context.Context, matchID string) ([]*ParticipantWithUser, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:get-match-participants")
		defer segment.End()
	}

	query := `
		SELECT p.id, p.match_id, p.user_id, p.status, p.joined_at, p.coins_remaining,
		       p.shop_cards, p.team, p.team_order, p.team_submitted_at,
		       p.placement, p.wins, p.losses, p.total_damage_dealt,
		       u.first_name, COALESCE(u.username, '')
		FROM game_match_participants p
		JOIN users u ON p.user_id = u.id
		WHERE p.match_id = $1
		ORDER BY p.joined_at
	`

	rows, err := r.db.QueryContext(ctx, query, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to query participants: %w", err)
	}
	defer rows.Close()

	participants := make([]*ParticipantWithUser, 0)
	for rows.Next() {
		p := &ParticipantWithUser{}
		err := rows.Scan(
			&p.ID, &p.MatchID, &p.UserID, &p.Status, &p.JoinedAt,
			&p.CoinsRemaining, &p.ShopCards, &p.Team, &p.TeamOrder,
			&p.TeamSubmittedAt, &p.Placement, &p.Wins, &p.Losses, &p.TotalDamageDealt,
			&p.FirstName, &p.Username,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan participant row: %w", err)
		}
		participants = append(participants, p)
	}

	return participants, rows.Err()
}

// RemoveParticipant removes a user from a match
func (r *GameRepository) RemoveParticipant(ctx context.Context, matchID string, userID int64) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:remove-participant")
		defer segment.End()
	}

	query := `DELETE FROM game_match_participants WHERE match_id = $1 AND user_id = $2`
	_, err := r.db.ExecContext(ctx, query, matchID, userID)
	return err
}

// UpdateParticipantShop updates a participant's shop state
func (r *GameRepository) UpdateParticipantShop(ctx context.Context, matchID string, userID int64, coins int, shopCards, team json.RawMessage, teamOrder []int64) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:update-participant-shop")
		defer segment.End()
	}

	query := `
		UPDATE game_match_participants
		SET coins_remaining = $3,
		    shop_cards = $4,
		    team = $5,
		    team_order = $6
		WHERE match_id = $1 AND user_id = $2
	`
	_, err := r.db.ExecContext(ctx, query, matchID, userID, coins, shopCards, team, pq.Array(teamOrder))
	return err
}

// SubmitTeam marks a participant's team as submitted
func (r *GameRepository) SubmitTeam(ctx context.Context, matchID string, userID int64) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:submit-team")
		defer segment.End()
	}

	query := `
		UPDATE game_match_participants
		SET status = 'ready',
		    team_submitted_at = NOW()
		WHERE match_id = $1 AND user_id = $2
	`
	_, err := r.db.ExecContext(ctx, query, matchID, userID)
	return err
}

// CreateRound creates a battle round record
func (r *GameRepository) CreateRound(ctx context.Context, matchID string, roundNumber int, playerAID, playerBID int64, playerATeam, playerBTeam, battleLog json.RawMessage, winnerID *int64, isDraw bool, playerADmg, playerBDmg, totalRounds int) (*MatchRound, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:create-round")
		defer segment.End()
	}

	query := `
		INSERT INTO game_match_rounds (match_id, round_number, player_a_id, player_b_id,
		                               player_a_team, player_b_team, battle_log,
		                               winner_id, is_draw, player_a_damage, player_b_damage, total_rounds)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, match_id, round_number, player_a_id, player_b_id,
		          player_a_team, player_b_team, winner_id, is_draw, battle_log,
		          player_a_damage, player_b_damage, total_rounds, created_at
	`

	round := &MatchRound{}
	err := r.db.QueryRowContext(ctx, query,
		matchID, roundNumber, playerAID, playerBID,
		playerATeam, playerBTeam, battleLog,
		winnerID, isDraw, playerADmg, playerBDmg, totalRounds,
	).Scan(
		&round.ID, &round.MatchID, &round.RoundNumber, &round.PlayerAID, &round.PlayerBID,
		&round.PlayerATeam, &round.PlayerBTeam, &round.WinnerID, &round.IsDraw, &round.BattleLog,
		&round.PlayerADmg, &round.PlayerBDmg, &round.TotalRounds, &round.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create round: %w", err)
	}

	return round, nil
}

// GetMatchRounds retrieves all rounds for a match
func (r *GameRepository) GetMatchRounds(ctx context.Context, matchID string) ([]*MatchRound, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:get-match-rounds")
		defer segment.End()
	}

	query := `
		SELECT id, match_id, round_number, player_a_id, player_b_id,
		       player_a_team, player_b_team, winner_id, is_draw, battle_log,
		       player_a_damage, player_b_damage, total_rounds, created_at
		FROM game_match_rounds
		WHERE match_id = $1
		ORDER BY round_number, id
	`

	rows, err := r.db.QueryContext(ctx, query, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to query rounds: %w", err)
	}
	defer rows.Close()

	rounds := make([]*MatchRound, 0)
	for rows.Next() {
		round := &MatchRound{}
		err := rows.Scan(
			&round.ID, &round.MatchID, &round.RoundNumber, &round.PlayerAID, &round.PlayerBID,
			&round.PlayerATeam, &round.PlayerBTeam, &round.WinnerID, &round.IsDraw, &round.BattleLog,
			&round.PlayerADmg, &round.PlayerBDmg, &round.TotalRounds, &round.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan round row: %w", err)
		}
		rounds = append(rounds, round)
	}

	return rounds, rows.Err()
}

// GetLeaderboard retrieves leaderboard entries for a chat
func (r *GameRepository) GetLeaderboard(ctx context.Context, chatID int64, matchType MatchType, limit, offset int) ([]*LeaderboardEntry, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:get-leaderboard")
		defer segment.End()
	}

	orderBy := "ranked_wins DESC, ranked_losses ASC"
	if matchType == MatchTypeRegular {
		orderBy = "regular_wins DESC, regular_losses ASC"
	}

	query := fmt.Sprintf(`
		SELECT l.user_id, l.chat_id,
		       l.ranked_wins, l.ranked_losses, l.ranked_tournaments_played, l.ranked_tournaments_won,
		       l.ranked_current_streak, l.ranked_best_streak,
		       l.regular_wins, l.regular_losses, l.regular_matches_played,
		       l.regular_current_streak, l.regular_best_streak,
		       l.head_to_head, l.first_match_at, l.last_match_at,
		       u.first_name, COALESCE(u.username, '')
		FROM game_leaderboard l
		JOIN users u ON l.user_id = u.id
		WHERE l.chat_id = $1
		ORDER BY %s
		LIMIT $2 OFFSET $3
	`, orderBy)

	rows, err := r.db.QueryContext(ctx, query, chatID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query leaderboard: %w", err)
	}
	defer rows.Close()

	entries := make([]*LeaderboardEntry, 0)
	for rows.Next() {
		e := &LeaderboardEntry{}
		err := rows.Scan(
			&e.UserID, &e.ChatID,
			&e.RankedWins, &e.RankedLosses, &e.RankedTournamentsPlayed, &e.RankedTournamentsWon,
			&e.RankedCurrentStreak, &e.RankedBestStreak,
			&e.RegularWins, &e.RegularLosses, &e.RegularMatchesPlayed,
			&e.RegularCurrentStreak, &e.RegularBestStreak,
			&e.HeadToHead, &e.FirstMatchAt, &e.LastMatchAt,
			&e.FirstName, &e.Username,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan leaderboard row: %w", err)
		}
		entries = append(entries, e)
	}

	return entries, rows.Err()
}

// UpdateLeaderboard updates a user's leaderboard stats (calls DB function)
func (r *GameRepository) UpdateLeaderboard(ctx context.Context, userID, chatID int64, matchType MatchType, isWin bool, opponentID *int64, isTournamentWin bool) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:update-leaderboard")
		defer segment.End()
	}

	query := `SELECT update_game_leaderboard($1, $2, $3, $4, $5, $6)`
	_, err := r.db.ExecContext(ctx, query, userID, chatID, matchType, isWin, opponentID, isTournamentWin)
	return err
}

// GetParticipantCount returns the number of participants in a match
func (r *GameRepository) GetParticipantCount(ctx context.Context, matchID string) (int, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:count-participants")
		defer segment.End()
	}

	var count int
	query := `SELECT COUNT(*) FROM game_match_participants WHERE match_id = $1`
	err := r.db.QueryRowContext(ctx, query, matchID).Scan(&count)
	return count, err
}

// GetReadyParticipantCount returns number of ready participants
func (r *GameRepository) GetReadyParticipantCount(ctx context.Context, matchID string) (int, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:count-ready-participants")
		defer segment.End()
	}

	var count int
	query := `SELECT COUNT(*) FROM game_match_participants WHERE match_id = $1 AND status = 'ready'`
	err := r.db.QueryRowContext(ctx, query, matchID).Scan(&count)
	return count, err
}

// GetMatchesByStatus returns all matches with a given status
func (r *GameRepository) GetMatchesByStatus(ctx context.Context, status MatchStatus) ([]*Match, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:get-matches-by-status")
		defer segment.End()
	}

	query := `
		SELECT id, chat_id, match_type, format, status, creator_user_id,
		       tournament_date, created_at, join_deadline, shop_phase_started_at,
		       shop_phase_deadline, battle_started_at, completed_at, winner_user_id
		FROM game_matches
		WHERE status = $1
		ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []*Match
	for rows.Next() {
		m := &Match{}
		err := rows.Scan(
			&m.ID, &m.ChatID, &m.MatchType, &m.Format, &m.Status, &m.CreatorUserID,
			&m.TournamentDate, &m.CreatedAt, &m.JoinDeadline, &m.ShopPhaseStartedAt,
			&m.ShopPhaseDeadline, &m.BattleStartedAt, &m.CompletedAt, &m.WinnerUserID,
		)
		if err != nil {
			return nil, err
		}
		matches = append(matches, m)
	}

	return matches, rows.Err()
}

// CancelMatch marks a match as cancelled
func (r *GameRepository) CancelMatch(ctx context.Context, matchID string) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:cancel-match")
		defer segment.End()
	}

	query := `
		UPDATE game_matches
		SET status = 'cancelled',
		    completed_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, matchID)
	return err
}
