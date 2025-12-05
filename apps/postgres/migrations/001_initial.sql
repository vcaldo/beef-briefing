-- Database initialization for Telegram Bot Message Logging
-- This file is run automatically when the PostgreSQL container starts

-- Enable UUID extension for generating UUIDs
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Chats table: stores information about Telegram groups/channels
CREATE TABLE chats (
    id BIGINT PRIMARY KEY,
    type VARCHAR(50) NOT NULL, -- 'private_group', 'public_group', 'channel', 'private'
    name VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Users table: stores information about Telegram users
CREATE TABLE users (
    id BIGINT PRIMARY KEY,
    username VARCHAR(255),
    first_name VARCHAR(255),
    last_name VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Messages table: stores all types of messages
CREATE TABLE messages (
    id BIGSERIAL PRIMARY KEY,
    telegram_message_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL REFERENCES chats(id),
    user_id BIGINT REFERENCES users(id),
    message_date TIMESTAMPTZ NOT NULL,
    message_type VARCHAR(50) NOT NULL, -- 'text', 'photo', 'video', 'voice', 'sticker', 'document', 'animation', 'video_note'
    text TEXT,
    reply_to_message_id BIGINT,
    forwarded_from_user_id BIGINT REFERENCES users(id),
    forwarded_from_chat_id BIGINT REFERENCES chats(id),
    forwarded_date TIMESTAMPTZ,
    edit_date TIMESTAMPTZ,

    -- Media file information
    media_sha256 VARCHAR(64), -- SHA256 hash of media file (used as MinIO key)
    media_file_name VARCHAR(512),
    media_file_size BIGINT,
    media_mime_type VARCHAR(255),
    media_duration_seconds INTEGER, -- for audio/video
    media_width INTEGER, -- for photo/video
    media_height INTEGER, -- for photo/video

    -- Metadata stored as JSON for flexibility
    entities JSONB, -- text entities (mentions, links, bold, etc.)
    metadata JSONB, -- additional message-specific data

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(chat_id, telegram_message_id)
);

-- Service messages table: stores service events (joins, leaves, etc.)
CREATE TABLE service_messages (
    id BIGSERIAL PRIMARY KEY,
    telegram_message_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL REFERENCES chats(id),
    actor_user_id BIGINT REFERENCES users(id),
    message_date TIMESTAMPTZ NOT NULL,
    action VARCHAR(100) NOT NULL, -- 'user_joined', 'user_left', 'chat_created', 'title_changed', 'photo_changed', etc.

    -- Action-specific data stored as JSON
    metadata JSONB, -- e.g., {"members": ["user1", "user2"], "new_title": "New Chat Name"}

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(chat_id, telegram_message_id)
);

-- Message reactions table: tracks individual reactors
CREATE TABLE message_reactions (
    id BIGSERIAL PRIMARY KEY,
    message_id BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id),
    emoji VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(message_id, user_id, emoji)
);

-- Indexes for better query performance
CREATE INDEX idx_messages_chat_id ON messages(chat_id);
CREATE INDEX idx_messages_user_id ON messages(user_id);
CREATE INDEX idx_messages_message_date ON messages(message_date);
CREATE INDEX idx_messages_telegram_message_id ON messages(telegram_message_id);
CREATE INDEX idx_messages_media_sha256 ON messages(media_sha256) WHERE media_sha256 IS NOT NULL;

CREATE INDEX idx_service_messages_chat_id ON service_messages(chat_id);
CREATE INDEX idx_service_messages_actor_user_id ON service_messages(actor_user_id);
CREATE INDEX idx_service_messages_message_date ON service_messages(message_date);

CREATE INDEX idx_message_reactions_message_id ON message_reactions(message_id);
CREATE INDEX idx_message_reactions_user_id ON message_reactions(user_id);

-- Trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_chats_updated_at BEFORE UPDATE ON chats
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_messages_updated_at BEFORE UPDATE ON messages
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
