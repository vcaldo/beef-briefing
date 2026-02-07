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

// GetUserActiveMatch retrieves a user's active match in a specific chat
func (r *MatchRepository) GetUserActiveMatch(ctx context.Context, chatID, userID int64) (*Match, error) {
	defer r.startDBSegment(ctx, "get-user-active-match")()

	// Use matchColumnsAliased to avoid ambiguous column references in JOIN
	query := fmt.Sprintf(`
		SELECT %s
		FROM game_matches m
		INNER JOIN game_match_participants p ON m.id = p.match_id
		WHERE m.chat_id = $1 AND p.user_id = $2 AND m.status NOT IN ('completed', 'cancelled')
		ORDER BY m.created_at DESC
		LIMIT 1
	`, matchColumnsAliased)

	match, err := scanMatch(r.db.QueryRowContext(ctx, query, chatID, userID))
	if err == sql.ErrNoRows {
		return nil, nil // No active match found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query user active match: %w", err)
	}

	return match, nil
}

// GetChatOpenMatch retrieves an open regular match for a chat
func (r *MatchRepository) GetChatOpenMatch(ctx context.Context, chatID int64) (*Match, error) {
	defer r.startDBSegment(ctx, "get-chat-open-match")()

	query := fmt.Sprintf(`
		SELECT %s
		FROM game_matches
		WHERE chat_id = $1
		  AND status = 'open'
		  AND match_type = 'regular'
		ORDER BY created_at DESC
		LIMIT 1
	`, matchColumns)

	match, err := scanMatch(r.db.QueryRowContext(ctx, query, chatID))
	if err == sql.ErrNoRows {
		return nil, nil // No open match found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query chat open match: %w", err)
	}

	return match, nil
}

// SetTelegramMessageID updates the telegram_message_id for a match
func (r *MatchRepository) SetTelegramMessageID(ctx context.Context, matchID string, messageID int64) error {
	defer nrutil.StartSegment(ctx, "db:set-telegram-message-id")()

	query := `UPDATE game_matches SET telegram_message_id = $2 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, matchID, messageID)
	return err
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

// GetLeaderboard retrieves leaderboard entries for a chat with ranking
func (r *MatchRepository) GetLeaderboard(ctx context.Context, chatID int64, matchType MatchType, limit, offset int) ([]*LeaderboardEntry, int, error) {
	defer nrutil.StartSegment(ctx, "db:get-leaderboard")()

	// Get total count
	countQuery := `SELECT COUNT(*) FROM game_leaderboard WHERE chat_id = $1`
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, chatID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count leaderboard: %w", err)
	}

	// Determine ordering and score expression based on match type
	var orderBy, scoreExpr string
	if matchType == MatchTypeRanked {
		// Ranked: Order by tournaments won, then wins, then fewest losses
		orderBy = "ranked_tournaments_won DESC, ranked_wins DESC, ranked_losses ASC"
		scoreExpr = "ranked_tournaments_won::FLOAT"
	} else {
		// Regular: Order by Wilson Score
		orderBy = "wilson_score_lower_bound(regular_wins, regular_losses) DESC, regular_wins DESC, regular_losses ASC"
		scoreExpr = "wilson_score_lower_bound(regular_wins, regular_losses)"
	}

	query := fmt.Sprintf(`
		WITH ranked_entries AS (
			SELECT l.user_id, l.chat_id,
			       l.ranked_wins, l.ranked_losses, l.ranked_draws, l.ranked_tournaments_played, l.ranked_tournaments_won,
			       l.ranked_current_streak, l.ranked_best_streak,
			       l.regular_wins, l.regular_losses, l.regular_draws, l.regular_matches_played,
			       l.regular_current_streak, l.regular_best_streak,
			       l.head_to_head, l.first_match_at, l.last_match_at,
			       u.first_name, COALESCE(u.username, '') as username,
			       (SELECT minio_object_key FROM user_profile_photos WHERE user_id = l.user_id ORDER BY width DESC LIMIT 1) as photo_key,
			       %s as score,
			       ROW_NUMBER() OVER (ORDER BY %s) as rank
			FROM game_leaderboard l
			JOIN users u ON l.user_id = u.id
			WHERE l.chat_id = $1
		)
		SELECT user_id, chat_id,
		       ranked_wins, ranked_losses, ranked_draws, ranked_tournaments_played, ranked_tournaments_won,
		       ranked_current_streak, ranked_best_streak,
		       regular_wins, regular_losses, regular_draws, regular_matches_played,
		       regular_current_streak, regular_best_streak,
		       head_to_head, first_match_at, last_match_at,
		       first_name, username, photo_key, rank, score
		FROM ranked_entries
		ORDER BY rank
		LIMIT $2 OFFSET $3
	`, scoreExpr, orderBy)

	rows, err := r.db.QueryContext(ctx, query, chatID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query leaderboard: %w", err)
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
			&e.FirstName, &e.Username, &e.PhotoObjectKey, &e.Rank, &e.Score,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan leaderboard row: %w", err)
		}
		entries = append(entries, e)
	}

	return entries, total, rows.Err()
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

	// Count total matches (one row per match via game_match_participants)
	var total int
	countQuery := `
		SELECT COUNT(*)
		FROM game_matches m
		JOIN game_match_participants p ON p.match_id = m.id AND p.user_id = $2
		WHERE m.chat_id = $1
		  AND m.status = 'completed'
	`
	if err := r.db.QueryRowContext(ctx, countQuery, chatID, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count match history: %w", err)
	}

	// Get match history — one row per match.
	// Uses game_matches as primary table to avoid duplicates from multi-round formats.
	// A lateral join fetches one representative round for opponent/team data.
	query := `
		SELECT
			m.id, m.match_type, m.completed_at,
			COALESCE(m.format::text, '1v1') as format,
			pcnt.player_count,
			r.opponent_id,
			u.first_name as opponent_name, COALESCE(u.username, '') as opponent_username,
			CASE
				WHEN m.winner_user_id IS NULL THEN 'draw'
				WHEN m.winner_user_id = $2 THEN 'win'
				ELSE 'loss'
			END as result,
			r.your_team,
			r.opponent_team,
			your_photo.minio_object_key as your_photo_key,
			opp_photo.minio_object_key as opponent_photo_key
		FROM game_matches m
		JOIN game_match_participants p ON p.match_id = m.id AND p.user_id = $2
		-- Count participants per match
		JOIN LATERAL (
			SELECT COUNT(*)::int as player_count
			FROM game_match_participants
			WHERE match_id = m.id
		) pcnt ON true
		-- Get one representative round for opponent/team info
		JOIN LATERAL (
			SELECT
				CASE WHEN rd.player_a_id = $2 THEN rd.player_b_id ELSE rd.player_a_id END as opponent_id,
				CASE WHEN rd.player_a_id = $2 THEN rd.player_a_team ELSE rd.player_b_team END as your_team,
				CASE WHEN rd.player_a_id = $2 THEN rd.player_b_team ELSE rd.player_a_team END as opponent_team
			FROM game_match_rounds rd
			WHERE rd.match_id = m.id
			  AND (rd.player_a_id = $2 OR rd.player_b_id = $2)
			ORDER BY rd.round_number ASC
			LIMIT 1
		) r ON true
		JOIN users u ON u.id = r.opponent_id
		LEFT JOIN LATERAL (
			SELECT minio_object_key FROM user_profile_photos
			WHERE user_id = $2 ORDER BY width DESC LIMIT 1
		) your_photo ON true
		LEFT JOIN LATERAL (
			SELECT minio_object_key FROM user_profile_photos
			WHERE user_id = r.opponent_id
			ORDER BY width DESC LIMIT 1
		) opp_photo ON true
		WHERE m.chat_id = $1
		  AND m.status = 'completed'
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
			&e.Format, &e.PlayerCount,
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
			COALESCE(m.format::text, '1v1') as format,
			pcnt.player_count,
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
		JOIN LATERAL (
			SELECT COUNT(*)::int as player_count
			FROM game_match_participants
			WHERE match_id = m.id
		) pcnt ON true
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
			&e.Format, &e.PlayerCount,
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
			l.first_match_at, l.last_match_at,
			(SELECT minio_object_key FROM user_profile_photos WHERE user_id = l.user_id ORDER BY width DESC LIMIT 1) as photo_key
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
		&profile.PhotoObjectKey,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user profile: %w", err)
	}

	return &profile, nil
}
