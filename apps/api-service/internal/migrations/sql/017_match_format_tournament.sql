-- Add bracket and free_for_all values to game_match_format enum
-- Required for tournament system (3+ player matches)

ALTER TYPE game_match_format ADD VALUE IF NOT EXISTS 'bracket';
ALTER TYPE game_match_format ADD VALUE IF NOT EXISTS 'free_for_all';
