-- 006_deck_active.down.sql
DROP INDEX IF EXISTS idx_decks_inactive;
ALTER TABLE decks
    DROP COLUMN IF EXISTS is_active,
    DROP COLUMN IF EXISTS expires_at;
