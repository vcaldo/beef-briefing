-- Database schema additions for ML Analytics
-- This migration adds tables for storing ML processing results

-- =====================================================
-- PROCESSING STATE
-- =====================================================

-- Track which messages have been processed by ML models
CREATE TABLE IF NOT EXISTS ml_processing_state (
    message_id BIGINT PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
    chat_id BIGINT NOT NULL,
    processor_version VARCHAR(32) NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ml_processing_state_chat
ON ml_processing_state(chat_id);

CREATE INDEX IF NOT EXISTS idx_ml_processing_state_processed_at
ON ml_processing_state(processed_at DESC);

COMMENT ON TABLE ml_processing_state IS
'Tracks which messages have been processed by the ML processor and when.';

-- =====================================================
-- SENTIMENT ANALYSIS RESULTS
-- =====================================================

CREATE TABLE IF NOT EXISTS ml_sentiment (
    message_id BIGINT PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
    chat_id BIGINT NOT NULL,
    label VARCHAR(16) NOT NULL,  -- 'positive', 'neutral', 'negative'
    score_positive REAL NOT NULL,
    score_neutral REAL NOT NULL,
    score_negative REAL NOT NULL,
    confidence REAL NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ml_sentiment_chat
ON ml_sentiment(chat_id);

CREATE INDEX IF NOT EXISTS idx_ml_sentiment_label
ON ml_sentiment(chat_id, label);

COMMENT ON TABLE ml_sentiment IS
'Sentiment analysis results using distilbert-multilingual-sentiments model.';

-- =====================================================
-- TOXICITY DETECTION RESULTS
-- =====================================================

CREATE TABLE IF NOT EXISTS ml_toxicity (
    message_id BIGINT PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
    chat_id BIGINT NOT NULL,
    is_toxic BOOLEAN NOT NULL,
    label VARCHAR(32) NOT NULL,
    score REAL NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ml_toxicity_chat
ON ml_toxicity(chat_id);

CREATE INDEX IF NOT EXISTS idx_ml_toxicity_is_toxic
ON ml_toxicity(chat_id, is_toxic) WHERE is_toxic = true;

COMMENT ON TABLE ml_toxicity IS
'Toxicity/hate speech detection results using IMSyPP/hate_speech_pt model.';

-- =====================================================
-- TOPIC CLUSTERS
-- =====================================================

-- Topic clusters discovered per chat
CREATE TABLE IF NOT EXISTS ml_topics (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    topic_id INT NOT NULL,
    keywords TEXT[],
    message_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(chat_id, topic_id)
);

CREATE INDEX IF NOT EXISTS idx_ml_topics_chat
ON ml_topics(chat_id);

CREATE TRIGGER update_ml_topics_updated_at
BEFORE UPDATE ON ml_topics
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE ml_topics IS
'Topic clusters derived from message embeddings via HDBSCAN clustering.';

-- Message-to-topic assignments
CREATE TABLE IF NOT EXISTS ml_message_topics (
    message_id BIGINT PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
    chat_id BIGINT NOT NULL,
    topic_id INT NOT NULL,
    similarity REAL NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ml_message_topics_chat_topic
ON ml_message_topics(chat_id, topic_id);

COMMENT ON TABLE ml_message_topics IS
'Maps messages to their assigned topic cluster.';

-- =====================================================
-- USER ML PROFILES
-- =====================================================

-- Aggregated ML-derived profiles per user per chat
CREATE TABLE IF NOT EXISTS ml_user_profiles (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    chat_id BIGINT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    avg_sentiment REAL,
    sentiment_variance REAL,
    toxicity_rate REAL,
    messages_analyzed INT NOT NULL DEFAULT 0,
    top_topics JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, chat_id)
);

CREATE INDEX IF NOT EXISTS idx_ml_user_profiles_chat
ON ml_user_profiles(chat_id);

CREATE INDEX IF NOT EXISTS idx_ml_user_profiles_user
ON ml_user_profiles(user_id);

CREATE TRIGGER update_ml_user_profiles_updated_at
BEFORE UPDATE ON ml_user_profiles
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE ml_user_profiles IS
'Aggregated ML-derived personality profiles per user per chat.';

-- =====================================================
-- REFRESH FUNCTION
-- =====================================================

-- Function to refresh user profiles from ML results
CREATE OR REPLACE FUNCTION refresh_ml_user_profiles(target_chat_id BIGINT DEFAULT NULL)
RETURNS void AS $$
BEGIN
    INSERT INTO ml_user_profiles (
        user_id, chat_id, avg_sentiment, sentiment_variance,
        toxicity_rate, messages_analyzed, updated_at
    )
    SELECT
        m.user_id,
        m.chat_id,
        AVG(CASE ms.label
            WHEN 'positive' THEN ms.confidence
            WHEN 'negative' THEN -ms.confidence
            ELSE 0
        END) as avg_sentiment,
        VARIANCE(CASE ms.label
            WHEN 'positive' THEN ms.confidence
            WHEN 'negative' THEN -ms.confidence
            ELSE 0
        END) as sentiment_variance,
        AVG(CASE WHEN mt.is_toxic THEN 1.0 ELSE 0.0 END) as toxicity_rate,
        COUNT(*) as messages_analyzed,
        NOW() as updated_at
    FROM messages m
    JOIN ml_sentiment ms ON ms.message_id = m.id
    LEFT JOIN ml_toxicity mt ON mt.message_id = m.id
    WHERE m.user_id IS NOT NULL
        AND (target_chat_id IS NULL OR m.chat_id = target_chat_id)
    GROUP BY m.user_id, m.chat_id
    ON CONFLICT (user_id, chat_id) DO UPDATE SET
        avg_sentiment = EXCLUDED.avg_sentiment,
        sentiment_variance = EXCLUDED.sentiment_variance,
        toxicity_rate = EXCLUDED.toxicity_rate,
        messages_analyzed = EXCLUDED.messages_analyzed,
        updated_at = NOW();

    RAISE NOTICE 'ML user profiles refreshed at %', NOW();
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION refresh_ml_user_profiles IS
'Refreshes ML user profiles from sentiment and toxicity results. Can target specific chat.';
