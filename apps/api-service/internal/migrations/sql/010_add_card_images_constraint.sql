-- Add missing unique constraint to ml_user_card_images
-- This constraint is needed for the ON CONFLICT clause in card image upserts
-- Required for supporting multiple themes per card (including compact cards)

-- Check if constraint already exists before adding
DO $$
BEGIN
    -- Try to add the unique constraint if it doesn't exist
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE table_name = 'ml_user_card_images'
        AND constraint_name = 'ml_user_card_images_card_id_theme_key'
        AND constraint_type = 'UNIQUE'
    ) THEN
        ALTER TABLE ml_user_card_images
        ADD CONSTRAINT ml_user_card_images_card_id_theme_key UNIQUE (card_id, theme);
        RAISE NOTICE 'Added unique constraint to ml_user_card_images';
    ELSE
        RAISE NOTICE 'Unique constraint already exists on ml_user_card_images';
    END IF;
END $$;
