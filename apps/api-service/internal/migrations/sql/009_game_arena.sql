-- =====================================================
-- BEEF ARENA - Card Game Schema (Consolidated)
-- =====================================================
-- This migration consolidates the entire game arena feature:
-- - Core match system with 1v1 and arena formats
-- - Ranked daily tournaments
-- - Leaderboard tracking
-- - Per-group tournament configuration

-- =====================================================
-- ENUMS
-- =====================================================

-- Match types
CREATE TYPE game_match_type AS ENUM ('ranked', 'regular');
CREATE TYPE game_match_format AS ENUM ('1v1', 'arena');
CREATE TYPE game_match_status AS ENUM ('open', 'shop_phase', 'battle_phase', 'completed', 'cancelled');
CREATE TYPE game_participant_status AS ENUM ('joined', 'ready', 'eliminated', 'winner');

-- Tournament statuses
CREATE TYPE game_tournament_status AS ENUM (
    'scheduled',    -- Created but not announced yet
    'open',         -- Announced, accepting participants (00:01 - 18:00)
    'in_progress',  -- Registration closed, match running
    'completed',    -- Tournament finished with winner
    'skipped'       -- No participants at close time
);

-- =====================================================
-- GAME MATCHES
-- =====================================================

CREATE TABLE IF NOT EXISTS game_matches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id BIGINT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    match_type game_match_type NOT NULL,
    format game_match_format,  -- NULL until determined by participant count

    status game_match_status NOT NULL DEFAULT 'open',

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    join_deadline TIMESTAMPTZ,  -- For regular matches (5 min window)
    shop_phase_started_at TIMESTAMPTZ,
    shop_phase_deadline TIMESTAMPTZ,  -- 3 min after shop_phase_started_at
    battle_started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,

    -- Tournament info (ranked only)
    tournament_date DATE,  -- For ranked matches, the day of the tournament

    -- Regular match info
    creator_user_id BIGINT REFERENCES users(id),

    -- Battle state
    current_round INT DEFAULT 0,

    -- Results
    winner_user_id BIGINT REFERENCES users(id),

    -- Constraints
    CONSTRAINT ranked_has_tournament_date CHECK (
        match_type != 'ranked' OR tournament_date IS NOT NULL
    ),
    CONSTRAINT regular_has_creator CHECK (
        match_type != 'regular' OR creator_user_id IS NOT NULL
    )
);

-- Indexes for common queries
CREATE INDEX idx_game_matches_chat_status ON game_matches(chat_id, status);
CREATE INDEX idx_game_matches_status_active ON game_matches(status)
    WHERE status NOT IN ('completed', 'cancelled');
CREATE INDEX idx_game_matches_tournament ON game_matches(chat_id, tournament_date)
    WHERE match_type = 'ranked';
CREATE INDEX idx_game_matches_created ON game_matches(created_at DESC);

-- =====================================================
-- MATCH PARTICIPANTS
-- =====================================================

CREATE TABLE IF NOT EXISTS game_match_participants (
    id BIGSERIAL PRIMARY KEY,
    match_id UUID NOT NULL REFERENCES game_matches(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Status tracking
    status game_participant_status NOT NULL DEFAULT 'joined',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Shop phase state
    coins_remaining INT NOT NULL DEFAULT 10,

    -- Available cards in shop (6 random from current week)
    -- Format: [{card_id, user_id, first_name, username, photo_url, atk, def, hp, stats}]
    shop_cards JSONB,

    -- Player's team (3 cards with upgrades applied)
    -- Format: [{card_id, user_id, first_name, username, photo_url, atk, def, hp, atk_upgrades, hp_upgrades}]
    team JSONB,

    -- Team positions (0=front, 1=mid, 2=back) - indexes into team array
    team_order INT[] DEFAULT ARRAY[0, 1, 2],

    team_submitted_at TIMESTAMPTZ,

    -- Results
    placement INT,  -- 1 = winner, 2 = runner-up, etc.
    wins INT DEFAULT 0,
    losses INT DEFAULT 0,
    total_damage_dealt INT DEFAULT 0,

    UNIQUE(match_id, user_id)
);

CREATE INDEX idx_game_participants_match ON game_match_participants(match_id);
CREATE INDEX idx_game_participants_user ON game_match_participants(user_id);
CREATE INDEX idx_game_participants_status ON game_match_participants(match_id, status);

-- =====================================================
-- MATCH ROUNDS (Battle Logs)
-- =====================================================

CREATE TABLE IF NOT EXISTS game_match_rounds (
    id BIGSERIAL PRIMARY KEY,
    match_id UUID NOT NULL REFERENCES game_matches(id) ON DELETE CASCADE,
    round_number INT NOT NULL,

    -- Combatants
    player_a_id BIGINT NOT NULL REFERENCES users(id),
    player_b_id BIGINT NOT NULL REFERENCES users(id),

    -- Teams at start of battle (snapshot for replay)
    -- Format: [{card_id, user_id, first_name, atk, def, hp, max_hp, position}]
    player_a_team JSONB NOT NULL,
    player_b_team JSONB NOT NULL,

    -- Result
    winner_id BIGINT REFERENCES users(id),  -- NULL for draw
    is_draw BOOLEAN NOT NULL DEFAULT FALSE,

    -- Battle replay data
    -- Format: [{type, round, attacker_id, defender_id, damage, hp_before, hp_after, message}]
    battle_log JSONB NOT NULL,

    -- Stats
    player_a_damage INT NOT NULL DEFAULT 0,
    player_b_damage INT NOT NULL DEFAULT 0,
    total_rounds INT NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(match_id, round_number, player_a_id, player_b_id)
);

CREATE INDEX idx_game_rounds_match ON game_match_rounds(match_id);
CREATE INDEX idx_game_rounds_players ON game_match_rounds(player_a_id, player_b_id);

-- =====================================================
-- GAME LEADERBOARD (Aggregated Stats)
-- =====================================================

CREATE TABLE IF NOT EXISTS game_leaderboard (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    chat_id BIGINT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,

    -- Ranked tournament stats
    ranked_wins INT NOT NULL DEFAULT 0,
    ranked_losses INT NOT NULL DEFAULT 0,
    ranked_tournaments_played INT NOT NULL DEFAULT 0,
    ranked_tournaments_won INT NOT NULL DEFAULT 0,
    ranked_current_streak INT NOT NULL DEFAULT 0,
    ranked_best_streak INT NOT NULL DEFAULT 0,

    -- Regular (casual) match stats
    regular_wins INT NOT NULL DEFAULT 0,
    regular_losses INT NOT NULL DEFAULT 0,
    regular_matches_played INT NOT NULL DEFAULT 0,
    regular_current_streak INT NOT NULL DEFAULT 0,
    regular_best_streak INT NOT NULL DEFAULT 0,

    -- Head-to-head records
    -- Format: {"opponent_user_id": {"wins": N, "losses": N}}
    head_to_head JSONB NOT NULL DEFAULT '{}'::jsonb,

    -- Timestamps
    first_match_at TIMESTAMPTZ,
    last_match_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (user_id, chat_id)
);

CREATE INDEX idx_game_leaderboard_chat ON game_leaderboard(chat_id);
CREATE INDEX idx_game_leaderboard_ranked ON game_leaderboard(chat_id, ranked_wins DESC);
CREATE INDEX idx_game_leaderboard_regular ON game_leaderboard(chat_id, regular_wins DESC);

-- =====================================================
-- RANKED TOURNAMENTS
-- =====================================================

CREATE TABLE IF NOT EXISTS game_ranked_tournaments (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    tournament_date DATE NOT NULL,
    status game_tournament_status NOT NULL DEFAULT 'scheduled',

    -- Announcement tracking
    announcement_message_id BIGINT,  -- Telegram message ID for editing/referencing
    announced_at TIMESTAMPTZ,

    -- Lifecycle timestamps
    registration_closed_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,

    -- Link to match (created when registration closes)
    match_id UUID REFERENCES game_matches(id) ON DELETE SET NULL,

    -- Results (copied from match for convenience)
    winner_user_id BIGINT REFERENCES users(id),
    participant_count INT NOT NULL DEFAULT 0,

    -- Bracket state for arena format (3+ players)
    -- Format: {rounds: [[{player_a, player_b, winner}]], current_round: N, next_round_at: timestamp}
    bracket_state JSONB,

    -- Metadata
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Only one tournament per chat per day
    UNIQUE(chat_id, tournament_date)
);

-- Indexes for scheduler queries
CREATE INDEX idx_ranked_tournaments_status ON game_ranked_tournaments(status);
CREATE INDEX idx_ranked_tournaments_chat_date ON game_ranked_tournaments(chat_id, tournament_date);
CREATE INDEX idx_ranked_tournaments_scheduled ON game_ranked_tournaments(status, chat_id)
    WHERE status IN ('scheduled', 'open', 'in_progress');

-- =====================================================
-- TOURNAMENT PARTICIPANTS
-- =====================================================

CREATE TABLE IF NOT EXISTS game_tournament_participants (
    id BIGSERIAL PRIMARY KEY,
    tournament_id BIGINT NOT NULL REFERENCES game_ranked_tournaments(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Only one entry per user per tournament
    UNIQUE(tournament_id, user_id)
);

CREATE INDEX idx_tournament_participants_tournament ON game_tournament_participants(tournament_id);
CREATE INDEX idx_tournament_participants_user ON game_tournament_participants(user_id);

-- =====================================================
-- PER-GROUP TOURNAMENT CONFIGURATION
-- =====================================================

ALTER TABLE chats
ADD COLUMN IF NOT EXISTS ranked_tournaments_enabled BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_chats_ranked_enabled
ON chats(ranked_tournaments_enabled)
WHERE ranked_tournaments_enabled = true;

COMMENT ON COLUMN chats.ranked_tournaments_enabled IS
'Controls whether daily ranked tournaments are enabled for this chat. Default: false (opt-in)';

-- =====================================================
-- HELPER FUNCTION: Update leaderboard after match
-- =====================================================

CREATE OR REPLACE FUNCTION update_game_leaderboard(
    p_user_id BIGINT,
    p_chat_id BIGINT,
    p_match_type game_match_type,
    p_is_win BOOLEAN,
    p_opponent_id BIGINT DEFAULT NULL,
    p_is_tournament_win BOOLEAN DEFAULT FALSE
) RETURNS VOID AS $$
DECLARE
    v_h2h JSONB;
    v_opponent_key TEXT;
    v_current_wins INT;
    v_current_losses INT;
BEGIN
    -- Upsert leaderboard entry
    INSERT INTO game_leaderboard (user_id, chat_id, first_match_at, last_match_at)
    VALUES (p_user_id, p_chat_id, NOW(), NOW())
    ON CONFLICT (user_id, chat_id) DO UPDATE
    SET last_match_at = NOW(),
        updated_at = NOW();

    -- Update stats based on match type
    IF p_match_type = 'ranked' THEN
        IF p_is_win THEN
            UPDATE game_leaderboard
            SET ranked_wins = ranked_wins + 1,
                ranked_tournaments_played = ranked_tournaments_played + 1,
                ranked_current_streak = ranked_current_streak + 1,
                ranked_best_streak = GREATEST(ranked_best_streak, ranked_current_streak + 1),
                ranked_tournaments_won = ranked_tournaments_won + (CASE WHEN p_is_tournament_win THEN 1 ELSE 0 END)
            WHERE user_id = p_user_id AND chat_id = p_chat_id;
        ELSE
            UPDATE game_leaderboard
            SET ranked_losses = ranked_losses + 1,
                ranked_tournaments_played = ranked_tournaments_played + 1,
                ranked_current_streak = 0
            WHERE user_id = p_user_id AND chat_id = p_chat_id;
        END IF;
    ELSE
        IF p_is_win THEN
            UPDATE game_leaderboard
            SET regular_wins = regular_wins + 1,
                regular_matches_played = regular_matches_played + 1,
                regular_current_streak = regular_current_streak + 1,
                regular_best_streak = GREATEST(regular_best_streak, regular_current_streak + 1)
            WHERE user_id = p_user_id AND chat_id = p_chat_id;
        ELSE
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
            v_h2h := v_h2h || jsonb_build_object(v_opponent_key, '{"wins": 0, "losses": 0}'::jsonb);
        END IF;

        -- Update wins/losses
        IF p_is_win THEN
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

-- =====================================================
-- HELPER: Get tournaments needing announcement
-- =====================================================
-- Returns tournaments in 'scheduled' status for chats where local time is 00:01-00:10

CREATE OR REPLACE FUNCTION get_tournaments_needing_announcement(p_current_time TIMESTAMPTZ)
RETURNS TABLE (
    tournament_id BIGINT,
    chat_id BIGINT,
    timezone VARCHAR,
    tournament_date DATE
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        t.id AS tournament_id,
        t.chat_id,
        COALESCE(c.timezone, 'America/Sao_Paulo') AS timezone,
        t.tournament_date
    FROM game_ranked_tournaments t
    JOIN chats c ON c.id = t.chat_id
    WHERE t.status = 'scheduled'
      AND c.ranked_tournaments_enabled = true
      AND (p_current_time AT TIME ZONE COALESCE(c.timezone, 'America/Sao_Paulo'))::TIME
          BETWEEN '00:01:00' AND '00:10:00'
      AND t.tournament_date = (p_current_time AT TIME ZONE COALESCE(c.timezone, 'America/Sao_Paulo'))::DATE;
END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- HELPER: Get tournaments needing registration close
-- =====================================================
-- Returns tournaments in 'open' status for chats where local time is 18:00-18:05

CREATE OR REPLACE FUNCTION get_tournaments_needing_close(p_current_time TIMESTAMPTZ)
RETURNS TABLE (
    tournament_id BIGINT,
    chat_id BIGINT,
    timezone VARCHAR,
    tournament_date DATE,
    participant_count BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        t.id AS tournament_id,
        t.chat_id,
        COALESCE(c.timezone, 'America/Sao_Paulo') AS timezone,
        t.tournament_date,
        (SELECT COUNT(*) FROM game_tournament_participants p WHERE p.tournament_id = t.id)
    FROM game_ranked_tournaments t
    JOIN chats c ON c.id = t.chat_id
    WHERE t.status = 'open'
      AND c.ranked_tournaments_enabled = true
      AND (p_current_time AT TIME ZONE COALESCE(c.timezone, 'America/Sao_Paulo'))::TIME
          BETWEEN '18:00:00' AND '18:05:00'
      AND t.tournament_date = (p_current_time AT TIME ZONE COALESCE(c.timezone, 'America/Sao_Paulo'))::DATE;
END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- HELPER: Create or get today's tournament for a chat
-- =====================================================

CREATE OR REPLACE FUNCTION get_or_create_tournament(
    p_chat_id BIGINT,
    p_date DATE
) RETURNS BIGINT AS $$
DECLARE
    v_tournament_id BIGINT;
BEGIN
    -- Try to get existing tournament
    SELECT id INTO v_tournament_id
    FROM game_ranked_tournaments
    WHERE chat_id = p_chat_id AND tournament_date = p_date;

    -- Create if not exists
    IF v_tournament_id IS NULL THEN
        INSERT INTO game_ranked_tournaments (chat_id, tournament_date)
        VALUES (p_chat_id, p_date)
        RETURNING id INTO v_tournament_id;
    END IF;

    RETURN v_tournament_id;
END;
$$ LANGUAGE plpgsql;
