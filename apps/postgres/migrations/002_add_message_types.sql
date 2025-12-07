-- Migration: Add support for additional Telegram message types
-- This migration adds tables for stickers, games, polls, contacts, venues, and dice

-- =====================================================
-- ENUMS
-- =====================================================

-- Add 'sticker' to media_type enum
ALTER TYPE media_type ADD VALUE IF NOT EXISTS 'sticker';

-- Poll type enum
CREATE TYPE poll_type AS ENUM ('regular', 'quiz');

-- =====================================================
-- STICKERS (stored in media_files with type 'sticker')
-- =====================================================

-- Stickers table (additional sticker-specific metadata)
CREATE TABLE stickers (
    id BIGSERIAL PRIMARY KEY,
    message_id BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    media_file_id BIGINT NOT NULL REFERENCES media_files(id) ON DELETE CASCADE,
    sticker_type TEXT NOT NULL,
    emoji TEXT,
    set_name TEXT,
    is_animated BOOLEAN DEFAULT FALSE,
    is_video BOOLEAN DEFAULT FALSE,
    custom_emoji_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_stickers_message_id ON stickers(message_id);
CREATE INDEX idx_stickers_set_name ON stickers(set_name) WHERE set_name IS NOT NULL;
CREATE INDEX idx_stickers_emoji ON stickers(emoji) WHERE emoji IS NOT NULL;

-- =====================================================
-- GAMES
-- =====================================================

-- Games table
CREATE TABLE games (
    id BIGSERIAL PRIMARY KEY,
    message_id BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    text TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_games_message_id ON games(message_id);
CREATE INDEX idx_games_title ON games(title);

-- Game photos table (multiple photos per game)
CREATE TABLE game_photos (
    id BIGSERIAL PRIMARY KEY,
    game_id BIGINT NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    telegram_file_id TEXT NOT NULL,
    telegram_file_unique_id TEXT NOT NULL,
    minio_object_key TEXT NOT NULL,
    file_hash TEXT NOT NULL,
    width INT NOT NULL,
    height INT NOT NULL,
    file_size BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_game_photos_game_id ON game_photos(game_id);
CREATE INDEX idx_game_photos_file_hash ON game_photos(file_hash);

-- =====================================================
-- POLLS
-- =====================================================

-- Polls table
CREATE TABLE polls (
    id BIGSERIAL PRIMARY KEY,
    message_id BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    telegram_poll_id TEXT NOT NULL,
    question TEXT NOT NULL,
    poll_type poll_type NOT NULL,
    total_voter_count INT NOT NULL DEFAULT 0,
    is_closed BOOLEAN DEFAULT FALSE,
    is_anonymous BOOLEAN DEFAULT TRUE,
    allows_multiple_answers BOOLEAN DEFAULT FALSE,
    correct_option_id INT,
    explanation TEXT,
    open_period INT,
    close_date TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_polls_message_id ON polls(message_id);
CREATE INDEX idx_polls_telegram_id ON polls(telegram_poll_id);

-- Poll options table
CREATE TABLE poll_options (
    id BIGSERIAL PRIMARY KEY,
    poll_id BIGINT NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    option_index INT NOT NULL,
    option_text TEXT NOT NULL,
    voter_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(poll_id, option_index)
);

CREATE INDEX idx_poll_options_poll_id ON poll_options(poll_id);

-- =====================================================
-- CONTACTS
-- =====================================================

-- Contacts table (shared contact information)
CREATE TABLE contacts (
    id BIGSERIAL PRIMARY KEY,
    message_id BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    phone_number TEXT NOT NULL,
    first_name TEXT NOT NULL,
    last_name TEXT,
    user_id BIGINT,
    vcard TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_contacts_message_id ON contacts(message_id);
CREATE INDEX idx_contacts_phone ON contacts(phone_number);
CREATE INDEX idx_contacts_user_id ON contacts(user_id) WHERE user_id IS NOT NULL;

-- =====================================================
-- VENUES
-- =====================================================

-- Venues table (location with venue information)
CREATE TABLE venues (
    id BIGSERIAL PRIMARY KEY,
    message_id BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    location_id BIGINT NOT NULL REFERENCES locations(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    address TEXT NOT NULL,
    foursquare_id TEXT,
    foursquare_type TEXT,
    google_place_id TEXT,
    google_place_type TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_venues_message_id ON venues(message_id);
CREATE INDEX idx_venues_location_id ON venues(location_id);
CREATE INDEX idx_venues_foursquare ON venues(foursquare_id) WHERE foursquare_id IS NOT NULL;
CREATE INDEX idx_venues_google_place ON venues(google_place_id) WHERE google_place_id IS NOT NULL;

-- =====================================================
-- DICE
-- =====================================================

-- Dice table (dice, darts, basketball, football, slot machine, bowling)
CREATE TABLE dice (
    id BIGSERIAL PRIMARY KEY,
    message_id BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    emoji TEXT NOT NULL,
    dice_value INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_dice_message_id ON dice(message_id);
CREATE INDEX idx_dice_emoji ON dice(emoji);

-- =====================================================
-- TRIGGERS
-- =====================================================

-- Trigger for polls updated_at
CREATE TRIGGER update_polls_updated_at BEFORE UPDATE ON polls
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
