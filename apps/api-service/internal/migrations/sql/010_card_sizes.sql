-- Card Image Size Variants
-- Adds support for generating multiple size variants (large, medium, small) for each card

-- =====================================================
-- CREATE CARD_SIZE ENUM TYPE
-- =====================================================

CREATE TYPE card_size AS ENUM ('large', 'medium', 'small');

-- =====================================================
-- ADD SIZE COLUMN TO ml_user_card_images
-- =====================================================

-- Step 1: Add size column (nullable for backward compatibility during migration)
ALTER TABLE ml_user_card_images
ADD COLUMN IF NOT EXISTS size card_size;

-- Step 2: Backfill existing images as 'large' (current 800x1200px output)
UPDATE ml_user_card_images
SET size = 'large'
WHERE size IS NULL;

-- Step 3: Make size NOT NULL
ALTER TABLE ml_user_card_images
ALTER COLUMN size SET NOT NULL;

-- =====================================================
-- UPDATE UNIQUE CONSTRAINT
-- =====================================================

-- Step 4: Drop old unique constraint (one image per card per theme)
ALTER TABLE ml_user_card_images
DROP CONSTRAINT IF EXISTS ml_user_card_images_card_id_theme_key;

-- Step 5: Add new unique constraint (one image per card per theme per size)
ALTER TABLE ml_user_card_images
ADD CONSTRAINT ml_user_card_images_card_id_theme_size_key
UNIQUE(card_id, theme, size);

-- =====================================================
-- UPDATE INDEXES
-- =====================================================

-- Step 6: Drop old index
DROP INDEX IF EXISTS idx_card_images_chat_week;

-- Step 7: Recreate index with size column for better filtering
CREATE INDEX idx_card_images_chat_week_size
ON ml_user_card_images(chat_id, week_start DESC, size);

-- Keep other indexes as-is since they remain useful for point lookups

-- =====================================================
-- UPDATE DOCUMENTATION
-- =====================================================

COMMENT ON COLUMN ml_user_card_images.size IS
'Image size variant: large (800x1200px), medium (400x600px), small (200x300px)';

COMMENT ON COLUMN ml_user_card_images.storage_path IS
'Object storage path: cards/{chat_id}/{week_start}/{theme}/{user_id}_{size}.png';

-- Update table comment to mention multiple sizes
COMMENT ON TABLE ml_user_card_images IS
'Stores references to rendered card images in object storage. Each card+theme+size combination produces a separate image row. Links to ml_user_cards for source data.';
