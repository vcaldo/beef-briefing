-- Add has_rerolled column to track whether participant has used their single reroll
-- Players can only reroll once per match (before or after purchasing cards)

ALTER TABLE game_match_participants
ADD COLUMN IF NOT EXISTS has_rerolled BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN game_match_participants.has_rerolled IS
'Tracks whether the participant has used their single reroll allowance for this match';
