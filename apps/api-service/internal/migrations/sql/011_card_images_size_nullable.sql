-- Migration: 011_card_images_size_nullable
-- Fix NotNullViolation on 'size' column in ml_user_card_images
-- This migration makes the 'size' column nullable to prevent insert failures
-- The 'size' column appears to have been added in production but not synced to dev

-- Check if 'size' column exists and make it nullable if it does
DO $$
BEGIN
    -- Check if the 'size' column exists
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'ml_user_card_images'
        AND column_name = 'size'
    ) THEN
        -- Alter the column to be nullable
        ALTER TABLE ml_user_card_images
        ALTER COLUMN size DROP NOT NULL;
        RAISE NOTICE 'Made size column nullable in ml_user_card_images';
    ELSE
        -- Column doesn't exist yet - nothing to do
        RAISE NOTICE 'size column does not exist in ml_user_card_images';
    END IF;
END $$;
