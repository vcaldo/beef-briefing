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

// startDBSegment starts a NewRelic segment for database operations and returns a cleanup function.
// Usage: defer r.startDBSegment(ctx, "operation-name")()
func (r *GameRepository) startDBSegment(ctx context.Context, name string) func() {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:" + name)
		return segment.End
	}
	return func() {} // no-op if no transaction
}

// rowScanner is an interface for sql.Row and sql.Rows
type rowScanner interface {
	Scan(dest ...interface{}) error
}

// matchColumns is the standard column order for Match queries
const matchColumns = `id, chat_id, match_type, format, status, created_at, join_deadline,
       shop_phase_started_at, shop_phase_deadline, battle_started_at, completed_at,
       tournament_date, creator_user_id, current_round, winner_user_id`

// scanMatch scans a row into a Match struct, handling nullable fields
func scanMatch(row rowScanner) (*Match, error) {
	match := &Match{}
	var format sql.NullString
	var tournDate sql.NullString

	err := row.Scan(
		&match.ID, &match.ChatID, &match.MatchType, &format, &match.Status,
		&match.CreatedAt, &match.JoinDeadline, &match.ShopPhaseStartedAt,
		&match.ShopPhaseDeadline, &match.BattleStartedAt, &match.CompletedAt,
		&tournDate, &match.CreatorUserID, &match.CurrentRound, &match.WinnerUserID,
	)
	if err != nil {
		return nil, err
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

// participantColumns is the standard column order for Participant queries
const participantColumns = `id, match_id, user_id, status, joined_at, coins_remaining,
       shop_cards, team, team_order, team_submitted_at,
       placement, wins, losses, total_damage_dealt`

// scanParticipant scans a row into a Participant struct
func scanParticipant(row rowScanner) (*Participant, error) {
	p := &Participant{}
	err := row.Scan(
		&p.ID, &p.MatchID, &p.UserID, &p.Status, &p.JoinedAt,
		&p.CoinsRemaining, &p.ShopCards, &p.Team, &p.TeamOrder,
		&p.TeamSubmittedAt, &p.Placement, &p.Wins, &p.Losses, &p.TotalDamageDealt,
	)
	return p, err
}

// tournamentColumns is the standard column order for RankedTournament queries
const tournamentColumns = `id, chat_id, tournament_date, status,
       announcement_message_id, announced_at, registration_closed_at,
       completed_at, match_id, winner_user_id, participant_count,
       bracket_state, created_at`

// scanTournament scans a row into a RankedTournament struct, handling nullable fields
func scanTournament(row rowScanner) (*RankedTournament, error) {
	t := &RankedTournament{}
	var matchID sql.NullString

	err := row.Scan(
		&t.ID, &t.ChatID, &t.TournamentDate, &t.Status,
		&t.AnnouncementMessageID, &t.AnnouncedAt, &t.RegistrationClosedAt,
		&t.CompletedAt, &matchID, &t.WinnerUserID, &t.ParticipantCount,
		&t.BracketState, &t.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if matchID.Valid {
		t.MatchID = &matchID.String
	}

	return t, nil
}

// roundColumns is the standard column order for MatchRound queries
const roundColumns = `id, match_id, round_number, player_a_id, player_b_id,
       player_a_team, player_b_team, winner_id, is_draw, battle_log,
       player_a_damage, player_b_damage, total_rounds, created_at`

// scanRound scans a row into a MatchRound struct
func scanRound(row rowScanner) (*MatchRound, error) {
	round := &MatchRound{}
	err := row.Scan(
		&round.ID, &round.MatchID, &round.RoundNumber, &round.PlayerAID, &round.PlayerBID,
		&round.PlayerATeam, &round.PlayerBTeam, &round.WinnerID, &round.IsDraw, &round.BattleLog,
		&round.PlayerADmg, &round.PlayerBDmg, &round.TotalRounds, &round.CreatedAt,
	)
	return round, err
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
	ID          int64           `json:"id"`
	MatchID     string          `json:"match_id"`
	RoundNumber int             `json:"round_number"`
	PlayerAID   int64           `json:"player_a_id"`
	PlayerBID   int64           `json:"player_b_id"`
	PlayerATeam json.RawMessage `json:"player_a_team"`
	PlayerBTeam json.RawMessage `json:"player_b_team"`
	WinnerID    *int64          `json:"winner_id,omitempty"`
	IsDraw      bool            `json:"is_draw"`
	BattleLog   json.RawMessage `json:"battle_log"`
	PlayerADmg  int             `json:"player_a_damage"`
	PlayerBDmg  int             `json:"player_b_damage"`
	TotalRounds int             `json:"total_rounds"`
	CreatedAt   time.Time       `json:"created_at"`
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
	FirstName string `json:"first_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

// CreateMatch creates a new match
func (r *GameRepository) CreateMatch(ctx context.Context, chatID int64, matchType MatchType, creatorUserID *int64, tournamentDate *string) (*Match, error) {
	defer r.startDBSegment(ctx, "create-match")()

	id := uuid.New().String()

	var joinDeadline *time.Time
	if matchType == MatchTypeRegular {
		deadline := time.Now().Add(5 * time.Minute)
		joinDeadline = &deadline
	}

	query := fmt.Sprintf(`
		INSERT INTO game_matches (id, chat_id, match_type, creator_user_id, tournament_date, join_deadline)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING %s
	`, matchColumns)

	row := r.db.QueryRowContext(ctx, query, id, chatID, matchType, creatorUserID, tournamentDate, joinDeadline)
	match, err := scanMatch(row)
	if err != nil {
		return nil, fmt.Errorf("failed to create match: %w", err)
	}

	return match, nil
}

// GetMatch retrieves a match by ID
func (r *GameRepository) GetMatch(ctx context.Context, matchID string) (*Match, error) {
	defer r.startDBSegment(ctx, "get-match")()

	query := fmt.Sprintf(`SELECT %s FROM game_matches WHERE id = $1`, matchColumns)

	row := r.db.QueryRowContext(ctx, query, matchID)
	match, err := scanMatch(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get match: %w", err)
	}

	return match, nil
}

// GetActiveMatches retrieves active matches for a chat
func (r *GameRepository) GetActiveMatches(ctx context.Context, chatID int64) ([]*Match, error) {
	defer r.startDBSegment(ctx, "get-active-matches")()

	query := fmt.Sprintf(`
		SELECT %s
		FROM game_matches
		WHERE chat_id = $1 AND status NOT IN ('completed', 'cancelled')
		ORDER BY created_at DESC
	`, matchColumns)

	rows, err := r.db.QueryContext(ctx, query, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to query active matches: %w", err)
	}
	defer rows.Close()

	matches := make([]*Match, 0)
	for rows.Next() {
		match, err := scanMatch(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan match row: %w", err)
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
	defer r.startDBSegment(ctx, "add-participant")()

	query := fmt.Sprintf(`
		INSERT INTO game_match_participants (match_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (match_id, user_id) DO NOTHING
		RETURNING %s
	`, participantColumns)

	row := r.db.QueryRowContext(ctx, query, matchID, userID)
	p, err := scanParticipant(row)
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
	defer r.startDBSegment(ctx, "get-participant")()

	query := fmt.Sprintf(`SELECT %s FROM game_match_participants WHERE match_id = $1 AND user_id = $2`, participantColumns)

	row := r.db.QueryRowContext(ctx, query, matchID, userID)
	p, err := scanParticipant(row)
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
	defer r.startDBSegment(ctx, "create-round")()

	query := fmt.Sprintf(`
		INSERT INTO game_match_rounds (match_id, round_number, player_a_id, player_b_id,
		                               player_a_team, player_b_team, battle_log,
		                               winner_id, is_draw, player_a_damage, player_b_damage, total_rounds)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING %s
	`, roundColumns)

	row := r.db.QueryRowContext(ctx, query,
		matchID, roundNumber, playerAID, playerBID,
		playerATeam, playerBTeam, battleLog,
		winnerID, isDraw, playerADmg, playerBDmg, totalRounds,
	)
	round, err := scanRound(row)
	if err != nil {
		return nil, fmt.Errorf("failed to create round: %w", err)
	}

	return round, nil
}

// GetMatchRounds retrieves all rounds for a match
func (r *GameRepository) GetMatchRounds(ctx context.Context, matchID string) ([]*MatchRound, error) {
	defer r.startDBSegment(ctx, "get-match-rounds")()

	query := fmt.Sprintf(`
		SELECT %s
		FROM game_match_rounds
		WHERE match_id = $1
		ORDER BY round_number, id
	`, roundColumns)

	rows, err := r.db.QueryContext(ctx, query, matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to query rounds: %w", err)
	}
	defer rows.Close()

	rounds := make([]*MatchRound, 0)
	for rows.Next() {
		round, err := scanRound(rows)
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
	defer r.startDBSegment(ctx, "get-matches-by-status")()

	query := fmt.Sprintf(`
		SELECT %s
		FROM game_matches
		WHERE status = $1
		ORDER BY created_at ASC
	`, matchColumns)

	rows, err := r.db.QueryContext(ctx, query, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []*Match
	for rows.Next() {
		match, err := scanMatch(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan match row: %w", err)
		}
		matches = append(matches, match)
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

// =====================================================
// RANKED TOURNAMENT METHODS
// =====================================================

// TournamentStatus enum
type TournamentStatus string

const (
	TournamentStatusScheduled  TournamentStatus = "scheduled"
	TournamentStatusOpen       TournamentStatus = "open"
	TournamentStatusInProgress TournamentStatus = "in_progress"
	TournamentStatusCompleted  TournamentStatus = "completed"
	TournamentStatusSkipped    TournamentStatus = "skipped"
)

// RankedTournament represents a daily ranked tournament
type RankedTournament struct {
	ID                    int64            `json:"id"`
	ChatID                int64            `json:"chat_id"`
	TournamentDate        string           `json:"tournament_date"`
	Status                TournamentStatus `json:"status"`
	AnnouncementMessageID *int64           `json:"announcement_message_id,omitempty"`
	AnnouncedAt           *time.Time       `json:"announced_at,omitempty"`
	RegistrationClosedAt  *time.Time       `json:"registration_closed_at,omitempty"`
	CompletedAt           *time.Time       `json:"completed_at,omitempty"`
	MatchID               *string          `json:"match_id,omitempty"`
	WinnerUserID          *int64           `json:"winner_user_id,omitempty"`
	ParticipantCount      int              `json:"participant_count"`
	BracketState          *json.RawMessage `json:"bracket_state,omitempty"`
	CreatedAt             time.Time        `json:"created_at"`
}

// TournamentParticipant represents a user registered for a tournament
type TournamentParticipant struct {
	ID           int64     `json:"id"`
	TournamentID int64     `json:"tournament_id"`
	UserID       int64     `json:"user_id"`
	JoinedAt     time.Time `json:"joined_at"`
	FirstName    string    `json:"first_name,omitempty"`
	Username     string    `json:"username,omitempty"`
}

// ChatTimezone represents a chat with its timezone
type ChatTimezone struct {
	ChatID   int64  `json:"chat_id"`
	Timezone string `json:"timezone"`
}

// TournamentInfo contains tournament data for scheduler queries
type TournamentInfo struct {
	TournamentID     int64  `json:"tournament_id"`
	ChatID           int64  `json:"chat_id"`
	Timezone         string `json:"timezone"`
	TournamentDate   string `json:"tournament_date"`
	ParticipantCount int64  `json:"participant_count"`
}

// GetOrCreateTournament creates or retrieves a tournament for a specific date
func (r *GameRepository) GetOrCreateTournament(ctx context.Context, chatID int64, date string) (*RankedTournament, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:get-or-create-tournament")
		defer segment.End()
	}

	// Use the database function for atomic get-or-create
	var tournamentID int64
	err := r.db.QueryRowContext(ctx,
		`SELECT get_or_create_tournament($1, $2::date)`,
		chatID, date,
	).Scan(&tournamentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create tournament: %w", err)
	}

	return r.GetTournamentByID(ctx, tournamentID)
}

// GetTournamentByID retrieves a tournament by ID
func (r *GameRepository) GetTournamentByID(ctx context.Context, id int64) (*RankedTournament, error) {
	defer r.startDBSegment(ctx, "get-tournament-by-id")()

	query := fmt.Sprintf(`SELECT %s FROM game_ranked_tournaments WHERE id = $1`, tournamentColumns)

	row := r.db.QueryRowContext(ctx, query, id)
	t, err := scanTournament(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get tournament: %w", err)
	}

	return t, nil
}

// GetTodayTournament retrieves today's tournament for a chat
func (r *GameRepository) GetTodayTournament(ctx context.Context, chatID int64, date string) (*RankedTournament, error) {
	defer r.startDBSegment(ctx, "get-today-tournament")()

	query := fmt.Sprintf(`SELECT %s FROM game_ranked_tournaments WHERE chat_id = $1 AND tournament_date = $2::date`, tournamentColumns)

	row := r.db.QueryRowContext(ctx, query, chatID, date)
	t, err := scanTournament(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get today's tournament: %w", err)
	}

	return t, nil
}

// UpdateTournamentStatus updates the status of a tournament
func (r *GameRepository) UpdateTournamentStatus(ctx context.Context, id int64, status TournamentStatus) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:update-tournament-status")
		defer segment.End()
	}

	query := `UPDATE game_ranked_tournaments SET status = $2 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id, status)
	return err
}

// SetTournamentAnnounced marks a tournament as announced
func (r *GameRepository) SetTournamentAnnounced(ctx context.Context, id int64, messageID int64) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:set-tournament-announced")
		defer segment.End()
	}

	query := `
		UPDATE game_ranked_tournaments
		SET status = 'open',
		    announcement_message_id = $2,
		    announced_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, id, messageID)
	return err
}

// CloseTournamentRegistration closes registration and links to match
func (r *GameRepository) CloseTournamentRegistration(ctx context.Context, id int64, matchID string) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:close-tournament-registration")
		defer segment.End()
	}

	query := `
		UPDATE game_ranked_tournaments
		SET status = 'in_progress',
		    registration_closed_at = NOW(),
		    match_id = $2
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, id, matchID)
	return err
}

// CompleteTournament marks a tournament as completed
func (r *GameRepository) CompleteTournament(ctx context.Context, id int64, winnerUserID *int64) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:complete-tournament")
		defer segment.End()
	}

	query := `
		UPDATE game_ranked_tournaments
		SET status = 'completed',
		    completed_at = NOW(),
		    winner_user_id = $2
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, id, winnerUserID)
	return err
}

// SkipTournament marks a tournament as skipped (no participants)
func (r *GameRepository) SkipTournament(ctx context.Context, id int64) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:skip-tournament")
		defer segment.End()
	}

	query := `
		UPDATE game_ranked_tournaments
		SET status = 'skipped',
		    completed_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// UpdateTournamentBracket updates the bracket state
func (r *GameRepository) UpdateTournamentBracket(ctx context.Context, id int64, bracketState json.RawMessage) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:update-tournament-bracket")
		defer segment.End()
	}

	query := `UPDATE game_ranked_tournaments SET bracket_state = $2 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id, bracketState)
	return err
}

// AddTournamentParticipant adds a user to a tournament
func (r *GameRepository) AddTournamentParticipant(ctx context.Context, tournamentID, userID int64) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:add-tournament-participant")
		defer segment.End()
	}

	query := `
		INSERT INTO game_tournament_participants (tournament_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (tournament_id, user_id) DO NOTHING
	`
	result, err := r.db.ExecContext(ctx, query, tournamentID, userID)
	if err != nil {
		return fmt.Errorf("failed to add tournament participant: %w", err)
	}

	// Update participant count
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		updateQuery := `
			UPDATE game_ranked_tournaments
			SET participant_count = participant_count + 1
			WHERE id = $1
		`
		_, err = r.db.ExecContext(ctx, updateQuery, tournamentID)
		if err != nil {
			return fmt.Errorf("failed to update participant count: %w", err)
		}
	}

	return nil
}

// RemoveTournamentParticipant removes a user from a tournament
func (r *GameRepository) RemoveTournamentParticipant(ctx context.Context, tournamentID, userID int64) error {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:remove-tournament-participant")
		defer segment.End()
	}

	query := `
		DELETE FROM game_tournament_participants
		WHERE tournament_id = $1 AND user_id = $2
	`
	result, err := r.db.ExecContext(ctx, query, tournamentID, userID)
	if err != nil {
		return fmt.Errorf("failed to remove tournament participant: %w", err)
	}

	// Update participant count
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		updateQuery := `
			UPDATE game_ranked_tournaments
			SET participant_count = GREATEST(0, participant_count - 1)
			WHERE id = $1
		`
		_, err = r.db.ExecContext(ctx, updateQuery, tournamentID)
		if err != nil {
			return fmt.Errorf("failed to update participant count: %w", err)
		}
	}

	return nil
}

// GetTournamentParticipants retrieves all participants for a tournament
func (r *GameRepository) GetTournamentParticipants(ctx context.Context, tournamentID int64) ([]*TournamentParticipant, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:get-tournament-participants")
		defer segment.End()
	}

	query := `
		SELECT tp.id, tp.tournament_id, tp.user_id, tp.joined_at,
		       u.first_name, COALESCE(u.username, '')
		FROM game_tournament_participants tp
		JOIN users u ON tp.user_id = u.id
		WHERE tp.tournament_id = $1
		ORDER BY tp.joined_at
	`

	rows, err := r.db.QueryContext(ctx, query, tournamentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query tournament participants: %w", err)
	}
	defer rows.Close()

	participants := make([]*TournamentParticipant, 0)
	for rows.Next() {
		p := &TournamentParticipant{}
		err := rows.Scan(&p.ID, &p.TournamentID, &p.UserID, &p.JoinedAt, &p.FirstName, &p.Username)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tournament participant: %w", err)
		}
		participants = append(participants, p)
	}

	return participants, rows.Err()
}

// IsTournamentParticipant checks if a user is already registered for a tournament
func (r *GameRepository) IsTournamentParticipant(ctx context.Context, tournamentID, userID int64) (bool, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:is-tournament-participant")
		defer segment.End()
	}

	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM game_tournament_participants WHERE tournament_id = $1 AND user_id = $2)`
	err := r.db.QueryRowContext(ctx, query, tournamentID, userID).Scan(&exists)
	return exists, err
}

// GetTournamentsNeedingAnnouncement returns tournaments that should be announced
func (r *GameRepository) GetTournamentsNeedingAnnouncement(ctx context.Context, currentTime time.Time) ([]*TournamentInfo, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:get-tournaments-needing-announcement")
		defer segment.End()
	}

	query := `SELECT * FROM get_tournaments_needing_announcement($1)`

	rows, err := r.db.QueryContext(ctx, query, currentTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query tournaments needing announcement: %w", err)
	}
	defer rows.Close()

	tournaments := make([]*TournamentInfo, 0)
	for rows.Next() {
		t := &TournamentInfo{}
		err := rows.Scan(&t.TournamentID, &t.ChatID, &t.Timezone, &t.TournamentDate)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tournament info: %w", err)
		}
		tournaments = append(tournaments, t)
	}

	return tournaments, rows.Err()
}

// GetTournamentsNeedingClose returns tournaments that should close registration
func (r *GameRepository) GetTournamentsNeedingClose(ctx context.Context, currentTime time.Time) ([]*TournamentInfo, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:get-tournaments-needing-close")
		defer segment.End()
	}

	query := `SELECT * FROM get_tournaments_needing_close($1)`

	rows, err := r.db.QueryContext(ctx, query, currentTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query tournaments needing close: %w", err)
	}
	defer rows.Close()

	tournaments := make([]*TournamentInfo, 0)
	for rows.Next() {
		t := &TournamentInfo{}
		err := rows.Scan(&t.TournamentID, &t.ChatID, &t.Timezone, &t.TournamentDate, &t.ParticipantCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tournament info: %w", err)
		}
		tournaments = append(tournaments, t)
	}

	return tournaments, rows.Err()
}

// GetChatsWithTimezone returns all chats with their timezone settings
func (r *GameRepository) GetChatsWithTimezone(ctx context.Context) ([]*ChatTimezone, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:get-chats-with-timezone")
		defer segment.End()
	}

	query := `
		SELECT id, COALESCE(timezone, 'America/Sao_Paulo')
		FROM chats
		WHERE type IN ('group', 'supergroup')
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query chats with timezone: %w", err)
	}
	defer rows.Close()

	chats := make([]*ChatTimezone, 0)
	for rows.Next() {
		c := &ChatTimezone{}
		err := rows.Scan(&c.ChatID, &c.Timezone)
		if err != nil {
			return nil, fmt.Errorf("failed to scan chat timezone: %w", err)
		}
		chats = append(chats, c)
	}

	return chats, rows.Err()
}

// GetTournamentsByStatus returns tournaments with a given status
func (r *GameRepository) GetTournamentsByStatus(ctx context.Context, status TournamentStatus) ([]*RankedTournament, error) {
	defer r.startDBSegment(ctx, "get-tournaments-by-status")()

	query := fmt.Sprintf(`
		SELECT %s
		FROM game_ranked_tournaments
		WHERE status = $1
		ORDER BY created_at
	`, tournamentColumns)

	rows, err := r.db.QueryContext(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to query tournaments by status: %w", err)
	}
	defer rows.Close()

	tournaments := make([]*RankedTournament, 0)
	for rows.Next() {
		t, err := scanTournament(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tournament: %w", err)
		}
		tournaments = append(tournaments, t)
	}

	return tournaments, rows.Err()
}

// GetTournamentsWithPendingRounds returns in-progress tournaments that need next round execution
func (r *GameRepository) GetTournamentsWithPendingRounds(ctx context.Context) ([]*RankedTournament, error) {
	defer r.startDBSegment(ctx, "get-tournaments-pending-rounds")()

	// Get tournaments in in_progress status where the linked match is in battle_phase
	// Note: Using explicit columns with table alias for JOIN query
	query := `
		SELECT t.id, t.chat_id, t.tournament_date, t.status,
		       t.announcement_message_id, t.announced_at, t.registration_closed_at,
		       t.completed_at, t.match_id, t.winner_user_id, t.participant_count,
		       t.bracket_state, t.created_at
		FROM game_ranked_tournaments t
		JOIN game_matches m ON t.match_id = m.id
		WHERE t.status = 'in_progress'
		  AND m.status = 'battle_phase'
		ORDER BY t.created_at
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tournaments with pending rounds: %w", err)
	}
	defer rows.Close()

	tournaments := make([]*RankedTournament, 0)
	for rows.Next() {
		t, err := scanTournament(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tournament: %w", err)
		}
		tournaments = append(tournaments, t)
	}

	return tournaments, rows.Err()
}

// MatchHistoryEntry represents a match in the user's history
type MatchHistoryEntry struct {
	MatchID      string    `json:"match_id"`
	MatchType    MatchType `json:"match_type"`
	OpponentID   int64     `json:"opponent_id"`
	OpponentName string    `json:"opponent_name"`
	OpponentUser string    `json:"opponent_username,omitempty"`
	Result       string    `json:"result"` // "win", "loss", "draw"
	YourTeam     []byte    `json:"your_team"`
	OpponentTeam []byte    `json:"opponent_team"`
	CompletedAt  time.Time `json:"completed_at"`
}

// GetMatchHistory retrieves a user's match history
func (r *GameRepository) GetMatchHistory(ctx context.Context, chatID, userID int64, limit, offset int) ([]*MatchHistoryEntry, int, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:get-match-history")
		defer segment.End()
	}

	// Count total matches
	var total int
	countQuery := `
		SELECT COUNT(*)
		FROM game_match_rounds r
		JOIN game_matches m ON r.match_id = m.id
		WHERE m.chat_id = $1
		  AND m.status = 'completed'
		  AND (r.player_a_id = $2 OR r.player_b_id = $2)
	`
	if err := r.db.QueryRowContext(ctx, countQuery, chatID, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count match history: %w", err)
	}

	// Get match history
	query := `
		SELECT
			m.id, m.match_type, m.completed_at,
			CASE WHEN r.player_a_id = $2 THEN r.player_b_id ELSE r.player_a_id END as opponent_id,
			u.first_name as opponent_name, COALESCE(u.username, '') as opponent_username,
			CASE
				WHEN r.is_draw THEN 'draw'
				WHEN r.winner_id = $2 THEN 'win'
				ELSE 'loss'
			END as result,
			CASE WHEN r.player_a_id = $2 THEN r.player_a_team ELSE r.player_b_team END as your_team,
			CASE WHEN r.player_a_id = $2 THEN r.player_b_team ELSE r.player_a_team END as opponent_team
		FROM game_match_rounds r
		JOIN game_matches m ON r.match_id = m.id
		JOIN users u ON u.id = CASE WHEN r.player_a_id = $2 THEN r.player_b_id ELSE r.player_a_id END
		WHERE m.chat_id = $1
		  AND m.status = 'completed'
		  AND (r.player_a_id = $2 OR r.player_b_id = $2)
		ORDER BY m.completed_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := r.db.QueryContext(ctx, query, chatID, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query match history: %w", err)
	}
	defer rows.Close()

	entries := make([]*MatchHistoryEntry, 0)
	for rows.Next() {
		e := &MatchHistoryEntry{}
		err := rows.Scan(
			&e.MatchID, &e.MatchType, &e.CompletedAt,
			&e.OpponentID, &e.OpponentName, &e.OpponentUser,
			&e.Result, &e.YourTeam, &e.OpponentTeam,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan match history row: %w", err)
		}
		entries = append(entries, e)
	}

	return entries, total, rows.Err()
}

// H2HRecord represents head-to-head record against an opponent
type H2HRecord struct {
	OpponentID   int64      `json:"opponent_id"`
	OpponentName string     `json:"opponent_name"`
	OpponentUser string     `json:"opponent_username,omitempty"`
	Wins         int        `json:"wins"`
	Losses       int        `json:"losses"`
	LastMatchAt  *time.Time `json:"last_match_at,omitempty"`
}

// GetH2HRecord retrieves head-to-head record for a specific opponent
func (r *GameRepository) GetH2HRecord(ctx context.Context, chatID, userID, opponentID int64) (*H2HRecord, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:get-h2h-record")
		defer segment.End()
	}

	query := `
		SELECT
			l.head_to_head->$3::text,
			u.first_name, COALESCE(u.username, ''), l.last_match_at
		FROM game_leaderboard l
		JOIN users u ON u.id = $3
		WHERE l.user_id = $2 AND l.chat_id = $1
	`

	var h2hJSON []byte
	var firstName, username string
	var lastMatchAt *time.Time

	err := r.db.QueryRowContext(ctx, query, chatID, userID, opponentID).Scan(&h2hJSON, &firstName, &username, &lastMatchAt)
	if err == sql.ErrNoRows {
		// No record exists, return empty
		return &H2HRecord{
			OpponentID:   opponentID,
			OpponentName: firstName,
			OpponentUser: username,
			Wins:         0,
			Losses:       0,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get h2h record: %w", err)
	}

	record := &H2HRecord{
		OpponentID:   opponentID,
		OpponentName: firstName,
		OpponentUser: username,
		LastMatchAt:  lastMatchAt,
	}

	// Parse the JSONB h2h data
	if len(h2hJSON) > 0 {
		var h2hData struct {
			Wins   int `json:"wins"`
			Losses int `json:"losses"`
		}
		if err := json.Unmarshal(h2hJSON, &h2hData); err == nil {
			record.Wins = h2hData.Wins
			record.Losses = h2hData.Losses
		}
	}

	return record, nil
}

// GetRecentMatchesVsOpponent retrieves recent matches against a specific opponent
func (r *GameRepository) GetRecentMatchesVsOpponent(ctx context.Context, chatID, userID, opponentID int64, limit int) ([]*MatchHistoryEntry, error) {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:get-recent-matches-vs-opponent")
		defer segment.End()
	}

	query := `
		SELECT
			m.id, m.match_type, m.completed_at,
			$3::bigint as opponent_id,
			u.first_name as opponent_name, COALESCE(u.username, '') as opponent_username,
			CASE
				WHEN r.is_draw THEN 'draw'
				WHEN r.winner_id = $2 THEN 'win'
				ELSE 'loss'
			END as result,
			CASE WHEN r.player_a_id = $2 THEN r.player_a_team ELSE r.player_b_team END as your_team,
			CASE WHEN r.player_a_id = $2 THEN r.player_b_team ELSE r.player_a_team END as opponent_team
		FROM game_match_rounds r
		JOIN game_matches m ON r.match_id = m.id
		JOIN users u ON u.id = $3
		WHERE m.chat_id = $1
		  AND m.status = 'completed'
		  AND ((r.player_a_id = $2 AND r.player_b_id = $3) OR (r.player_a_id = $3 AND r.player_b_id = $2))
		ORDER BY m.completed_at DESC
		LIMIT $4
	`

	rows, err := r.db.QueryContext(ctx, query, chatID, userID, opponentID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent matches: %w", err)
	}
	defer rows.Close()

	entries := make([]*MatchHistoryEntry, 0)
	for rows.Next() {
		e := &MatchHistoryEntry{}
		err := rows.Scan(
			&e.MatchID, &e.MatchType, &e.CompletedAt,
			&e.OpponentID, &e.OpponentName, &e.OpponentUser,
			&e.Result, &e.YourTeam, &e.OpponentTeam,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan recent match row: %w", err)
		}
		entries = append(entries, e)
	}

	return entries, rows.Err()
}
