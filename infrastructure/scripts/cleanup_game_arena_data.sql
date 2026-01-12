-- =====================================================
-- CLEANUP SCRIPT: Game Arena Data Reset
-- =====================================================
-- This script drops all game arena tables, enums, functions, and migration records
-- Use this BEFORE deploying the consolidated 009_game_arena.sql migration
--
-- Prerequisites:
--   - make pg-tunnel must be running in another terminal
--   - Connect via: psql "postgresql://postgres@localhost:5433/beef?sslmode=disable"
--
-- Usage:
--   psql "postgresql://postgres@localhost:5433/beef?sslmode=disable" < cleanup_game_arena_data.sql

BEGIN;

-- =====================================================
-- DROP FUNCTIONS FIRST (before types they depend on)
-- =====================================================

DROP FUNCTION IF EXISTS update_game_leaderboard(BIGINT, BIGINT, game_match_type, BOOLEAN, BIGINT, BOOLEAN) CASCADE;
DROP FUNCTION IF EXISTS get_tournaments_needing_announcement(TIMESTAMPTZ) CASCADE;
DROP FUNCTION IF EXISTS get_tournaments_needing_close(TIMESTAMPTZ) CASCADE;
DROP FUNCTION IF EXISTS get_or_create_tournament(BIGINT, DATE) CASCADE;

-- =====================================================
-- DROP TABLES (in dependency order)
-- =====================================================

DROP TABLE IF EXISTS game_tournament_participants CASCADE;
DROP TABLE IF EXISTS game_ranked_tournaments CASCADE;
DROP TABLE IF EXISTS game_match_rounds CASCADE;
DROP TABLE IF EXISTS game_match_participants CASCADE;
DROP TABLE IF EXISTS game_leaderboard CASCADE;
DROP TABLE IF EXISTS game_matches CASCADE;

-- =====================================================
-- DROP ENUMS (after functions that depend on them)
-- =====================================================

DROP TYPE IF EXISTS game_tournament_status CASCADE;
DROP TYPE IF EXISTS game_participant_status CASCADE;
DROP TYPE IF EXISTS game_match_status CASCADE;
DROP TYPE IF EXISTS game_match_format CASCADE;
DROP TYPE IF EXISTS game_match_type CASCADE;

-- =====================================================
-- DROP COLUMN FROM CHATS
-- =====================================================

ALTER TABLE chats DROP COLUMN IF EXISTS ranked_tournaments_enabled;

-- =====================================================
-- REMOVE MIGRATION RECORDS
-- =====================================================

DELETE FROM schema_migrations WHERE version IN ('009', '010', '011', '012');

-- =====================================================
-- VERIFICATION
-- =====================================================

-- Verify cleanup was successful
SELECT 'Cleanup complete!' as status;
SELECT COUNT(*) as remaining_game_tables
FROM information_schema.tables
WHERE table_name LIKE 'game_%'
AND table_schema = 'public';

SELECT COUNT(*) as remaining_migrations
FROM schema_migrations
WHERE version IN ('009', '010', '011', '012');

COMMIT;
