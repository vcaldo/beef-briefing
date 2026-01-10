-- Migration: 008_chat_timezone
-- Add timezone column to chats table for group-level timezone configuration
-- Used for streak calculations and activity displays in the mini-app

ALTER TABLE chats ADD COLUMN IF NOT EXISTS timezone VARCHAR(64) DEFAULT NULL;

COMMENT ON COLUMN chats.timezone IS 'IANA timezone identifier for the group (e.g., America/Sao_Paulo). Used for streak calculations and activity displays. NULL defaults to UTC.';
