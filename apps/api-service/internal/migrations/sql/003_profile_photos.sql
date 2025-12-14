-- Database schema for user and chat profile photos
-- This migration adds storage for Telegram profile photos

-- =====================================================
-- USER PROFILE PHOTOS
-- =====================================================

-- Stores all sizes of a user's current profile photo
-- Old photos are deleted when new ones are uploaded
CREATE TABLE IF NOT EXISTS user_profile_photos (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    telegram_file_id TEXT NOT NULL,
    telegram_file_unique_id TEXT NOT NULL,
    minio_object_key TEXT NOT NULL,
    file_hash TEXT NOT NULL,
    width INT NOT NULL,
    height INT NOT NULL,
    file_size BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_profile_photos_user_id
ON user_profile_photos(user_id);

CREATE INDEX IF NOT EXISTS idx_user_profile_photos_file_hash
ON user_profile_photos(file_hash);

COMMENT ON TABLE user_profile_photos IS
'Stores all sizes of a user''s current profile photo. Photos are replaced atomically when updated.';

-- =====================================================
-- CHAT PROFILE PHOTOS
-- =====================================================

-- Stores both sizes (small 160x160, big 640x640) of a chat's profile photo
CREATE TABLE IF NOT EXISTS chat_profile_photos (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    photo_size TEXT NOT NULL CHECK (photo_size IN ('small', 'big')),
    telegram_file_id TEXT NOT NULL,
    telegram_file_unique_id TEXT NOT NULL,
    minio_object_key TEXT NOT NULL,
    file_hash TEXT NOT NULL,
    width INT,
    height INT,
    file_size BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(chat_id, photo_size)
);

CREATE INDEX IF NOT EXISTS idx_chat_profile_photos_chat_id
ON chat_profile_photos(chat_id);

CREATE INDEX IF NOT EXISTS idx_chat_profile_photos_file_hash
ON chat_profile_photos(file_hash);

COMMENT ON TABLE chat_profile_photos IS
'Stores both sizes (small 160x160, big 640x640) of a chat''s profile photo. Photos are replaced atomically when updated.';
