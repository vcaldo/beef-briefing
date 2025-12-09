-- Database schema for Telegram Message Logging
-- This file is run automatically when the PostgreSQL container starts

-- Enable UUID extension for potential future use
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- =====================================================
-- ENUMS
-- =====================================================

-- Reaction type enum
CREATE TYPE reaction_type AS ENUM ('emoji', 'custom_emoji', 'paid');

-- Chat type enum
CREATE TYPE chat_type AS ENUM ('private', 'group', 'supergroup', 'channel');

-- Media type enum (includes sticker for extended message types)
CREATE TYPE media_type AS ENUM ('photo', 'video', 'audio', 'voice', 'document', 'animation', 'video_note', 'sticker');

-- Poll type enum
CREATE TYPE poll_type AS ENUM ('regular', 'quiz');

-- =====================================================
-- CORE TABLES
-- =====================================================

-- Chats table (groups, supergroups, channels, private chats)
CREATE TABLE chats (
    id BIGINT PRIMARY KEY,
    type chat_type NOT NULL,
    title TEXT,
    username TEXT,
    first_name TEXT,
    last_name TEXT,
    is_forum BOOLEAN DEFAULT FALSE,
    migrated_from_chat_id BIGINT UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_chats_username ON chats(username) WHERE username IS NOT NULL;
CREATE INDEX idx_chats_type ON chats(type);
CREATE INDEX idx_chats_migrated_from ON chats(migrated_from_chat_id) WHERE migrated_from_chat_id IS NOT NULL;

-- Users table
CREATE TABLE users (
    id BIGINT PRIMARY KEY,
    is_bot BOOLEAN NOT NULL DEFAULT FALSE,
    first_name TEXT NOT NULL,
    last_name TEXT,
    username TEXT,
    language_code TEXT,
    is_premium BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_username ON users(username) WHERE username IS NOT NULL;
CREATE INDEX idx_users_is_bot ON users(is_bot);

-- Updates table (stores all incoming Telegram updates)
CREATE TABLE updates (
    id BIGSERIAL PRIMARY KEY,
    update_id BIGINT NOT NULL UNIQUE,
    update_type TEXT NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    raw_data JSONB NOT NULL
);

CREATE INDEX idx_updates_update_id ON updates(update_id);
CREATE INDEX idx_updates_received_at ON updates(received_at DESC);
CREATE INDEX idx_updates_type ON updates(update_type);

-- =====================================================
-- MESSAGES
-- =====================================================

-- Messages table
CREATE TABLE messages (
    id BIGSERIAL PRIMARY KEY,
    message_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    message_thread_id BIGINT,
    reply_to_message_id BIGINT,
    date TIMESTAMPTZ NOT NULL,
    edit_date TIMESTAMPTZ,
    text TEXT,
    caption TEXT,
    media_group_id TEXT,
    has_protected_content BOOLEAN DEFAULT FALSE,
    is_automatic_forward BOOLEAN DEFAULT FALSE,
    is_topic_message BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(chat_id, message_id)
);

CREATE INDEX idx_messages_chat_id ON messages(chat_id);
CREATE INDEX idx_messages_user_id ON messages(user_id);
CREATE INDEX idx_messages_date ON messages(date DESC);
CREATE INDEX idx_messages_chat_date ON messages(chat_id, date DESC);
CREATE INDEX idx_messages_reply_to ON messages(reply_to_message_id) WHERE reply_to_message_id IS NOT NULL;
CREATE INDEX idx_messages_media_group ON messages(media_group_id) WHERE media_group_id IS NOT NULL;
CREATE INDEX idx_messages_thread ON messages(message_thread_id) WHERE message_thread_id IS NOT NULL;

-- Message entities (bold, italic, mentions, links, etc.)
CREATE TABLE message_entities (
    id BIGSERIAL PRIMARY KEY,
    message_id BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    entity_type TEXT NOT NULL,
    entity_offset INT NOT NULL,
    entity_length INT NOT NULL,
    url TEXT,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    language TEXT,
    custom_emoji_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_entities_message_id ON message_entities(message_id);
CREATE INDEX idx_entities_type ON message_entities(entity_type);

-- Message edits audit table
CREATE TABLE message_edits (
    id BIGSERIAL PRIMARY KEY,
    message_id BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    edit_date TIMESTAMPTZ NOT NULL,
    previous_text TEXT,
    new_text TEXT,
    previous_caption TEXT,
    new_caption TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_edits_message_id ON message_edits(message_id);
CREATE INDEX idx_edits_edit_date ON message_edits(edit_date DESC);

-- =====================================================
-- REACTIONS
-- =====================================================

-- DESIGN NOTE: Reactions are intentionally denormalized
-- The message_id column stores the Telegram message ID (not a FK to messages.id).
-- This allows storing reactions for messages that may not have been captured by the bot
-- (e.g., reactions to old messages sent before the bot was added to the chat).
-- To find the local message record, join using: (chat_id, message_id) -> messages(chat_id, message_id)

-- Individual message reactions (from MessageReactionUpdated)
CREATE TABLE message_reactions (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    message_id BIGINT NOT NULL,  -- Telegram message ID, not FK (see design note above)
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    actor_chat_id BIGINT REFERENCES chats(id) ON DELETE SET NULL,
    reaction_type reaction_type NOT NULL,
    emoji_value TEXT,
    custom_emoji_id TEXT,
    date TIMESTAMPTZ NOT NULL,
    is_removed BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(chat_id, message_id, user_id, reaction_type, emoji_value, custom_emoji_id)
);

CREATE INDEX idx_reactions_message ON message_reactions(chat_id, message_id);
CREATE INDEX idx_reactions_user ON message_reactions(user_id);
CREATE INDEX idx_reactions_type ON message_reactions(reaction_type);
CREATE INDEX idx_reactions_date ON message_reactions(date DESC);

-- Aggregate reaction counts (from MessageReactionCountUpdated)
-- Same denormalized design as message_reactions (see design note above)
CREATE TABLE reaction_counts (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    message_id BIGINT NOT NULL,  -- Telegram message ID, not FK (see design note in message_reactions)
    reaction_type reaction_type NOT NULL,
    emoji_value TEXT,
    custom_emoji_id TEXT,
    total_count INT NOT NULL DEFAULT 0,
    date TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(chat_id, message_id, reaction_type, emoji_value, custom_emoji_id)
);

CREATE INDEX idx_reaction_counts_message ON reaction_counts(chat_id, message_id);
CREATE INDEX idx_reaction_counts_date ON reaction_counts(date DESC);

-- =====================================================
-- MEDIA FILES
-- =====================================================

-- Media files table (stores metadata and MinIO references)
CREATE TABLE media_files (
    id BIGSERIAL PRIMARY KEY,
    message_id BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    media_type media_type NOT NULL,
    telegram_file_id TEXT NOT NULL,
    telegram_file_unique_id TEXT NOT NULL,
    minio_object_key TEXT,  -- NULL if file was not downloaded (e.g., exceeded size limit)
    file_hash TEXT,  -- NULL if file was not downloaded
    file_size BIGINT,
    mime_type TEXT,
    file_name TEXT,
    duration INT,
    width INT,
    height INT,
    performer TEXT,
    title TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_media_message_id ON media_files(message_id);
CREATE INDEX idx_media_file_id ON media_files(telegram_file_id);
CREATE INDEX idx_media_type ON media_files(media_type);
CREATE INDEX idx_media_file_hash ON media_files(file_hash);

-- Unique constraint to prevent duplicate file_id per message
-- (catches cases where Telegram populates both Document and Animation fields for GIFs)
CREATE UNIQUE INDEX idx_media_unique_file_per_message
ON media_files(message_id, telegram_file_id);

COMMENT ON COLUMN media_files.minio_object_key IS
'MinIO storage path. NULL if file was not downloaded (e.g., exceeded size limit).';

COMMENT ON COLUMN media_files.file_hash IS
'SHA256 hash of file content. NULL if file was not downloaded.';

COMMENT ON INDEX idx_media_unique_file_per_message IS
'Ensures each telegram_file_id appears only once per message, preventing duplicates when Telegram populates multiple fields (e.g., Document + Animation for GIFs)';

-- Photos table (multiple sizes per message)
CREATE TABLE photos (
    id BIGSERIAL PRIMARY KEY,
    message_id BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    telegram_file_id TEXT NOT NULL,
    telegram_file_unique_id TEXT NOT NULL,
    minio_object_key TEXT NOT NULL,
    file_hash TEXT NOT NULL,
    width INT NOT NULL,
    height INT NOT NULL,
    file_size BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_photos_message_id ON photos(message_id);
CREATE INDEX idx_photos_file_id ON photos(telegram_file_id);
CREATE INDEX idx_photos_file_hash ON photos(file_hash);

-- Locations table
CREATE TABLE locations (
    id BIGSERIAL PRIMARY KEY,
    message_id BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    latitude DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,
    horizontal_accuracy DOUBLE PRECISION,
    live_period INT,
    heading INT,
    proximity_alert_radius INT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_locations_message_id ON locations(message_id);

-- =====================================================
-- EXTENDED MESSAGE TYPES
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
-- FUNCTIONS & TRIGGERS
-- =====================================================

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers for updated_at
CREATE TRIGGER update_chats_updated_at BEFORE UPDATE ON chats
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_messages_updated_at BEFORE UPDATE ON messages
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_reaction_counts_updated_at BEFORE UPDATE ON reaction_counts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_polls_updated_at BEFORE UPDATE ON polls
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
