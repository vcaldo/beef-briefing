-- Database schema additions for Beef Dashboard
-- This migration adds session storage and materialized views for analytics

-- =====================================================
-- DASHBOARD SESSIONS
-- =====================================================

-- Session storage for dashboard authentication
CREATE TABLE IF NOT EXISTS dashboard_sessions (
    session_id VARCHAR(64) PRIMARY KEY,
    user_id BIGINT NOT NULL,
    username VARCHAR(255),
    first_name VARCHAR(255) NOT NULL,
    photo_url TEXT,
    allowed_chat_ids TEXT,  -- Comma-separated list of chat IDs user can access
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    last_accessed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dashboard_sessions_user_id
ON dashboard_sessions(user_id);

CREATE INDEX IF NOT EXISTS idx_dashboard_sessions_expires_at
ON dashboard_sessions(expires_at);

COMMENT ON TABLE dashboard_sessions IS
'Stores user sessions for the analytics dashboard. Sessions expire after 7 days by default.';

-- =====================================================
-- MATERIALIZED VIEWS FOR ANALYTICS
-- =====================================================

-- Daily message statistics per chat
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_daily_message_stats AS
SELECT
    m.chat_id,
    DATE(m.date) as date,
    COUNT(*) as message_count,
    COUNT(DISTINCT m.user_id) as unique_users,
    COUNT(DISTINCT CASE WHEN m.text IS NOT NULL THEN m.id END) as text_messages,
    COUNT(DISTINCT CASE WHEN m.caption IS NOT NULL THEN m.id END) as media_messages,
    AVG(LENGTH(COALESCE(m.text, ''))) as avg_message_length
FROM messages m
GROUP BY m.chat_id, DATE(m.date);

CREATE UNIQUE INDEX IF NOT EXISTS idx_mv_daily_stats_chat_date
ON mv_daily_message_stats(chat_id, date);

COMMENT ON MATERIALIZED VIEW mv_daily_message_stats IS
'Pre-computed daily message statistics for fast dashboard loading. Refresh daily.';

-- User statistics per chat
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_user_statistics AS
WITH user_msgs AS (
    SELECT
        m.chat_id,
        m.user_id,
        COUNT(*) as message_count,
        MIN(m.date) as first_message_date,
        MAX(m.date) as last_message_date,
        COUNT(DISTINCT DATE(m.date)) as active_days,
        AVG(LENGTH(COALESCE(m.text, ''))) as avg_message_length
    FROM messages m
    WHERE m.user_id IS NOT NULL
    GROUP BY m.chat_id, m.user_id
),
user_reactions_sent AS (
    SELECT
        mr.chat_id,
        mr.user_id,
        COUNT(*) as reactions_sent
    FROM message_reactions mr
    WHERE mr.user_id IS NOT NULL AND mr.is_removed = FALSE
    GROUP BY mr.chat_id, mr.user_id
),
user_reactions_received AS (
    SELECT
        m.chat_id,
        m.user_id,
        COUNT(*) as reactions_received
    FROM message_reactions mr
    JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
    WHERE m.user_id IS NOT NULL AND mr.is_removed = FALSE
    GROUP BY m.chat_id, m.user_id
)
SELECT
    um.chat_id,
    um.user_id,
    u.first_name,
    u.last_name,
    u.username,
    u.is_premium,
    u.is_bot,
    um.message_count,
    um.first_message_date,
    um.last_message_date,
    um.active_days,
    um.avg_message_length,
    COALESCE(urs.reactions_sent, 0) as reactions_sent,
    COALESCE(urr.reactions_received, 0) as reactions_received
FROM user_msgs um
JOIN users u ON u.id = um.user_id
LEFT JOIN user_reactions_sent urs ON urs.chat_id = um.chat_id AND urs.user_id = um.user_id
LEFT JOIN user_reactions_received urr ON urr.chat_id = um.chat_id AND urr.user_id = um.user_id;

CREATE UNIQUE INDEX IF NOT EXISTS idx_mv_user_stats_chat_user
ON mv_user_statistics(chat_id, user_id);

CREATE INDEX IF NOT EXISTS idx_mv_user_stats_message_count
ON mv_user_statistics(chat_id, message_count DESC);

COMMENT ON MATERIALIZED VIEW mv_user_statistics IS
'Pre-computed user statistics for leaderboards and user cards. Refresh daily.';

-- Hourly activity patterns per chat
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_hourly_activity AS
SELECT
    m.chat_id,
    EXTRACT(DOW FROM m.date)::int as day_of_week,
    EXTRACT(HOUR FROM m.date)::int as hour,
    COUNT(*) as message_count,
    COUNT(DISTINCT m.user_id) as unique_users
FROM messages m
GROUP BY m.chat_id, EXTRACT(DOW FROM m.date), EXTRACT(HOUR FROM m.date);

CREATE UNIQUE INDEX IF NOT EXISTS idx_mv_hourly_activity_key
ON mv_hourly_activity(chat_id, day_of_week, hour);

COMMENT ON MATERIALIZED VIEW mv_hourly_activity IS
'Pre-computed hourly activity patterns for heatmaps. Refresh weekly.';

-- Media distribution per chat
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_media_distribution AS
SELECT
    m.chat_id,
    mf.media_type::text as media_type,
    COUNT(*) as count,
    SUM(mf.file_size) as total_size
FROM media_files mf
JOIN messages m ON m.id = mf.message_id
GROUP BY m.chat_id, mf.media_type

UNION ALL

SELECT
    m.chat_id,
    'photo' as media_type,
    COUNT(*) as count,
    SUM(p.file_size) as total_size
FROM photos p
JOIN messages m ON m.id = p.message_id
GROUP BY m.chat_id;

CREATE INDEX IF NOT EXISTS idx_mv_media_dist_chat
ON mv_media_distribution(chat_id);

COMMENT ON MATERIALIZED VIEW mv_media_distribution IS
'Pre-computed media type distribution. Refresh daily.';

-- Reaction distribution per chat
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_reaction_distribution AS
SELECT
    mr.chat_id,
    mr.reaction_type::text as reaction_type,
    COALESCE(mr.emoji_value, mr.custom_emoji_id, 'paid') as emoji,
    COUNT(*) as count
FROM message_reactions mr
WHERE mr.is_removed = FALSE
GROUP BY mr.chat_id, mr.reaction_type, mr.emoji_value, mr.custom_emoji_id;

CREATE INDEX IF NOT EXISTS idx_mv_reaction_dist_chat
ON mv_reaction_distribution(chat_id);

COMMENT ON MATERIALIZED VIEW mv_reaction_distribution IS
'Pre-computed reaction distribution. Refresh daily.';

-- =====================================================
-- REFRESH FUNCTIONS
-- =====================================================

-- Function to refresh all dashboard materialized views
CREATE OR REPLACE FUNCTION refresh_dashboard_views()
RETURNS void AS $$
BEGIN
    REFRESH MATERIALIZED VIEW CONCURRENTLY mv_daily_message_stats;
    REFRESH MATERIALIZED VIEW CONCURRENTLY mv_user_statistics;
    REFRESH MATERIALIZED VIEW CONCURRENTLY mv_hourly_activity;
    REFRESH MATERIALIZED VIEW mv_media_distribution;
    REFRESH MATERIALIZED VIEW mv_reaction_distribution;

    RAISE NOTICE 'Dashboard materialized views refreshed at %', NOW();
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION refresh_dashboard_views IS
'Refreshes all dashboard materialized views. Call daily via cron or pg_cron.';

-- Function to cleanup expired dashboard sessions
CREATE OR REPLACE FUNCTION cleanup_expired_dashboard_sessions()
RETURNS integer AS $$
DECLARE
    deleted_count integer;
BEGIN
    DELETE FROM dashboard_sessions
    WHERE expires_at < NOW();

    GET DIAGNOSTICS deleted_count = ROW_COUNT;

    IF deleted_count > 0 THEN
        RAISE NOTICE 'Deleted % expired dashboard sessions', deleted_count;
    END IF;

    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION cleanup_expired_dashboard_sessions IS
'Removes expired dashboard sessions. Call periodically via cron.';

-- =====================================================
-- GRANTS FOR READ-ONLY DASHBOARD ACCESS
-- =====================================================

-- Note: This creates a read-only role for the dashboard.
-- In production, uncomment and adjust the password.

-- CREATE ROLE dashboard_readonly WITH LOGIN PASSWORD 'change_me_in_production';
-- GRANT CONNECT ON DATABASE beef_briefing TO dashboard_readonly;
-- GRANT USAGE ON SCHEMA public TO dashboard_readonly;
-- GRANT SELECT ON ALL TABLES IN SCHEMA public TO dashboard_readonly;
-- GRANT SELECT ON ALL SEQUENCES IN SCHEMA public TO dashboard_readonly;
-- ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO dashboard_readonly;

-- For session management, the dashboard needs write access to sessions table only:
-- GRANT INSERT, UPDATE, DELETE ON dashboard_sessions TO dashboard_readonly;
