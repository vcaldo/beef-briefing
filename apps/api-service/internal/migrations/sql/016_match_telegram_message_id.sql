-- Add telegram_message_id column to game_matches table
-- This stores the Telegram message ID of the game modal so the bot can edit it later

ALTER TABLE game_matches
ADD COLUMN IF NOT EXISTS telegram_message_id BIGINT;
