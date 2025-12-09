-- Migration: Add unique constraint to prevent duplicate media registration
-- This migration enforces that the same telegram_file_id cannot be registered
-- multiple times for the same message, preventing issues like GIFs being
-- registered as both 'document' and 'animation' media types.

-- =====================================================
-- UNIQUE CONSTRAINT ON MEDIA FILES
-- =====================================================

-- Create unique index to prevent duplicate file_id per message
-- This catches cases where Telegram populates both Document and Animation
-- fields with the same file_id (common for GIFs)
CREATE UNIQUE INDEX idx_media_unique_file_per_message
ON media_files(message_id, telegram_file_id);

-- Add comment explaining the constraint
COMMENT ON INDEX idx_media_unique_file_per_message IS
'Ensures each telegram_file_id appears only once per message, preventing duplicates when Telegram populates multiple fields (e.g., Document + Animation for GIFs)';
