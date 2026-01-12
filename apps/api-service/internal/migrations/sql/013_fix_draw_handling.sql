-- =====================================================
-- FIX: Draw Handling in Battle Mechanics
-- =====================================================
-- This migration fixes critical bugs in the battle system:
-- 1. Draws were detected but not recorded in leaderboard
-- 2. Head-to-head records had no draw tracking
-- 3. Players could have incomplete tournament records
--
-- Changes:
-- - Add ranked_draws and regular_draws columns
-- - Update update_game_leaderboard() to handle draws
-- - Update head-to-head tracking to include draws
-- - Preserve win streaks when draws occur (design choice)

-- =====================================================
-- ADD DRAW TRACKING COLUMNS
-- =====================================================

ALTER TABLE game_leaderboard
    ADD COLUMN IF NOT EXISTS ranked_draws INTEGER DEFAULT 0,
    ADD COLUMN IF NOT EXISTS regular_draws INTEGER DEFAULT 0;

-- Add comment for clarity
COMMENT ON COLUMN game_leaderboard.ranked_draws IS 'Count of draws in ranked matches';
COMMENT ON COLUMN game_leaderboard.regular_draws IS 'Count of draws in regular/casual matches';

-- =====================================================
-- UPDATE LEADERBOARD FUNCTION WITH DRAW SUPPORT
-- =====================================================

CREATE OR REPLACE FUNCTION update_game_leaderboard(
    p_user_id BIGINT,
    p_chat_id BIGINT,
    p_match_type game_match_type,
    p_is_win BOOLEAN,
    p_opponent_id BIGINT DEFAULT NULL,
    p_is_tournament_win BOOLEAN DEFAULT FALSE,
    p_is_draw BOOLEAN DEFAULT FALSE
) RETURNS VOID AS $$
DECLARE
    v_h2h JSONB;
    v_opponent_key TEXT;
    v_current_wins INT;
    v_current_losses INT;
    v_current_draws INT;
BEGIN
    -- Upsert leaderboard entry
    INSERT INTO game_leaderboard (user_id, chat_id, first_match_at, last_match_at)
    VALUES (p_user_id, p_chat_id, NOW(), NOW())
    ON CONFLICT (user_id, chat_id) DO UPDATE
    SET last_match_at = NOW(),
        updated_at = NOW();

    -- Update stats based on match type
    IF p_match_type = 'ranked' THEN
        IF p_is_draw THEN
            -- Handle draw: increment matches_played and draws, PRESERVE streak
            UPDATE game_leaderboard
            SET ranked_tournaments_played = ranked_tournaments_played + 1,
                ranked_draws = ranked_draws + 1
                -- NOTE: ranked_current_streak is NOT modified (preserved)
            WHERE user_id = p_user_id AND chat_id = p_chat_id;
        ELSIF p_is_win THEN
            -- Win: increment wins, matches_played, and streak
            UPDATE game_leaderboard
            SET ranked_wins = ranked_wins + 1,
                ranked_tournaments_played = ranked_tournaments_played + 1,
                ranked_current_streak = ranked_current_streak + 1,
                ranked_best_streak = GREATEST(ranked_best_streak, ranked_current_streak + 1),
                ranked_tournaments_won = ranked_tournaments_won + (CASE WHEN p_is_tournament_win THEN 1 ELSE 0 END)
            WHERE user_id = p_user_id AND chat_id = p_chat_id;
        ELSE
            -- Loss: increment losses, matches_played, and reset streak
            UPDATE game_leaderboard
            SET ranked_losses = ranked_losses + 1,
                ranked_tournaments_played = ranked_tournaments_played + 1,
                ranked_current_streak = 0
            WHERE user_id = p_user_id AND chat_id = p_chat_id;
        END IF;
    ELSE
        -- Regular/casual matches
        IF p_is_draw THEN
            -- Handle draw: increment matches_played and draws, PRESERVE streak
            UPDATE game_leaderboard
            SET regular_matches_played = regular_matches_played + 1,
                regular_draws = regular_draws + 1
                -- NOTE: regular_current_streak is NOT modified (preserved)
            WHERE user_id = p_user_id AND chat_id = p_chat_id;
        ELSIF p_is_win THEN
            -- Win: increment wins, matches_played, and streak
            UPDATE game_leaderboard
            SET regular_wins = regular_wins + 1,
                regular_matches_played = regular_matches_played + 1,
                regular_current_streak = regular_current_streak + 1,
                regular_best_streak = GREATEST(regular_best_streak, regular_current_streak + 1)
            WHERE user_id = p_user_id AND chat_id = p_chat_id;
        ELSE
            -- Loss: increment losses, matches_played, and reset streak
            UPDATE game_leaderboard
            SET regular_losses = regular_losses + 1,
                regular_matches_played = regular_matches_played + 1,
                regular_current_streak = 0
            WHERE user_id = p_user_id AND chat_id = p_chat_id;
        END IF;
    END IF;

    -- Update head-to-head if opponent specified
    IF p_opponent_id IS NOT NULL THEN
        v_opponent_key := p_opponent_id::TEXT;

        SELECT head_to_head INTO v_h2h
        FROM game_leaderboard
        WHERE user_id = p_user_id AND chat_id = p_chat_id;

        -- Initialize opponent record if not exists
        IF NOT v_h2h ? v_opponent_key THEN
            v_h2h := v_h2h || jsonb_build_object(v_opponent_key, '{"wins": 0, "losses": 0, "draws": 0}'::jsonb);
        END IF;

        -- Update wins/losses/draws
        IF p_is_draw THEN
            v_current_draws := COALESCE((v_h2h -> v_opponent_key ->> 'draws')::INT, 0);
            v_h2h := jsonb_set(v_h2h, ARRAY[v_opponent_key, 'draws'], to_jsonb(v_current_draws + 1));
        ELSIF p_is_win THEN
            v_current_wins := (v_h2h -> v_opponent_key ->> 'wins')::INT;
            v_h2h := jsonb_set(v_h2h, ARRAY[v_opponent_key, 'wins'], to_jsonb(v_current_wins + 1));
        ELSE
            v_current_losses := (v_h2h -> v_opponent_key ->> 'losses')::INT;
            v_h2h := jsonb_set(v_h2h, ARRAY[v_opponent_key, 'losses'], to_jsonb(v_current_losses + 1));
        END IF;

        UPDATE game_leaderboard
        SET head_to_head = v_h2h
        WHERE user_id = p_user_id AND chat_id = p_chat_id;
    END IF;
END;
$$ LANGUAGE plpgsql;
