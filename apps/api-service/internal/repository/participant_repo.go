package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"beef-briefing/apps/api-service/internal/nrutil"

	"github.com/lib/pq"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// ParticipantRepository handles database operations for match participants
type ParticipantRepository struct {
	db    *sql.DB
	nrApp *newrelic.Application
}

// NewParticipantRepository creates a new ParticipantRepository
func NewParticipantRepository(db *sql.DB, nrApp *newrelic.Application) *ParticipantRepository {
	return &ParticipantRepository{db: db, nrApp: nrApp}
}

// startDBSegment starts a NewRelic segment for database operations and returns a cleanup function.
func (r *ParticipantRepository) startDBSegment(ctx context.Context, name string) func() {
	txn := newrelic.FromContext(ctx)
	if txn != nil {
		segment := txn.StartSegment("db:" + name)
		return segment.End
	}
	return func() {}
}

// AddParticipant adds a user to a match
func (r *ParticipantRepository) AddParticipant(ctx context.Context, matchID string, userID int64) (*Participant, error) {
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
func (r *ParticipantRepository) GetParticipant(ctx context.Context, matchID string, userID int64) (*Participant, error) {
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
func (r *ParticipantRepository) GetMatchParticipants(ctx context.Context, matchID string) ([]*ParticipantWithUser, error) {
	defer nrutil.StartSegment(ctx, "db:get-match-participants")()

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
func (r *ParticipantRepository) RemoveParticipant(ctx context.Context, matchID string, userID int64) error {
	defer nrutil.StartSegment(ctx, "db:remove-participant")()

	query := `DELETE FROM game_match_participants WHERE match_id = $1 AND user_id = $2`
	_, err := r.db.ExecContext(ctx, query, matchID, userID)
	return err
}

// UpdateParticipantShop updates a participant's shop state
func (r *ParticipantRepository) UpdateParticipantShop(ctx context.Context, matchID string, userID int64, coins int, shopCards, team json.RawMessage, teamOrder []int64) error {
	defer nrutil.StartSegment(ctx, "db:update-participant-shop")()

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
func (r *ParticipantRepository) SubmitTeam(ctx context.Context, matchID string, userID int64) error {
	defer nrutil.StartSegment(ctx, "db:submit-team")()

	query := `
		UPDATE game_match_participants
		SET status = 'ready',
		    team_submitted_at = NOW()
		WHERE match_id = $1 AND user_id = $2
	`
	_, err := r.db.ExecContext(ctx, query, matchID, userID)
	return err
}

// GetParticipantCount returns the number of participants in a match
func (r *ParticipantRepository) GetParticipantCount(ctx context.Context, matchID string) (int, error) {
	defer nrutil.StartSegment(ctx, "db:count-participants")()

	var count int
	query := `SELECT COUNT(*) FROM game_match_participants WHERE match_id = $1`
	err := r.db.QueryRowContext(ctx, query, matchID).Scan(&count)
	return count, err
}

// GetReadyParticipantCount returns number of ready participants
func (r *ParticipantRepository) GetReadyParticipantCount(ctx context.Context, matchID string) (int, error) {
	defer nrutil.StartSegment(ctx, "db:count-ready-participants")()

	var count int
	query := `SELECT COUNT(*) FROM game_match_participants WHERE match_id = $1 AND status = 'ready'`
	err := r.db.QueryRowContext(ctx, query, matchID).Scan(&count)
	return count, err
}
