package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

// setupTestDB creates a new database connection for testing.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	host := getEnvOrDefault("TEST_DB_HOST", "localhost")
	port := getEnvOrDefault("TEST_DB_PORT", "5432")
	user := getEnvOrDefault("TEST_DB_USER", "postgres")
	password := getEnvOrDefault("TEST_DB_PASSWORD", "Vb96vpmAEZVdSpruALAgDmZrHFRDXATg")
	dbname := getEnvOrDefault("TEST_DB_NAME", "beef_briefing")
	sslmode := getEnvOrDefault("TEST_DB_SSLMODE", "disable")

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Configure connection pool for testing
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)

	// Verify connection
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Fatalf("failed to ping test database: %v", err)
	}

	return db
}

// teardownTestDB closes the test database connection.
func teardownTestDB(t *testing.T, db *sql.DB) {
	t.Helper()

	if db == nil {
		return
	}

	if err := db.Close(); err != nil {
		t.Errorf("failed to close test database: %v", err)
	}
}

// cleanupTables truncates the specified tables.
func cleanupTables(t *testing.T, db *sql.DB, tables ...string) {
	t.Helper()

	for _, table := range tables {
		query := fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)
		if _, err := db.Exec(query); err != nil {
			t.Fatalf("failed to truncate table %s: %v", table, err)
		}
	}
}

// getEnvOrDefault returns the value of an environment variable or a default value.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// insertTestChat inserts a test chat for integration tests.
func insertTestChat(t *testing.T, db *sql.DB, chatID int64) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO chats (id, type) VALUES ($1, 'supergroup')`, chatID)
	if err != nil {
		t.Fatalf("Failed to insert test chat: %v", err)
	}
}

// insertTestUser inserts a test user for integration tests.
func insertTestUser(t *testing.T, db *sql.DB, userID int64, firstName string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO users (id, first_name, is_bot) VALUES ($1, $2, false)`, userID, firstName)
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}
}

// TestCreateMatch tests creating a new match
func TestCreateMatch(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)

	repo := NewGameRepository(db, nil)

	t.Run("CreateRegularMatch", func(t *testing.T) {
		ctx := context.Background()
		chatID := int64(-1002345678901)
		creatorUserID := int64(12345)

		insertTestChat(t, db, chatID)
		insertTestUser(t, db, creatorUserID, "Creator")

		match, err := repo.CreateMatch(ctx, chatID, MatchTypeRegular, &creatorUserID, nil)
		if err != nil {
			t.Fatalf("CreateMatch failed: %v", err)
		}

		// Verify match properties
		if match.ID == "" {
			t.Error("Expected match ID to be set")
		}
		if match.ChatID != chatID {
			t.Errorf("Expected chat_id=%d, got %d", chatID, match.ChatID)
		}
		if match.MatchType != MatchTypeRegular {
			t.Errorf("Expected match_type=%s, got %s", MatchTypeRegular, match.MatchType)
		}
		if match.Status != MatchStatusOpen {
			t.Errorf("Expected status=%s, got %s", MatchStatusOpen, match.Status)
		}
		if match.CreatorUserID == nil || *match.CreatorUserID != creatorUserID {
			t.Errorf("Expected creator_user_id=%d, got %v", creatorUserID, match.CreatorUserID)
		}
		if match.CurrentRound != 0 {
			t.Errorf("Expected current_round=0, got %d", match.CurrentRound)
		}
		if match.Format != nil {
			t.Errorf("Expected format to be nil for new match, got %v", *match.Format)
		}
	})

	t.Run("CreateRankedMatch", func(t *testing.T) {
		cleanupTables(t, db, "game_match_participants", "game_matches", "users", "chats")

		ctx := context.Background()
		chatID := int64(-1002345678901)
		tournamentDate := "2026-01-16"

		insertTestChat(t, db, chatID)

		match, err := repo.CreateMatch(ctx, chatID, MatchTypeRanked, nil, &tournamentDate)
		if err != nil {
			t.Fatalf("CreateMatch failed: %v", err)
		}

		// Verify match properties
		if match.MatchType != MatchTypeRanked {
			t.Errorf("Expected match_type=%s, got %s", MatchTypeRanked, match.MatchType)
		}
		if match.TournamentDate == nil {
			t.Errorf("Expected tournament_date=%s, got nil", tournamentDate)
		} else if !strings.HasPrefix(*match.TournamentDate, tournamentDate) {
			t.Errorf("Expected tournament_date to start with %s, got %s", tournamentDate, *match.TournamentDate)
		}
		if match.CreatorUserID != nil {
			t.Errorf("Expected creator_user_id to be nil for ranked match, got %v", match.CreatorUserID)
		}
	})
}

// TestGetMatch_Found tests retrieving a match by ID
func TestGetMatch_Found(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)
	defer cleanupTables(t, db, "game_match_participants", "game_matches", "users", "chats")

	repo := NewGameRepository(db, nil)
	cleanupTables(t, db, "game_match_participants", "game_matches", "users", "chats")

	ctx := context.Background()
	chatID := int64(-1002345678901)

	insertTestChat(t, db, chatID)
	creatorID := int64(99999)
	insertTestUser(t, db, creatorID, "Creator")

	// Create a match
	created, err := repo.CreateMatch(ctx, chatID, MatchTypeRegular, &creatorID, nil)
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	// Retrieve the match
	found, err := repo.GetMatch(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetMatch failed: %v", err)
	}

	// Verify match properties
	if found.ID != created.ID {
		t.Errorf("Expected ID=%s, got %s", created.ID, found.ID)
	}
	if found.ChatID != created.ChatID {
		t.Errorf("Expected chat_id=%d, got %d", created.ChatID, found.ChatID)
	}
	if found.MatchType != created.MatchType {
		t.Errorf("Expected match_type=%s, got %s", created.MatchType, found.MatchType)
	}
	if found.Status != created.Status {
		t.Errorf("Expected status=%s, got %s", created.Status, found.Status)
	}
}

// TestGetMatch_NotFound tests retrieving a non-existent match
func TestGetMatch_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)
	defer cleanupTables(t, db, "game_match_participants", "game_matches", "users", "chats")

	repo := NewGameRepository(db, nil)
	cleanupTables(t, db, "game_match_participants", "game_matches", "users", "chats")

	ctx := context.Background()
	nonExistentID := "00000000-0000-0000-0000-000000000000"

	match, err := repo.GetMatch(ctx, nonExistentID)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if match != nil {
		t.Errorf("Expected nil match for non-existent ID, got %v", match)
	}
}

// TestUpdateMatchStatus tests updating match status
func TestUpdateMatchStatus(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)
	defer cleanupTables(t, db, "game_match_participants", "game_matches", "users", "chats")

	repo := NewGameRepository(db, nil)
	cleanupTables(t, db, "game_match_participants", "game_matches", "users", "chats")

	ctx := context.Background()
	chatID := int64(-1002345678901)

	insertTestChat(t, db, chatID)
	creatorID := int64(99999)
	insertTestUser(t, db, creatorID, "Creator")

	// Create a match
	match, err := repo.CreateMatch(ctx, chatID, MatchTypeRegular, &creatorID, nil)
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	// Update status to shop_phase
	err = repo.UpdateMatchStatus(ctx, match.ID, MatchStatusShopPhase)
	if err != nil {
		t.Fatalf("UpdateMatchStatus failed: %v", err)
	}

	// Verify status was updated
	updated, err := repo.GetMatch(ctx, match.ID)
	if err != nil {
		t.Fatalf("GetMatch failed: %v", err)
	}
	if updated.Status != MatchStatusShopPhase {
		t.Errorf("Expected status=%s, got %s", MatchStatusShopPhase, updated.Status)
	}
}

// TestAddParticipant tests adding a participant to a match
func TestAddParticipant(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)
	defer cleanupTables(t, db, "game_match_participants", "game_matches", "users", "chats")

	repo := NewGameRepository(db, nil)
	cleanupTables(t, db, "game_match_participants", "game_matches", "users", "chats")

	ctx := context.Background()
	chatID := int64(-1002345678901)
	userID := int64(12345)

	insertTestChat(t, db, chatID)
	creatorID := int64(99999)
	insertTestUser(t, db, creatorID, "Creator")
	insertTestUser(t, db, userID, "Player")

	// Create a match
	match, err := repo.CreateMatch(ctx, chatID, MatchTypeRegular, &creatorID, nil)
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	// Add participant
	participant, err := repo.AddParticipant(ctx, match.ID, userID)
	if err != nil {
		t.Fatalf("AddParticipant failed: %v", err)
	}

	// Verify participant properties
	if participant.MatchID != match.ID {
		t.Errorf("Expected match_id=%s, got %s", match.ID, participant.MatchID)
	}
	if participant.UserID != userID {
		t.Errorf("Expected user_id=%d, got %d", userID, participant.UserID)
	}
	if participant.Status != ParticipantStatusJoined {
		t.Errorf("Expected status=%s, got %s", ParticipantStatusJoined, participant.Status)
	}
	if participant.CoinsRemaining != 10 {
		t.Errorf("Expected coins_remaining=10, got %d", participant.CoinsRemaining)
	}
	if participant.Wins != 0 || participant.Losses != 0 {
		t.Errorf("Expected wins=0, losses=0, got wins=%d, losses=%d", participant.Wins, participant.Losses)
	}
}

// TestGetParticipants tests retrieving all participants for a match
func TestGetParticipants(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)
	defer cleanupTables(t, db, "game_match_participants", "game_matches", "users")

	repo := NewGameRepository(db, nil)
	cleanupTables(t, db, "game_match_participants", "game_matches", "users")

	ctx := context.Background()
	chatID := int64(-1002345678901)

	insertTestChat(t, db, chatID)
	creatorID := int64(99999)
	insertTestUser(t, db, creatorID, "Creator")
	insertTestUser(t, db, 12345, "Alice")
	insertTestUser(t, db, 67890, "Bob")
	_, err := db.Exec(`UPDATE users SET username = 'alice' WHERE id = 12345`)
	if err != nil {
		t.Fatalf("Failed to update username: %v", err)
	}
	_, err = db.Exec(`UPDATE users SET username = 'bob' WHERE id = 67890`)
	if err != nil {
		t.Fatalf("Failed to update username: %v", err)
	}

	// Create a match
	match, err := repo.CreateMatch(ctx, chatID, MatchTypeRegular, &creatorID, nil)
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	// Add participants
	_, err = repo.AddParticipant(ctx, match.ID, 12345)
	if err != nil {
		t.Fatalf("AddParticipant failed for user 12345: %v", err)
	}
	_, err = repo.AddParticipant(ctx, match.ID, 67890)
	if err != nil {
		t.Fatalf("AddParticipant failed for user 67890: %v", err)
	}

	// Get all participants
	participants, err := repo.GetMatchParticipants(ctx, match.ID)
	if err != nil {
		t.Fatalf("GetMatchParticipants failed: %v", err)
	}

	// Verify count
	if len(participants) != 2 {
		t.Fatalf("Expected 2 participants, got %d", len(participants))
	}

	// Verify participants include user info
	for _, p := range participants {
		if p.FirstName == "" {
			t.Error("Expected first_name to be populated")
		}
	}
}

// TestCreateRound tests creating a battle round
func TestCreateRound(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)
	defer cleanupTables(t, db, "game_match_rounds", "game_match_participants", "game_matches", "users", "chats")

	repo := NewGameRepository(db, nil)
	cleanupTables(t, db, "game_match_rounds", "game_match_participants", "game_matches", "users", "chats")

	ctx := context.Background()
	chatID := int64(-1002345678901)

	insertTestChat(t, db, chatID)
	creatorID := int64(99999)
	insertTestUser(t, db, creatorID, "Creator")

	// Create a match
	match, err := repo.CreateMatch(ctx, chatID, MatchTypeRegular, &creatorID, nil)
	if err != nil {
		t.Fatalf("CreateMatch failed: %v", err)
	}

	// Create test team data
	playerAID := int64(12345)
	playerBID := int64(67890)
	playerATeam := json.RawMessage(`[{"id":1,"name":"Card A","atk":10,"hp":30}]`)
	playerBTeam := json.RawMessage(`[{"id":2,"name":"Card B","atk":15,"hp":25}]`)
	battleLog := json.RawMessage(`[{"type":"attack","attacker_id":1,"target_id":2}]`)
	winnerID := playerAID

	// Create round
	round, err := repo.CreateRound(ctx, match.ID, 1, playerAID, playerBID,
		playerATeam, playerBTeam, battleLog, &winnerID, false, 15, 10, 5)
	if err != nil {
		t.Fatalf("CreateRound failed: %v", err)
	}

	// Verify round properties
	if round.MatchID != match.ID {
		t.Errorf("Expected match_id=%s, got %s", match.ID, round.MatchID)
	}
	if round.RoundNumber != 1 {
		t.Errorf("Expected round_number=1, got %d", round.RoundNumber)
	}
	if round.PlayerAID != playerAID {
		t.Errorf("Expected player_a_id=%d, got %d", playerAID, round.PlayerAID)
	}
	if round.PlayerBID != playerBID {
		t.Errorf("Expected player_b_id=%d, got %d", playerBID, round.PlayerBID)
	}
	if round.WinnerID == nil || *round.WinnerID != winnerID {
		t.Errorf("Expected winner_id=%d, got %v", winnerID, round.WinnerID)
	}
	if round.IsDraw {
		t.Error("Expected is_draw=false, got true")
	}
	if round.PlayerADmg != 15 {
		t.Errorf("Expected player_a_damage=15, got %d", round.PlayerADmg)
	}
	if round.PlayerBDmg != 10 {
		t.Errorf("Expected player_b_damage=10, got %d", round.PlayerBDmg)
	}
	if round.TotalRounds != 5 {
		t.Errorf("Expected total_rounds=5, got %d", round.TotalRounds)
	}
}

// TestUpdateLeaderboard tests updating leaderboard stats
func TestUpdateLeaderboard(t *testing.T) {
	db := setupTestDB(t)
	defer teardownTestDB(t, db)
	defer cleanupTables(t, db, "game_leaderboard")

	repo := NewGameRepository(db, nil)
	cleanupTables(t, db, "game_leaderboard")

	ctx := context.Background()
	userID := int64(12345)
	chatID := int64(-1002345678901)
	opponentID := int64(67890)

	// Record a win
	err := repo.UpdateLeaderboard(ctx, userID, chatID, MatchTypeRanked, true, &opponentID, false, false)
	if err != nil {
		t.Fatalf("UpdateLeaderboard failed: %v", err)
	}

	// Verify leaderboard entry was created
	var rankedWins, rankedCurrentStreak int
	var h2h pq.Int64Array
	err = db.QueryRow(`
		SELECT ranked_wins, ranked_current_streak, head_to_head
		FROM game_leaderboard
		WHERE user_id = $1 AND chat_id = $2
	`, userID, chatID).Scan(&rankedWins, &rankedCurrentStreak, &h2h)
	if err != nil {
		t.Fatalf("Failed to query leaderboard: %v", err)
	}

	if rankedWins != 1 {
		t.Errorf("Expected ranked_wins=1, got %d", rankedWins)
	}
	if rankedCurrentStreak != 1 {
		t.Errorf("Expected ranked_current_streak=1, got %d", rankedCurrentStreak)
	}

	// Record a loss
	err = repo.UpdateLeaderboard(ctx, userID, chatID, MatchTypeRanked, false, &opponentID, false, false)
	if err != nil {
		t.Fatalf("UpdateLeaderboard failed on loss: %v", err)
	}

	// Verify streak was reset
	err = db.QueryRow(`
		SELECT ranked_wins, ranked_losses, ranked_current_streak
		FROM game_leaderboard
		WHERE user_id = $1 AND chat_id = $2
	`, userID, chatID).Scan(&rankedWins, new(int), &rankedCurrentStreak)
	if err != nil {
		t.Fatalf("Failed to query leaderboard after loss: %v", err)
	}

	if rankedWins != 1 {
		t.Errorf("Expected ranked_wins=1 after loss, got %d", rankedWins)
	}
	if rankedCurrentStreak != 0 {
		t.Errorf("Expected ranked_current_streak=0 after loss, got %d", rankedCurrentStreak)
	}
}
