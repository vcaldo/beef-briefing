-- Migration: Add ranked tournaments enable/disable per group
-- Version: 011
-- Description: Adds ranked_tournaments_enabled column to chats table

BEGIN;

-- Add column with default false (disabled by default)
ALTER TABLE chats
ADD COLUMN IF NOT EXISTS ranked_tournaments_enabled BOOLEAN
NOT NULL DEFAULT false;

-- Create index for faster filtering in scheduler queries
CREATE INDEX IF NOT EXISTS idx_chats_ranked_enabled
ON chats(ranked_tournaments_enabled)
WHERE ranked_tournaments_enabled = true;

-- Add helpful comment
COMMENT ON COLUMN chats.ranked_tournaments_enabled IS
'Controls whether daily ranked tournaments are enabled for this chat. Default: false (opt-in)';

COMMIT;
