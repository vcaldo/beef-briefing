-- Card Image Storage Schema
-- Stores references to rendered card images in object storage

-- =====================================================
-- RENDERED CARD IMAGES
-- =====================================================

CREATE TABLE IF NOT EXISTS ml_user_card_images (
    id BIGSERIAL PRIMARY KEY,

    -- Link to the source card data
    card_id BIGINT NOT NULL REFERENCES ml_user_cards(id) ON DELETE CASCADE,

    -- Denormalized for query convenience (avoids joins for common lookups)
    user_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL,
    week_start DATE NOT NULL,

    -- Image storage info
    storage_path TEXT NOT NULL,
    file_hash VARCHAR(64) NOT NULL,
    file_size INTEGER NOT NULL,
    mime_type VARCHAR(32) NOT NULL DEFAULT 'image/png',

    -- Image dimensions
    width INTEGER NOT NULL DEFAULT 400,
    height INTEGER NOT NULL DEFAULT 600,

    -- Rendering metadata
    theme VARCHAR(32) NOT NULL DEFAULT 'gaming',
    template_version INTEGER NOT NULL DEFAULT 1,
    card_data_version INTEGER NOT NULL,

    -- Timestamps
    generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- One image per card per theme
    UNIQUE(card_id, theme)
);

-- Query by chat and week (most common query pattern)
CREATE INDEX IF NOT EXISTS idx_card_images_chat_week
ON ml_user_card_images(chat_id, week_start DESC);

-- Query by user
CREATE INDEX IF NOT EXISTS idx_card_images_user
ON ml_user_card_images(user_id);

-- Deduplication lookup
CREATE INDEX IF NOT EXISTS idx_card_images_hash
ON ml_user_card_images(file_hash);

-- Find images for a specific card
CREATE INDEX IF NOT EXISTS idx_card_images_card
ON ml_user_card_images(card_id);

COMMENT ON TABLE ml_user_card_images IS
'Stores references to rendered card images in object storage. Links to ml_user_cards for source data.';

COMMENT ON COLUMN ml_user_card_images.storage_path IS
'Object storage path: cards/{chat_id}/{week_start}/{user_id}.png';

COMMENT ON COLUMN ml_user_card_images.card_data_version IS
'Version of ml_user_cards.card_version when image was generated. Used to detect stale images.';

COMMENT ON COLUMN ml_user_card_images.theme IS
'Visual theme used for rendering (e.g., gaming, neon, minimalist)';
