-- Migration: Make media file storage optional
-- This migration allows media_files records to exist without actual file downloads.
-- This is useful for tracking video metadata even when files are too large to download.

-- =====================================================
-- MODIFY MEDIA FILES TABLE
-- =====================================================

-- Make minio_object_key nullable (file might not be downloaded/stored)
ALTER TABLE media_files
ALTER COLUMN minio_object_key DROP NOT NULL;

-- Make file_hash nullable (no file = no hash)
ALTER TABLE media_files
ALTER COLUMN file_hash DROP NOT NULL;

-- Add comment explaining the nullable columns
COMMENT ON COLUMN media_files.minio_object_key IS
'MinIO storage path. NULL if file was not downloaded (e.g., exceeded size limit).';

COMMENT ON COLUMN media_files.file_hash IS
'SHA256 hash of file content. NULL if file was not downloaded.';
