package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"beef-briefing/apps/api-service/internal/nrutil"

	"github.com/newrelic/go-agent/v3/newrelic"
)

// TournamentRepository handles database operations for ranked tournaments
type TournamentRepository struct {
	db    *sql.DB
	nrApp *newrelic.Application
}

// NewTournamentRepository creates a new TournamentRepository
func NewTournamentRepository(db *sql.DB, nrApp *newrelic.Application) *TournamentRepository {
	return &TournamentRepository{db: db, nrApp: nrApp}
}

// startDBSegment starts a NewRelic segment for database operations and returns a cleanup function.
func (r *TournamentRepository) startDBSegment(ctx context.Context, name string) func() {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:" + name)
		return segment.End
	}
	return func() {}
}

// GetOrCreateTournament creates or retrieves a tournament for a specific date
func (r *TournamentRepository) GetOrCreateTournament(ctx context.Context, chatID int64, date string) (*RankedTournament, error) {
	defer nrutil.StartSegment(ctx, "db:get-or-create-tournament")()

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
func (r *TournamentRepository) GetTournamentByID(ctx context.Context, id int64) (*RankedTournament, error) {
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
func (r *TournamentRepository) GetTodayTournament(ctx context.Context, chatID int64, date string) (*RankedTournament, error) {
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
func (r *TournamentRepository) UpdateTournamentStatus(ctx context.Context, id int64, status TournamentStatus) error {
	defer nrutil.StartSegment(ctx, "db:update-tournament-status")()

	query := `UPDATE game_ranked_tournaments SET status = $2 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id, status)
	return err
}

// SetTournamentAnnounced marks a tournament as announced
func (r *TournamentRepository) SetTournamentAnnounced(ctx context.Context, id int64, messageID int64) error {
	defer nrutil.StartSegment(ctx, "db:set-tournament-announced")()

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
func (r *TournamentRepository) CloseTournamentRegistration(ctx context.Context, id int64, matchID string) error {
	defer nrutil.StartSegment(ctx, "db:close-tournament-registration")()

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
func (r *TournamentRepository) CompleteTournament(ctx context.Context, id int64, winnerUserID *int64) error {
	defer nrutil.StartSegment(ctx, "db:complete-tournament")()

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
func (r *TournamentRepository) SkipTournament(ctx context.Context, id int64) error {
	defer nrutil.StartSegment(ctx, "db:skip-tournament")()

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
func (r *TournamentRepository) UpdateTournamentBracket(ctx context.Context, id int64, bracketState json.RawMessage) error {
	defer nrutil.StartSegment(ctx, "db:update-tournament-bracket")()

	query := `UPDATE game_ranked_tournaments SET bracket_state = $2 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id, bracketState)
	return err
}

// AddTournamentParticipant adds a user to a tournament
func (r *TournamentRepository) AddTournamentParticipant(ctx context.Context, tournamentID, userID int64) error {
	defer nrutil.StartSegment(ctx, "db:add-tournament-participant")()

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
func (r *TournamentRepository) RemoveTournamentParticipant(ctx context.Context, tournamentID, userID int64) error {
	defer nrutil.StartSegment(ctx, "db:remove-tournament-participant")()

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
func (r *TournamentRepository) GetTournamentParticipants(ctx context.Context, tournamentID int64) ([]*TournamentParticipant, error) {
	defer nrutil.StartSegment(ctx, "db:get-tournament-participants")()

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
func (r *TournamentRepository) IsTournamentParticipant(ctx context.Context, tournamentID, userID int64) (bool, error) {
	defer nrutil.StartSegment(ctx, "db:is-tournament-participant")()

	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM game_tournament_participants WHERE tournament_id = $1 AND user_id = $2)`
	err := r.db.QueryRowContext(ctx, query, tournamentID, userID).Scan(&exists)
	return exists, err
}

// GetTournamentsNeedingAnnouncement returns tournaments that should be announced
func (r *TournamentRepository) GetTournamentsNeedingAnnouncement(ctx context.Context, currentTime time.Time) ([]*TournamentInfo, error) {
	defer nrutil.StartSegment(ctx, "db:get-tournaments-needing-announcement")()

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
func (r *TournamentRepository) GetTournamentsNeedingClose(ctx context.Context, currentTime time.Time) ([]*TournamentInfo, error) {
	defer nrutil.StartSegment(ctx, "db:get-tournaments-needing-close")()

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
func (r *TournamentRepository) GetChatsWithTimezone(ctx context.Context) ([]*ChatTimezone, error) {
	defer nrutil.StartSegment(ctx, "db:get-chats-with-timezone")()

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
func (r *TournamentRepository) GetTournamentsByStatus(ctx context.Context, status TournamentStatus) ([]*RankedTournament, error) {
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
func (r *TournamentRepository) GetTournamentsWithPendingRounds(ctx context.Context) ([]*RankedTournament, error) {
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
