-- Migration: 006_card_timezone
-- Add timezone column to ml_user_cards table

ALTER TABLE ml_user_cards ADD COLUMN IF NOT EXISTS timezone VARCHAR(64);

COMMENT ON COLUMN ml_user_cards.timezone IS
'IANA timezone identifier used for week boundaries and chronotype calculation (e.g., America/Sao_Paulo)';
