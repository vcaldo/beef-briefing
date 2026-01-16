package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"beef-briefing/apps/api-service/internal/jsonutil"
	"beef-briefing/apps/api-service/internal/nrutil"

	"github.com/google/uuid"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// MatchRepository handles database operations for matches and rounds
type MatchRepository struct {
	db    *sql.DB
	nrApp *newrelic.Application
}

// NewMatchRepository creates a new MatchRepository
func NewMatchRepository(db *sql.DB, nrApp *newrelic.Application) *MatchRepository {
	return &MatchRepository{db: db, nrApp: nrApp}
}

// startDBSegment starts a NewRelic segment for database operations and returns a cleanup function.
func (r *MatchRepository) startDBSegment(ctx context.Context, name string) func() {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:" + name)
		return segment.End
	}
	return func() {}
}

// CreateMatch creates a new match
func (r *MatchRepository) CreateMatch(ctx context.Context, chatID int64, matchType MatchType, creatorUserID *int64, tournamentDate *string) (*Match, error) {
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
func (r *MatchRepository) GetMatch(ctx context.Context, matchID string) (*Match, error) {
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
func (r *MatchRepository) GetActiveMatches(ctx context.Context, chatID int64) ([]*Match, error) {
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
func (r *MatchRepository) UpdateMatchStatus(ctx context.Context, matchID string, status MatchStatus) error {
	defer nrutil.StartSegment(ctx, "db:update-match-status")()

	query := `UPDATE game_matches SET status = $2 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, matchID, status)
	return err
}

// UpdateMatchFormat sets the match format
func (r *MatchRepository) UpdateMatchFormat(ctx context.Context, matchID string, format MatchFormat) error {
	defer nrutil.StartSegment(ctx, "db:update-match-format")()

	query := `UPDATE game_matches SET format = $2 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, matchID, format)
	return err
}

// StartShopPhase transitions match to shop phase with format
func (r *MatchRepository) StartShopPhase(ctx context.Context, matchID string, format MatchFormat, deadline time.Time) error {
	defer nrutil.StartSegment(ctx, "db:start-shop-phase")()

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
func (r *MatchRepository) StartBattlePhase(ctx context.Context, matchID string) error {
	defer nrutil.StartSegment(ctx, "db:start-battle-phase")()

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
func (r *MatchRepository) CompleteMatch(ctx context.Context, matchID string, winnerUserID *int64) error {
	defer nrutil.StartSegment(ctx, "db:complete-match")()

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

// CancelMatch marks a match as cancelled
func (r *MatchRepository) CancelMatch(ctx context.Context, matchID string) error {
	defer nrutil.StartSegment(ctx, "db:cancel-match")()

	query := `
		UPDATE game_matches
		SET status = 'cancelled',
		    completed_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, matchID)
	return err
}

// GetMatchesByStatus returns all matches with a given status
func (r *MatchRepository) GetMatchesByStatus(ctx context.Context, status MatchStatus) ([]*Match, error) {
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

// CreateRound creates a battle round record
func (r *MatchRepository) CreateRound(ctx context.Context, matchID string, roundNumber int, playerAID, playerBID int64, playerATeam, playerBTeam, battleLog json.RawMessage, winnerID *int64, isDraw bool, playerADmg, playerBDmg, totalRounds int) (*MatchRound, error) {
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
func (r *MatchRepository) GetMatchRounds(ctx context.Context, matchID string) ([]*MatchRound, error) {
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
func (r *MatchRepository) GetLeaderboard(ctx context.Context, chatID int64, matchType MatchType, limit, offset int) ([]*LeaderboardEntry, error) {
	defer nrutil.StartSegment(ctx, "db:get-leaderboard")()

	orderBy := "ranked_wins DESC, ranked_losses ASC"
	if matchType == MatchTypeRegular {
		orderBy = "regular_wins DESC, regular_losses ASC"
	}

	query := fmt.Sprintf(`
		SELECT l.user_id, l.chat_id,
		       l.ranked_wins, l.ranked_losses, l.ranked_draws, l.ranked_tournaments_played, l.ranked_tournaments_won,
		       l.ranked_current_streak, l.ranked_best_streak,
		       l.regular_wins, l.regular_losses, l.regular_draws, l.regular_matches_played,
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
			&e.RankedWins, &e.RankedLosses, &e.RankedDraws, &e.RankedTournamentsPlayed, &e.RankedTournamentsWon,
			&e.RankedCurrentStreak, &e.RankedBestStreak,
			&e.RegularWins, &e.RegularLosses, &e.RegularDraws, &e.RegularMatchesPlayed,
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
func (r *MatchRepository) UpdateLeaderboard(ctx context.Context, userID, chatID int64, matchType MatchType, isWin bool, opponentID *int64, isTournamentWin bool, isDraw bool) error {
	defer nrutil.StartSegment(ctx, "db:update-leaderboard")()

	query := `SELECT update_game_leaderboard($1, $2, $3, $4, $5, $6, $7)`
	_, err := r.db.ExecContext(ctx, query, userID, chatID, matchType, isWin, opponentID, isTournamentWin, isDraw)
	return err
}

// GetMatchHistory retrieves a user's match history
func (r *MatchRepository) GetMatchHistory(ctx context.Context, chatID, userID int64, limit, offset int) ([]*MatchHistoryEntry, int, error) {
	defer nrutil.StartSegment(ctx, "db:get-match-history")()

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

	// Get match history with user profile photos
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
			CASE WHEN r.player_a_id = $2 THEN r.player_b_team ELSE r.player_a_team END as opponent_team,
			your_photo.minio_object_key as your_photo_key,
			opp_photo.minio_object_key as opponent_photo_key
		FROM game_match_rounds r
		JOIN game_matches m ON r.match_id = m.id
		JOIN users u ON u.id = CASE WHEN r.player_a_id = $2 THEN r.player_b_id ELSE r.player_a_id END
		LEFT JOIN LATERAL (
			SELECT minio_object_key FROM user_profile_photos
			WHERE user_id = $2 ORDER BY width DESC LIMIT 1
		) your_photo ON true
		LEFT JOIN LATERAL (
			SELECT minio_object_key FROM user_profile_photos
			WHERE user_id = CASE WHEN r.player_a_id = $2 THEN r.player_b_id ELSE r.player_a_id END
			ORDER BY width DESC LIMIT 1
		) opp_photo ON true
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
			&e.YourPhotoKey, &e.OpponentPhotoKey,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan match history row: %w", err)
		}
		entries = append(entries, e)
	}

	return entries, total, rows.Err()
}

// GetH2HRecord retrieves head-to-head record for a specific opponent
func (r *MatchRepository) GetH2HRecord(ctx context.Context, chatID, userID, opponentID int64) (*H2HRecord, error) {
	defer nrutil.StartSegment(ctx, "db:get-h2h-record")()

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
			Draws  int `json:"draws"`
		}
		if err := jsonutil.Unmarshal(h2hJSON, &h2hData); err == nil {
			record.Wins = h2hData.Wins
			record.Losses = h2hData.Losses
			record.Draws = h2hData.Draws
		}
	}

	return record, nil
}

// GetRecentMatchesVsOpponent retrieves recent matches against a specific opponent
func (r *MatchRepository) GetRecentMatchesVsOpponent(ctx context.Context, chatID, userID, opponentID int64, limit int) ([]*MatchHistoryEntry, error) {
	defer nrutil.StartSegment(ctx, "db:get-recent-matches-vs-opponent")()

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

// GetUserProfile retrieves a user's profile with rank positions
func (r *MatchRepository) GetUserProfile(ctx context.Context, chatID, userID int64) (*UserProfile, error) {
	defer nrutil.StartSegment(ctx, "db:get-user-profile")()

	query := `
		WITH ranked_lb AS (
			SELECT user_id, ROW_NUMBER() OVER (ORDER BY ranked_wins DESC, ranked_losses ASC) as rank
			FROM game_leaderboard
			WHERE chat_id = $1 AND (ranked_wins > 0 OR ranked_losses > 0)
		),
		regular_lb AS (
			SELECT user_id, ROW_NUMBER() OVER (ORDER BY regular_wins DESC, regular_losses ASC) as rank
			FROM game_leaderboard
			WHERE chat_id = $1 AND (regular_wins > 0 OR regular_losses > 0)
		)
		SELECT
			l.user_id,
			u.first_name, COALESCE(u.username, ''),
			l.ranked_wins, l.ranked_losses, l.ranked_draws,
			l.ranked_tournaments_played, l.ranked_tournaments_won,
			l.ranked_current_streak, l.ranked_best_streak,
			COALESCE(rl.rank, 0),
			l.regular_wins, l.regular_losses, l.regular_draws, l.regular_matches_played,
			l.regular_current_streak, l.regular_best_streak,
			COALESCE(rgl.rank, 0),
			l.first_match_at, l.last_match_at
		FROM game_leaderboard l
		JOIN users u ON u.id = l.user_id
		LEFT JOIN ranked_lb rl ON rl.user_id = l.user_id
		LEFT JOIN regular_lb rgl ON rgl.user_id = l.user_id
		WHERE l.chat_id = $1 AND l.user_id = $2
	`

	var profile UserProfile
	err := r.db.QueryRowContext(ctx, query, chatID, userID).Scan(
		&profile.UserID,
		&profile.FirstName, &profile.Username,
		&profile.RankedWins, &profile.RankedLosses, &profile.RankedDraws,
		&profile.RankedTournamentsPlayed, &profile.RankedTournamentsWon,
		&profile.RankedCurrentStreak, &profile.RankedBestStreak,
		&profile.RankedRank,
		&profile.RegularWins, &profile.RegularLosses, &profile.RegularDraws, &profile.RegularMatchesPlayed,
		&profile.RegularCurrentStreak, &profile.RegularBestStreak,
		&profile.RegularRank,
		&profile.FirstMatchAt, &profile.LastMatchAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user profile: %w", err)
	}

	return &profile, nil
}
