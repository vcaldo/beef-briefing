-- =====================================================
-- RANKED TOURNAMENTS - Daily Tournament Tracking
-- =====================================================
-- Tracks tournament lifecycle separate from matches.
-- Participants join the tournament, then a match is created at 18:00.

-- Tournament statuses
CREATE TYPE game_tournament_status AS ENUM (
    'scheduled',    -- Created but not announced yet
    'open',         -- Announced, accepting participants (00:01 - 18:00)
    'in_progress',  -- Registration closed, match running
    'completed',    -- Tournament finished with winner
    'skipped'       -- No participants at close time
);

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
-- Tracks users who joined before match creation.
-- Separate from game_match_participants which is created later.

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
