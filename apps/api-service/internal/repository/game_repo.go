package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/lib/pq"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// GameRepository is a facade that coordinates match, participant, and tournament repositories.
// It provides a unified API for backward compatibility while delegating to specialized repositories.
type GameRepository struct {
	db    *sql.DB
	nrApp *newrelic.Application

	// Specialized repositories
	matchRepo       *MatchRepository
	participantRepo *ParticipantRepository
	tournamentRepo  *TournamentRepository
}

// NewGameRepository creates a new GameRepository that delegates to specialized repos
func NewGameRepository(db *sql.DB, nrApp *newrelic.Application) *GameRepository {
	return &GameRepository{
		db:              db,
		nrApp:           nrApp,
		matchRepo:       NewMatchRepository(db, nrApp),
		participantRepo: NewParticipantRepository(db, nrApp),
		tournamentRepo:  NewTournamentRepository(db, nrApp),
	}
}

// ============================================================================
// TYPES AND ENUMS
// ============================================================================

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
       placement, wins, losses, total_damage_dealt, has_rerolled`

// scanParticipant scans a row into a Participant struct
func scanParticipant(row rowScanner) (*Participant, error) {
	p := &Participant{}
	err := row.Scan(
		&p.ID, &p.MatchID, &p.UserID, &p.Status, &p.JoinedAt,
		&p.CoinsRemaining, &p.ShopCards, &p.Team, &p.TeamOrder,
		&p.TeamSubmittedAt, &p.Placement, &p.Wins, &p.Losses, &p.TotalDamageDealt,
		&p.HasRerolled,
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

// TournamentStatus enum
type TournamentStatus string

const (
	TournamentStatusScheduled  TournamentStatus = "scheduled"
	TournamentStatusOpen       TournamentStatus = "open"
	TournamentStatusInProgress TournamentStatus = "in_progress"
	TournamentStatusCompleted  TournamentStatus = "completed"
	TournamentStatusSkipped    TournamentStatus = "skipped"
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
	HasRerolled      bool              `json:"has_rerolled"`
}

// ParticipantWithUser includes user info
type ParticipantWithUser struct {
	Participant
	FirstName      string  `json:"first_name"`
	Username       string  `json:"username,omitempty"`
	PhotoObjectKey *string `json:"-"`                      // Internal: minio object key (not serialized)
	PhotoURL       *string `json:"photo_url,omitempty"`    // Presigned URL for profile photo
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
	RankedDraws             int             `json:"ranked_draws"`
	RankedTournamentsPlayed int             `json:"ranked_tournaments_played"`
	RankedTournamentsWon    int             `json:"ranked_tournaments_won"`
	RankedCurrentStreak     int             `json:"ranked_current_streak"`
	RankedBestStreak        int             `json:"ranked_best_streak"`
	RegularWins             int             `json:"regular_wins"`
	RegularLosses           int             `json:"regular_losses"`
	RegularDraws            int             `json:"regular_draws"`
	RegularMatchesPlayed    int             `json:"regular_matches_played"`
	RegularCurrentStreak    int             `json:"regular_current_streak"`
	RegularBestStreak       int             `json:"regular_best_streak"`
	HeadToHead              json.RawMessage `json:"head_to_head"`
	FirstMatchAt            *time.Time      `json:"first_match_at,omitempty"`
	LastMatchAt             *time.Time      `json:"last_match_at,omitempty"`
	// Ranking fields
	Rank  int     `json:"rank"`  // 1-indexed position in leaderboard
	Score float64 `json:"score"` // Wilson Score for regular, tournaments_won for ranked
	// User info from join
	FirstName      string  `json:"first_name,omitempty"`
	Username       string  `json:"username,omitempty"`
	PhotoObjectKey *string `json:"-"`                   // Internal: minio object key (not serialized)
	PhotoURL       *string `json:"photo_url,omitempty"` // Presigned URL for profile photo
}

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

// MatchHistoryEntry represents a match in the user's history
type MatchHistoryEntry struct {
	MatchID          string    `json:"match_id"`
	MatchType        MatchType `json:"match_type"`
	OpponentID       int64     `json:"opponent_id"`
	OpponentName     string    `json:"opponent_name"`
	OpponentUser     string    `json:"opponent_username,omitempty"`
	Result           string    `json:"result"` // "win", "loss", "draw"
	YourTeam         []byte    `json:"your_team"`
	OpponentTeam     []byte    `json:"opponent_team"`
	CompletedAt      time.Time `json:"completed_at"`
	YourPhotoKey     *string   `json:"-"` // minio object key for your photo
	OpponentPhotoKey *string   `json:"-"` // minio object key for opponent photo
}

// H2HRecord represents head-to-head record against an opponent
type H2HRecord struct {
	OpponentID   int64      `json:"opponent_id"`
	OpponentName string     `json:"opponent_name"`
	OpponentUser string     `json:"opponent_username,omitempty"`
	Wins         int        `json:"wins"`
	Losses       int        `json:"losses"`
	Draws        int        `json:"draws"`
	LastMatchAt  *time.Time `json:"last_match_at,omitempty"`
}

// UserProfile represents a user's arena profile with rank positions
type UserProfile struct {
	UserID                  int64      `json:"user_id"`
	FirstName               string     `json:"first_name"`
	Username                string     `json:"username,omitempty"`
	RankedWins              int        `json:"ranked_wins"`
	RankedLosses            int        `json:"ranked_losses"`
	RankedDraws             int        `json:"ranked_draws"`
	RankedTournamentsPlayed int        `json:"ranked_tournaments_played"`
	RankedTournamentsWon    int        `json:"ranked_tournaments_won"`
	RankedCurrentStreak     int        `json:"ranked_current_streak"`
	RankedBestStreak        int        `json:"ranked_best_streak"`
	RankedRank              int        `json:"ranked_rank"`
	RegularWins             int        `json:"regular_wins"`
	RegularLosses           int        `json:"regular_losses"`
	RegularDraws            int        `json:"regular_draws"`
	RegularMatchesPlayed    int        `json:"regular_matches_played"`
	RegularCurrentStreak    int        `json:"regular_current_streak"`
	RegularBestStreak       int        `json:"regular_best_streak"`
	RegularRank             int        `json:"regular_rank"`
	FirstMatchAt            *time.Time `json:"first_match_at,omitempty"`
	LastMatchAt             *time.Time `json:"last_match_at,omitempty"`
	PhotoObjectKey          *string    `json:"-"`                        // Internal: minio object key (not serialized)
	PhotoURL                *string    `json:"photo_url,omitempty"`      // Presigned URL for profile photo
}

// ============================================================================
// MATCH OPERATIONS (delegated to MatchRepository)
// ============================================================================

func (r *GameRepository) CreateMatch(ctx context.Context, chatID int64, matchType MatchType, creatorUserID *int64, tournamentDate *string) (*Match, error) {
	return r.matchRepo.CreateMatch(ctx, chatID, matchType, creatorUserID, tournamentDate)
}

func (r *GameRepository) GetMatch(ctx context.Context, matchID string) (*Match, error) {
	return r.matchRepo.GetMatch(ctx, matchID)
}

func (r *GameRepository) GetActiveMatches(ctx context.Context, chatID int64) ([]*Match, error) {
	return r.matchRepo.GetActiveMatches(ctx, chatID)
}

func (r *GameRepository) UpdateMatchStatus(ctx context.Context, matchID string, status MatchStatus) error {
	return r.matchRepo.UpdateMatchStatus(ctx, matchID, status)
}

func (r *GameRepository) UpdateMatchFormat(ctx context.Context, matchID string, format MatchFormat) error {
	return r.matchRepo.UpdateMatchFormat(ctx, matchID, format)
}

func (r *GameRepository) StartShopPhase(ctx context.Context, matchID string, format MatchFormat, deadline time.Time) error {
	return r.matchRepo.StartShopPhase(ctx, matchID, format, deadline)
}

func (r *GameRepository) StartBattlePhase(ctx context.Context, matchID string) error {
	return r.matchRepo.StartBattlePhase(ctx, matchID)
}

func (r *GameRepository) CompleteMatch(ctx context.Context, matchID string, winnerUserID *int64) error {
	return r.matchRepo.CompleteMatch(ctx, matchID, winnerUserID)
}

func (r *GameRepository) CancelMatch(ctx context.Context, matchID string) error {
	return r.matchRepo.CancelMatch(ctx, matchID)
}

func (r *GameRepository) GetMatchesByStatus(ctx context.Context, status MatchStatus) ([]*Match, error) {
	return r.matchRepo.GetMatchesByStatus(ctx, status)
}

func (r *GameRepository) CreateRound(ctx context.Context, matchID string, roundNumber int, playerAID, playerBID int64, playerATeam, playerBTeam, battleLog json.RawMessage, winnerID *int64, isDraw bool, playerADmg, playerBDmg, totalRounds int) (*MatchRound, error) {
	return r.matchRepo.CreateRound(ctx, matchID, roundNumber, playerAID, playerBID, playerATeam, playerBTeam, battleLog, winnerID, isDraw, playerADmg, playerBDmg, totalRounds)
}

func (r *GameRepository) GetMatchRounds(ctx context.Context, matchID string) ([]*MatchRound, error) {
	return r.matchRepo.GetMatchRounds(ctx, matchID)
}

func (r *GameRepository) GetLeaderboard(ctx context.Context, chatID int64, matchType MatchType, limit, offset int) ([]*LeaderboardEntry, int, error) {
	return r.matchRepo.GetLeaderboard(ctx, chatID, matchType, limit, offset)
}

func (r *GameRepository) UpdateLeaderboard(ctx context.Context, userID, chatID int64, matchType MatchType, isWin bool, opponentID *int64, isTournamentWin bool, isDraw bool) error {
	return r.matchRepo.UpdateLeaderboard(ctx, userID, chatID, matchType, isWin, opponentID, isTournamentWin, isDraw)
}

func (r *GameRepository) GetMatchHistory(ctx context.Context, chatID, userID int64, limit, offset int) ([]*MatchHistoryEntry, int, error) {
	return r.matchRepo.GetMatchHistory(ctx, chatID, userID, limit, offset)
}

func (r *GameRepository) GetH2HRecord(ctx context.Context, chatID, userID, opponentID int64) (*H2HRecord, error) {
	return r.matchRepo.GetH2HRecord(ctx, chatID, userID, opponentID)
}

func (r *GameRepository) GetRecentMatchesVsOpponent(ctx context.Context, chatID, userID, opponentID int64, limit int) ([]*MatchHistoryEntry, error) {
	return r.matchRepo.GetRecentMatchesVsOpponent(ctx, chatID, userID, opponentID, limit)
}

func (r *GameRepository) GetUserProfile(ctx context.Context, chatID, userID int64) (*UserProfile, error) {
	return r.matchRepo.GetUserProfile(ctx, chatID, userID)
}

// ============================================================================
// PARTICIPANT OPERATIONS (delegated to ParticipantRepository)
// ============================================================================

func (r *GameRepository) AddParticipant(ctx context.Context, matchID string, userID int64) (*Participant, error) {
	return r.participantRepo.AddParticipant(ctx, matchID, userID)
}

func (r *GameRepository) GetParticipant(ctx context.Context, matchID string, userID int64) (*Participant, error) {
	return r.participantRepo.GetParticipant(ctx, matchID, userID)
}

func (r *GameRepository) GetMatchParticipants(ctx context.Context, matchID string) ([]*ParticipantWithUser, error) {
	return r.participantRepo.GetMatchParticipants(ctx, matchID)
}

func (r *GameRepository) RemoveParticipant(ctx context.Context, matchID string, userID int64) error {
	return r.participantRepo.RemoveParticipant(ctx, matchID, userID)
}

func (r *GameRepository) UpdateParticipantShop(ctx context.Context, matchID string, userID int64, coins int, shopCards, team json.RawMessage, teamOrder []int64) error {
	return r.participantRepo.UpdateParticipantShop(ctx, matchID, userID, coins, shopCards, team, teamOrder)
}

func (r *GameRepository) SubmitTeam(ctx context.Context, matchID string, userID int64) error {
	return r.participantRepo.SubmitTeam(ctx, matchID, userID)
}

func (r *GameRepository) GetParticipantCount(ctx context.Context, matchID string) (int, error) {
	return r.participantRepo.GetParticipantCount(ctx, matchID)
}

func (r *GameRepository) GetReadyParticipantCount(ctx context.Context, matchID string) (int, error) {
	return r.participantRepo.GetReadyParticipantCount(ctx, matchID)
}

// ============================================================================
// TOURNAMENT OPERATIONS (delegated to TournamentRepository)
// ============================================================================

func (r *GameRepository) GetOrCreateTournament(ctx context.Context, chatID int64, date string) (*RankedTournament, error) {
	return r.tournamentRepo.GetOrCreateTournament(ctx, chatID, date)
}

func (r *GameRepository) GetTournamentByID(ctx context.Context, id int64) (*RankedTournament, error) {
	return r.tournamentRepo.GetTournamentByID(ctx, id)
}

func (r *GameRepository) GetTodayTournament(ctx context.Context, chatID int64, date string) (*RankedTournament, error) {
	return r.tournamentRepo.GetTodayTournament(ctx, chatID, date)
}

func (r *GameRepository) UpdateTournamentStatus(ctx context.Context, id int64, status TournamentStatus) error {
	return r.tournamentRepo.UpdateTournamentStatus(ctx, id, status)
}

func (r *GameRepository) SetTournamentAnnounced(ctx context.Context, id int64, messageID int64) error {
	return r.tournamentRepo.SetTournamentAnnounced(ctx, id, messageID)
}

func (r *GameRepository) CloseTournamentRegistration(ctx context.Context, id int64, matchID string) error {
	return r.tournamentRepo.CloseTournamentRegistration(ctx, id, matchID)
}

func (r *GameRepository) CompleteTournament(ctx context.Context, id int64, winnerUserID *int64) error {
	return r.tournamentRepo.CompleteTournament(ctx, id, winnerUserID)
}

func (r *GameRepository) SkipTournament(ctx context.Context, id int64) error {
	return r.tournamentRepo.SkipTournament(ctx, id)
}

func (r *GameRepository) UpdateTournamentBracket(ctx context.Context, id int64, bracketState json.RawMessage) error {
	return r.tournamentRepo.UpdateTournamentBracket(ctx, id, bracketState)
}

func (r *GameRepository) AddTournamentParticipant(ctx context.Context, tournamentID, userID int64) error {
	return r.tournamentRepo.AddTournamentParticipant(ctx, tournamentID, userID)
}

func (r *GameRepository) RemoveTournamentParticipant(ctx context.Context, tournamentID, userID int64) error {
	return r.tournamentRepo.RemoveTournamentParticipant(ctx, tournamentID, userID)
}

func (r *GameRepository) GetTournamentParticipants(ctx context.Context, tournamentID int64) ([]*TournamentParticipant, error) {
	return r.tournamentRepo.GetTournamentParticipants(ctx, tournamentID)
}

func (r *GameRepository) IsTournamentParticipant(ctx context.Context, tournamentID, userID int64) (bool, error) {
	return r.tournamentRepo.IsTournamentParticipant(ctx, tournamentID, userID)
}

func (r *GameRepository) GetTournamentsNeedingAnnouncement(ctx context.Context, currentTime time.Time) ([]*TournamentInfo, error) {
	return r.tournamentRepo.GetTournamentsNeedingAnnouncement(ctx, currentTime)
}

func (r *GameRepository) GetTournamentsNeedingClose(ctx context.Context, currentTime time.Time) ([]*TournamentInfo, error) {
	return r.tournamentRepo.GetTournamentsNeedingClose(ctx, currentTime)
}

func (r *GameRepository) GetChatsWithTimezone(ctx context.Context) ([]*ChatTimezone, error) {
	return r.tournamentRepo.GetChatsWithTimezone(ctx)
}

func (r *GameRepository) GetTournamentsByStatus(ctx context.Context, status TournamentStatus) ([]*RankedTournament, error) {
	return r.tournamentRepo.GetTournamentsByStatus(ctx, status)
}

func (r *GameRepository) GetTournamentsWithPendingRounds(ctx context.Context) ([]*RankedTournament, error) {
	return r.tournamentRepo.GetTournamentsWithPendingRounds(ctx)
}
